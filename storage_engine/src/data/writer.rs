use std::{
    fs::{self, File},
    io::{self, BufWriter, Write},
    mem,
};

use rs_arrow::SchemaRef;
use uuid::Uuid;

use crate::{
    FsBackend, StorageError,
    catalog::SegmentFileRef,
    data::{
        extract_meta_chunk,
        footer::{FooterEntry, encode_footer},
    },
};

const SEAL_SIZE: usize = 64 * 1024 * 1024;

#[derive(Default)]
pub struct WriteBuffer {
    chunks: Vec<Vec<u8>>,
    pub total_bytes: usize,
}

impl WriteBuffer {
    pub fn push(&mut self, chunk: Vec<u8>) -> Result<(), StorageError> {
        self.total_bytes += chunk.len();
        self.chunks.push(chunk);
        Ok(())
    }

    pub fn should_seal(&self) -> bool {
        self.total_bytes > SEAL_SIZE
    }

    pub fn take(&mut self) -> Vec<Vec<u8>> {
        self.total_bytes = 0;
        mem::take(&mut self.chunks)
    }
}

pub struct SegmentWriter;

impl SegmentWriter {
    pub fn segment_path(table_name: &str, seg_id: Uuid) -> String {
        format!("{table_name}/segments/seg-{seg_id}.dat")
    }

    /// Seals an array of chunks into an immutable segment .dat file
    pub async fn seal(
        be: &FsBackend,
        table_name: &str,
        chunks: Vec<Vec<u8>>,
        schema: SchemaRef,
        _index_key_column: Option<usize>,
    ) -> Result<SegmentFileRef, StorageError> {
        if chunks.is_empty() {
            return Err(StorageError::Unknown);
        }
        let seg_id = Uuid::new_v4();
        let seg_rel = Self::segment_path(table_name, seg_id);
        let dest_path = be.resolve(&seg_rel);

        let parent_dir = dest_path.parent().ok_or_else(|| {
            StorageError::Io(io::Error::new(
                io::ErrorKind::InvalidInput,
                "missing parent directory",
            ))
        })?;

        if !parent_dir.exists() {
            fs::create_dir_all(parent_dir)?;
        }

        let tmp_path = parent_dir.join(format!(".tmp-seg-{}", Uuid::new_v4()));
        let mut footer_entries = Vec::with_capacity(chunks.len());
        let mut offset: usize = 0;
        let mut total_rows = 0;

        // stream the chunks to temp file
        let write_result = (|| -> Result<(usize, usize), StorageError> {
            let tmp_file = File::create(&tmp_path)?;
            let mut writer = BufWriter::with_capacity(64 * 1024, tmp_file);
            for chunk in chunks {
                let chunk_len = chunk.len();
                let (num_rows, meta_bytes, column_pointers) =
                    extract_meta_chunk(&chunk, offset, &schema)?;
                total_rows += num_rows;
                footer_entries.push(FooterEntry {
                    offset: offset as u64,
                    len: chunk_len as u64,
                    num_rows: num_rows as u32,
                    meta_bytes,
                    column_pointers,
                });

                writer.write_all(&chunk)?;
                offset += chunk_len;
            }

            let footer_offset = offset;
            let footer_bytes = encode_footer(&footer_entries);
            writer.write_all(&footer_bytes)?;
            writer.flush()?;

            if be.fsync {
                writer.get_ref().sync_all()?;
            }

            Ok((footer_offset, footer_bytes.len()))
        })();

        let (footer_offset, footer_len) = match write_result {
            Ok(off) => off,
            Err(e) => {
                let _ = fs::remove_file(&tmp_path);
                return Err(e);
            }
        };

        // atomic rename
        if let Err(e) = fs::rename(&tmp_path, &dest_path) {
            let _ = fs::remove_file(&tmp_path);
            return Err(StorageError::Io(e));
        }

        // directory fsync for crash durability
        #[cfg(unix)]
        if be.fsync {
            if let Ok(dir_file) = File::open(parent_dir) {
                let _ = dir_file.sync_all();
            }
        }

        // created_at_version is set to 0 here;
        // Catalog::register_segment sets it to current_version + 1 under lock
        Ok(SegmentFileRef {
            dat_path: seg_rel,
            footer_offset,
            footer_len: footer_len as u32,
            row_count: total_rows,
            chunk_count: footer_entries.len(),
            created_at_version: 0,
            footer_entries,
            validity_mask: None,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{Config, data::footer::decode_footer};
    use rs_arrow::{
        DataType, Field, RecordBatch, Schema,
        array::{PrimitiveArray, StringArray},
        ipc::record_batch::Writer as ArrowWriter,
    };
    use std::sync::Arc;

    #[test]
    fn test_write_buffer_push_and_should_seal() {
        let mut buf = WriteBuffer::default();
        assert_eq!(buf.total_bytes, 0);
        assert!(!buf.should_seal());

        // Push small chunk
        let chunk1 = vec![0u8; 1000];
        buf.push(chunk1).unwrap();
        assert_eq!(buf.total_bytes, 1000);
        assert!(!buf.should_seal());

        // Push large chunk crossing SEAL_SIZE (64 MB)
        let large_chunk = vec![1u8; SEAL_SIZE];
        buf.push(large_chunk).unwrap();
        assert_eq!(buf.total_bytes, 1000 + SEAL_SIZE);
        assert!(buf.should_seal());

        // Drain buffer
        let chunks = buf.take();
        assert_eq!(chunks.len(), 2);
        assert_eq!(buf.total_bytes, 0);
        assert!(!buf.should_seal());
    }

    #[tokio::test]
    async fn test_segment_writer_seal_roundtrip() {
        let temp_dir = tempfile::tempdir().unwrap();
        let be = FsBackend::try_new(Some(Config {
            root_dir: temp_dir.path().to_path_buf(),
            fsync: false,
        }))
        .unwrap();

        let schema = Arc::new(Schema::new(vec![
            Field {
                name: "id".to_string(),
                data_type: DataType::Int32,
                nullable: false,
            },
            Field {
                name: "name".to_string(),
                data_type: DataType::Utf8,
                nullable: false,
            },
        ]));

        // Build 2 batches
        let mut chunks = Vec::new();
        for i in 0..2 {
            let id_col = Arc::new(PrimitiveArray::from(vec![i * 10, i * 10 + 1]));
            let name_col = Arc::new(StringArray::from(vec!["foo", "bar"]));
            let batch = RecordBatch::try_new(schema.clone(), vec![id_col, name_col]).unwrap();

            let mut chunk_bytes = Vec::new();
            {
                let mut writer = ArrowWriter::new(&mut chunk_bytes);
                writer.write(&batch).unwrap();
            }
            chunks.push(chunk_bytes);
        }

        let seg_ref = SegmentWriter::seal(&be, "users", chunks, schema, None)
            .await
            .unwrap();

        assert_eq!(seg_ref.chunk_count, 2);
        assert_eq!(seg_ref.row_count, 4);
        assert_eq!(seg_ref.footer_entries.len(), 2);

        // Read footer directly from disk to verify persistence
        let footer_range =
            seg_ref.footer_offset..(seg_ref.footer_offset + seg_ref.footer_len as usize);
        let footer_bytes = be
            .read_range(&seg_ref.dat_path, footer_range)
            .await
            .unwrap();
        let decoded_entries = decode_footer(&footer_bytes).unwrap();
        assert_eq!(decoded_entries.len(), 2);
        assert_eq!(decoded_entries[0].num_rows, 2);
        assert_eq!(decoded_entries[1].num_rows, 2);

        // Verify temporary files are cleaned up
        let seg_dir = be.resolve("users/segments");
        let entries: Vec<_> = fs::read_dir(seg_dir)
            .unwrap()
            .filter_map(|e| e.ok())
            .map(|e| e.file_name().to_string_lossy().into_owned())
            .collect();
        assert_eq!(entries.len(), 1);
        assert!(!entries[0].starts_with(".tmp-"));
    }
}
