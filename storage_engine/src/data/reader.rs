use std::sync::Arc;

use async_trait::async_trait;

use crate::{
    FsBackend, StorageError, StorageIterator, catalog::manifest::TableManifest,
    data::footer::FooterEntry,
};

pub struct SegmentReader {
    be: Arc<FsBackend>,
    manifest: TableManifest,
    seg_idx: usize,
    chunk_idx: usize,
    pub projections: Vec<usize>,
    buffer: Vec<u8>,
}

impl SegmentReader {
    pub fn new(be: Arc<FsBackend>, manifest: TableManifest, projections: Vec<usize>) -> Self {
        Self {
            be,
            manifest,
            seg_idx: 0,
            chunk_idx: 0,
            projections,
            buffer: Vec::new(),
        }
    }

    #[inline]
    fn current(&self) -> Option<(&str, &FooterEntry)> {
        if self.seg_idx >= self.manifest.segment_files.len() {
            return None;
        }
        let seg = self.manifest.segment_files.get(self.seg_idx)?;
        let entry = seg.footer_entries.get(self.chunk_idx)?;
        Some((seg.dat_path.as_str(), entry))
    }

    #[inline]
    fn advance(&mut self) {
        if let Some(seg) = self.manifest.segment_files.get(self.seg_idx) {
            self.chunk_idx += 1;
            if self.chunk_idx >= seg.footer_entries.len() {
                self.seg_idx += 1;
                self.chunk_idx = 0;
            }
        }
    }
}

#[async_trait]
impl StorageIterator for SegmentReader {
    async fn next_chunk_meta(&mut self) -> Result<Option<&[u8]>, StorageError> {
        match self.current() {
            Some((_, entry)) => Ok(Some(&entry.meta_bytes)),
            None => Ok(None),
        }
    }

    fn skip_current_chunk(&mut self) -> Result<(), StorageError> {
        if self.current().is_none() {
            return Err(StorageError::IteratorExhausted);
        }

        self.advance();
        Ok(())
    }

    async fn read_current_chunk_data(&mut self) -> Result<&[u8], StorageError> {
        let (seg_ref_path, range) = {
            let seg = self
                .manifest
                .segment_files
                .get(self.seg_idx)
                .ok_or(StorageError::IteratorExhausted)?;
            let entry = seg
                .footer_entries
                .get(self.chunk_idx)
                .ok_or(StorageError::IteratorExhausted)?;
            let range = (entry.offset as usize)..((entry.offset + entry.len) as usize);
            (seg.dat_path.as_str(), range)
        };

        // Disjoint field borrow: borrows `seg_ref_path` from `self.manifest` and mutably borrows `self.buffer`
        self.be
            .read_range_into(seg_ref_path, range, &mut self.buffer)
            .await?;
        self.advance();
        Ok(&self.buffer)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;
    use crate::{Config, catalog::manifest::SegmentFileRef};

    #[tokio::test]
    async fn test_segment_reader_metadata_and_advance() {
        let temp_dir = tempfile::tempdir().unwrap();
        let be = Arc::new(
            FsBackend::try_new(Some(Config {
                root_dir: temp_dir.path().to_path_buf(),
                fsync: false,
            }))
            .unwrap(),
        );

        // Write 2 mock segment files
        let seg1_path = "table/seg-1.dat";
        let seg2_path = "table/seg-2.dat";
        be.write_atomic(seg1_path, b"CHUNK_00_DATA_CHUNK_01_DATA_".to_vec())
            .await
            .unwrap();
        be.write_atomic(seg2_path, b"CHUNK_10_DATA_CHUNK_11_DATA_".to_vec())
            .await
            .unwrap();

        let mut manifest = TableManifest::new(vec![]);
        manifest.segment_files = vec![
            SegmentFileRef {
                dat_path: seg1_path.to_string(),
                footer_offset: 100,
                footer_len: 20,
                row_count: 20,
                chunk_count: 2,
                created_at_version: 1,
                footer_entries: vec![
                    FooterEntry {
                        offset: 0,
                        len: 14,
                        num_rows: 10,
                        meta_bytes: b"meta_00".to_vec(),
                        column_pointers: vec![],
                    },
                    FooterEntry {
                        offset: 14,
                        len: 14,
                        num_rows: 10,
                        meta_bytes: b"meta_01".to_vec(),
                        column_pointers: vec![],
                    },
                ],
                validity_mask: None,
            },
            SegmentFileRef {
                dat_path: seg2_path.to_string(),
                footer_offset: 100,
                footer_len: 20,
                row_count: 20,
                chunk_count: 2,
                created_at_version: 2,
                footer_entries: vec![
                    FooterEntry {
                        offset: 0,
                        len: 14,
                        num_rows: 10,
                        meta_bytes: b"meta_10".to_vec(),
                        column_pointers: vec![],
                    },
                    FooterEntry {
                        offset: 14,
                        len: 14,
                        num_rows: 10,
                        meta_bytes: b"meta_11".to_vec(),
                        column_pointers: vec![],
                    },
                ],
                validity_mask: None,
            },
        ];

        let mut reader = SegmentReader::new(be, manifest, vec![]);

        // 1. Chunk 0: Read meta, then read data -> advances cursor
        let meta0 = reader.next_chunk_meta().await.unwrap();
        assert_eq!(meta0, Some(b"meta_00".as_ref()));

        let data0 = reader.read_current_chunk_data().await.unwrap();
        assert_eq!(data0, b"CHUNK_00_DATA_");

        // 2. Chunk 1: Read meta, then SKIP -> advances cursor across segment boundary
        let meta1 = reader.next_chunk_meta().await.unwrap();
        assert_eq!(meta1, Some(b"meta_01".as_ref()));
        reader.skip_current_chunk().unwrap();

        // 3. Chunk 2 (Segment 2, Chunk 0): Read data directly
        let meta2 = reader.next_chunk_meta().await.unwrap();
        assert_eq!(meta2, Some(b"meta_10".as_ref()));
        let data2 = reader.read_current_chunk_data().await.unwrap();
        assert_eq!(data2, b"CHUNK_10_DATA_");

        // 4. Chunk 3: Skip
        reader.skip_current_chunk().unwrap();

        // 5. EOF reached
        assert_eq!(reader.next_chunk_meta().await.unwrap(), None);
        assert!(reader.read_current_chunk_data().await.is_err());
        assert!(reader.skip_current_chunk().is_err());
    }
}
