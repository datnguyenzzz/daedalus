pub mod manifest;
use std::{fs, sync::Arc};

use crate::{
    CatalogStorage, FsBackend, StorageError, TableMetadata,
    catalog::manifest::{SegmentFileRef, TableManifest, decode_manifest, encode_manifest},
};
use async_trait::async_trait;
use rs_arrow::{
    DataType,
    ipc::record_batch::MessageReader,
    object::{self, ColumnStat},
};

pub static BINCODE_CONF: bincode::config::Configuration = bincode::config::standard();

pub struct Catalog {
    be: Arc<FsBackend>,
}

#[inline]
fn current_path(table_name: &str) -> String {
    format!("{table_name}/CURRENT")
}

#[inline]
fn manifest_file_name(version: u64) -> String {
    format!("manifest-{version}.bin")
}

#[inline]
fn manifest_path(table_name: &str, version: u64) -> String {
    format!("{table_name}/{}", manifest_file_name(version))
}

impl Catalog {
    pub fn new(be: Arc<FsBackend>) -> Self {
        Self { be }
    }

    pub async fn fetch_manifest(&self, table_name: &str) -> Result<TableManifest, StorageError> {
        let cur_rel = current_path(table_name);
        let cur_path = self.be.resolve(&cur_rel);

        if !cur_path.exists() {
            return Err(StorageError::TableNotFound(table_name.to_string()));
        }

        let content = fs::read_to_string(&cur_path)?;
        let manifest_file = content.trim(); // manifest file content is "manifest-{version}.bin"
        if manifest_file.is_empty() {
            return Err(StorageError::TableNotFound(table_name.to_string()));
        }

        let manifest_rel = format!("{table_name}/{manifest_file}");
        let manifest_path = self.be.resolve(&manifest_rel);
        let manifest_bytes = fs::read(&manifest_path)?;

        decode_manifest(&manifest_bytes)
    }

    pub async fn register_segment(
        &self,
        table: &str,
        mut seg_ref: SegmentFileRef,
    ) -> Result<u64, StorageError> {
        let be = self.be.clone();
        let clone_table_name = table.to_string();
        let mut columns_stat = Vec::with_capacity(seg_ref.footer_entries.len());
        for entry in &seg_ref.footer_entries {
            let mut reader = MessageReader::new(&entry.meta_bytes[..]);
            if let Ok(Some(fb_msg)) = reader.try_next_meta_only() {
                if let Some(fb_batch) = fb_msg.header_as_fb_record_batch() {
                    if let Some(nodes) = fb_batch.nodes() {
                        for (col_idx, node) in nodes.iter().enumerate() {
                            columns_stat
                                .push((col_idx, ColumnStat::decode(node.stats_byte().bytes())));
                        }
                    }
                }
            }
        }
        self.be
            .with_table_lock(&table, move || -> Result<u64, StorageError> {
                let cur_rel = current_path(&clone_table_name);
                let cur_path = be.resolve(&cur_rel);
                // read the CURRENT manifest
                let mut manifest = if cur_path.exists() {
                    let content = fs::read_to_string(&cur_path)?;
                    let manifest_file = content.trim();
                    let manifest_bytes =
                        fs::read(be.resolve(&format!("{clone_table_name}/{manifest_file}")))?;
                    decode_manifest(&manifest_bytes)?
                } else {
                    return Err(StorageError::TableNotFound(clone_table_name));
                };

                // MVCC commit sequence
                let next_ver = manifest.version + 1;
                seg_ref.created_at_version = next_ver;

                for (col_idx, opt_stat) in columns_stat {
                    if let Some(stat) = opt_stat {
                        manifest
                            .table_zonemap
                            .entry(col_idx)
                            .and_modify(|existing| *existing = object::merge(existing, &stat))
                            .or_insert(stat);
                    }
                }

                manifest.total_rows += seg_ref.row_count;
                manifest.segment_files.push(seg_ref);
                manifest.version = next_ver;

                // atomic write manifest file
                let new_manifest_rel = manifest_path(&clone_table_name, next_ver);
                let encoded_manifest = encode_manifest(&manifest)?;
                let current_rel = current_path(&clone_table_name);

                be.write_atomic_sync(&new_manifest_rel, &encoded_manifest)?;

                be.write_atomic_sync(&current_rel, manifest_file_name(next_ver).as_bytes())?;

                Ok(next_ver)
            })
            .await
    }
}

#[async_trait]
impl CatalogStorage for Catalog {
    async fn fetch_table_meta(&self, table_name: &str) -> Result<TableMetadata, StorageError> {
        let manifest = self.fetch_manifest(table_name).await?;
        let (columns, _): (Vec<(String, DataType)>, usize) =
            bincode::decode_from_slice(&manifest.schema_bytes, BINCODE_CONF).map_err(|e| {
                StorageError::CorruptFooter {
                    segment_id: "?".into(),
                    reason: format!("failed to deserialize schema columns: {e}"),
                }
            })?;
        Ok(TableMetadata {
            name: table_name.to_string(),
            columns,
        })
    }

    async fn register_table_meta(
        &self,
        table: &str,
        metadata: TableMetadata,
    ) -> Result<(), StorageError> {
        let be = self.be.clone();
        let clone_table_name = table.to_string();
        self.be
            .with_table_lock(&table, move || -> Result<(), StorageError> {
                let bytes =
                    bincode::encode_to_vec(&metadata.columns, BINCODE_CONF).map_err(|e| {
                        StorageError::Arrow(format!("schema serialization failed: {e}"))
                    })?;
                let manifest = TableManifest::new(bytes);
                let manifest_rel = manifest_path(&clone_table_name, manifest.version);
                let encoded = encode_manifest(&manifest)?;

                be.write_atomic_sync(&manifest_rel, &encoded)?;

                let current_rel = current_path(&clone_table_name);
                be.write_atomic_sync(&current_rel, manifest_file_name(manifest.version).as_bytes())?;
                Ok(())
            })
            .await
    }

    async fn drop_table_meta(&self, table: &str) -> Result<(), StorageError> {
        let be = self.be.clone();
        let clone_table_name = table.to_string();
        self.be
            .with_table_lock(&table, move || -> Result<(), StorageError> {
                let table_dir = be.resolve(&clone_table_name);
                if table_dir.exists() {
                    fs::remove_dir_all(table_dir)?;
                }
                Ok(())
            })
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::Config;
    use std::sync::Arc;
    use tokio::task::JoinSet;

    #[tokio::test]
    async fn test_catalog_register_and_fetch_table_meta() {
        let temp_dir = tempfile::tempdir().unwrap();
        let be = Arc::new(
            FsBackend::try_new(Some(Config {
                root_dir: temp_dir.path().to_path_buf(),
                fsync: false,
            }))
            .unwrap(),
        );
        let catalog = Catalog::new(be);

        let table = "users";
        let meta = TableMetadata {
            name: table.to_string(),
            columns: vec![
                ("id".to_string(), DataType::Int32),
                ("name".to_string(), DataType::Utf8),
            ],
        };

        // 1. Register table
        catalog
            .register_table_meta(table, meta.clone())
            .await
            .unwrap();

        // 2. Fetch table
        let fetched = catalog.fetch_table_meta(table).await.unwrap();
        assert_eq!(fetched, meta);

        // 3. Drop table
        catalog.drop_table_meta(table).await.unwrap();
        let err = catalog.fetch_table_meta(table).await.unwrap_err();
        assert!(matches!(err, StorageError::TableNotFound(_)));
    }

    #[tokio::test]
    async fn test_catalog_register_segment_monotonic_sequence() {
        let temp_dir = tempfile::tempdir().unwrap();
        let be = Arc::new(
            FsBackend::try_new(Some(Config {
                root_dir: temp_dir.path().to_path_buf(),
                fsync: false,
            }))
            .unwrap(),
        );
        let catalog = Catalog::new(be);

        let table = "orders";
        catalog
            .register_table_meta(
                table,
                TableMetadata {
                    name: table.to_string(),
                    columns: vec![("id".to_string(), DataType::Int64)],
                },
            )
            .await
            .unwrap();

        // Initial manifest version is 1
        let m1 = catalog.fetch_manifest(table).await.unwrap();
        assert_eq!(m1.version, 1);
        assert_eq!(m1.segment_files.len(), 0);

        // Register segment 1 -> version 2
        let seg1 = SegmentFileRef {
            dat_path: format!("{table}/segments/seg-1.dat"),
            footer_offset: 100,
            footer_len: 50,
            row_count: 100,
            chunk_count: 1,
            created_at_version: 0,
            footer_entries: vec![],
            validity_mask: None,
        };
        let v2 = catalog.register_segment(table, seg1).await.unwrap();
        assert_eq!(v2, 2);

        // Register segment 2 -> version 3
        let seg2 = SegmentFileRef {
            dat_path: format!("{table}/segments/seg-2.dat"),
            footer_offset: 200,
            footer_len: 50,
            row_count: 200,
            chunk_count: 1,
            created_at_version: 0,
            footer_entries: vec![],
            validity_mask: None,
        };
        let v3 = catalog.register_segment(table, seg2).await.unwrap();
        assert_eq!(v3, 3);

        // Verify manifest snapshot
        let m3 = catalog.fetch_manifest(table).await.unwrap();
        assert_eq!(m3.version, 3);
        assert_eq!(m3.total_rows, 300);
        assert_eq!(m3.segment_files.len(), 2);
        assert_eq!(m3.segment_files[0].created_at_version, 2);
        assert_eq!(m3.segment_files[1].created_at_version, 3);
    }

    #[tokio::test]
    async fn test_concurrent_segment_registration_verification_gate() {
        let temp_dir = tempfile::tempdir().unwrap();
        let be = Arc::new(
            FsBackend::try_new(Some(Config {
                root_dir: temp_dir.path().to_path_buf(),
                fsync: false,
            }))
            .unwrap(),
        );
        let catalog = Arc::new(Catalog::new(be));

        let table = "concurrent_appends";
        catalog
            .register_table_meta(
                table,
                TableMetadata {
                    name: table.to_string(),
                    columns: vec![("v".to_string(), DataType::Int32)],
                },
            )
            .await
            .unwrap();

        let num_threads = 10;
        let mut set = JoinSet::new();

        for i in 0..num_threads {
            let cat = catalog.clone();
            set.spawn(async move {
                let seg = SegmentFileRef {
                    dat_path: format!("concurrent_appends/segments/seg-{i}.dat"),
                    footer_offset: i * 100,
                    footer_len: 50,
                    row_count: 10,
                    chunk_count: 1,
                    created_at_version: 0,
                    footer_entries: vec![],
                    validity_mask: None,
                };
                cat.register_segment("concurrent_appends", seg).await
            });
        }

        // Collect all results
        let mut committed_versions = Vec::new();
        while let Some(res) = set.join_next().await {
            let ver = res.unwrap().unwrap();
            committed_versions.push(ver);
        }

        assert_eq!(committed_versions.len(), num_threads);

        // Fetch final manifest state
        let final_manifest = catalog.fetch_manifest(table).await.unwrap();

        // 1. Assert no lost segments: exactly num_threads segments present
        assert_eq!(final_manifest.segment_files.len(), num_threads);
        assert_eq!(final_manifest.total_rows, num_threads * 10);

        // 2. Assert manifest version is strictly 1 + num_threads
        assert_eq!(final_manifest.version, 1 + num_threads as u64);

        // 3. Assert segment created_at_version sequence numbers are strictly monotonic and unique (2..=11)
        let versions_in_manifest: Vec<u64> = final_manifest
            .segment_files
            .iter()
            .map(|s| s.created_at_version)
            .collect();

        // Sequence must be strictly increasing: 2, 3, 4, ..., 11
        let expected_versions: Vec<u64> = (2..=1 + num_threads as u64).collect();
        assert_eq!(versions_in_manifest, expected_versions);
    }
}
