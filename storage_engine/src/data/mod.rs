pub mod footer;
pub mod writer;

use std::sync::{Arc, Mutex};

use async_trait::async_trait;
use bincode::{Decode, Encode};
use dashmap::DashMap;
use rs_arrow::{
    DataType, Schema, SchemaRef,
    ipc::record_batch::{MessageReader, Writer},
    object::ColumnStat,
};

use crate::{
    Catalog, CatalogStorage, DataStorage, FsBackend, StorageError, StorageIterator,
    data::writer::{SegmentWriter, WriteBuffer},
};

pub const DB_MAGIC: [u8; 2] = [0x4e, 0x47];
pub const DB_VERSION: u16 = 1;

pub struct Data {
    be: Arc<FsBackend>,
    catalog: Arc<Catalog>,
    schema_cache: DashMap<String, SchemaRef>,
    write_buffers: DashMap<String, Mutex<WriteBuffer>>, // write buffer per table
}

impl Data {
    pub fn try_new(be: Arc<FsBackend>) -> Result<Self, StorageError> {
        let catalog = Arc::new(Catalog::new(be.clone()));
        Ok(Self {
            be,
            catalog,
            write_buffers: DashMap::new(),
            schema_cache: DashMap::new(),
        })
    }
}

impl Data {
    async fn get_table_schema(&self, table_name: &str) -> Result<SchemaRef, StorageError> {
        if let Some(cached) = self.schema_cache.get(table_name) {
            return Ok(cached.clone());
        }

        let meta = self.catalog.fetch_table_meta(table_name).await?;
        let fields: Vec<rs_arrow::Field> = meta
            .columns
            .into_iter()
            .map(|(name, data_type)| rs_arrow::Field {
                name,
                data_type,
                nullable: true,
            })
            .collect();
        let schema = Arc::new(Schema::new(fields));

        self.schema_cache
            .insert(table_name.to_string(), schema.clone());
        Ok(schema)
    }
}

#[async_trait]
impl DataStorage for Data {
    async fn new_iterator(
        &self,
        _table_name: &str,
        _projection: &[usize],
    ) -> Result<Box<dyn StorageIterator>, StorageError> {
        todo!()
    }

    async fn flush(&self, table_name: &str) -> Result<(), StorageError> {
        let chunks = {
            if let Some(buffer) = self.write_buffers.get(table_name) {
                let mut wbuf = buffer.lock().unwrap();
                if wbuf.total_bytes == 0 {
                    return Ok(());
                }
                wbuf.take()
            } else {
                return Ok(());
            }
        };

        let schema = self.get_table_schema(table_name).await?;
        let seg_ref = SegmentWriter::seal(&self.be, table_name, chunks, schema, None).await?;

        self.catalog.register_segment(table_name, seg_ref).await?;
        Ok(())
    }

    /// Appends a chunk into the table buffer and seals if policy threshold is reached
    async fn append_chunk(&self, table_name: &str, chunk: Vec<u8>) -> Result<(), StorageError> {
        let should_seal = {
            let buffer = if let Some(buffer) = self.write_buffers.get(table_name) {
                buffer
            } else {
                self.write_buffers
                    .entry(table_name.to_string())
                    .or_insert_with(|| Mutex::new(WriteBuffer::default()))
                    .downgrade()
            };
            let mut wbuf = buffer.lock().unwrap();
            wbuf.push(chunk)?;
            wbuf.should_seal()
        };

        if should_seal {
            self.flush(table_name).await?
        }

        Ok(())
    }

    async fn get_chunk_count(&self, _table_name: &str) -> Result<usize, StorageError> {
        todo!()
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Decode, Encode)]
pub struct ColumnPointer {
    /// Absolute byte offset in the sealed segment file
    pub offset: u64,
    /// Total compressed byte length of this column
    pub length: u32,
    /// Individual segment lengths (e.g. validity, values, offsets)
    pub segment_lengths: Vec<u32>,
}

/// extract meta (without decoding the meta block) from an 
/// IPC-encoded [`RecordBatch`] and encode it to IPC format
pub fn extract_meta_chunk(
    chunk: &[u8],
    offset: usize,
    schema: &Schema,
) -> Result<(usize, Vec<u8>, Vec<ColumnPointer>), StorageError> {
    let mut reader = MessageReader::new(chunk);
    let fb_msg = reader
        .try_next_meta_only()
        .map_err(|e| StorageError::Arrow(e.to_string()))?
        .ok_or_else(|| StorageError::CorruptFooter {
            segment_id: "?".into(),
            reason: "empty chunk".into(),
        })?;
    let fb_batch =
        fb_msg
            .header_as_fb_record_batch()
            .ok_or_else(|| StorageError::CorruptFooter {
                segment_id: "?".into(),
                reason: "not a RecordBatch message".into(),
            })?;
    let num_rows = fb_batch.length();
    let fb_nodes = fb_batch
        .nodes()
        .ok_or_else(|| StorageError::CorruptFooter {
            segment_id: "?".into(),
            reason: "missing nodes in batch header".into(),
        })?;

    let mut nodes = Vec::with_capacity(fb_nodes.len());
    for n in fb_nodes.iter() {
        let stat = ColumnStat::decode(n.stats_byte().bytes()).ok_or_else(|| {
            StorageError::CorruptFooter {
                segment_id: "?".into(),
                reason: "failed to decode ColumnStat".into(),
            }
        })?;
        nodes.push((n.length(), n.null_count(), stat));
    }

    let mut meta_buf = Vec::new();
    Writer::new(&mut meta_buf)
        .write_meta_node(nodes, num_rows)
        .map_err(|e| StorageError::Arrow(e.to_string()))?;

    let body_offset = offset + (chunk.len() - fb_msg.body_length() as usize);
    let fb_segments = fb_batch
        .segments()
        .ok_or_else(|| StorageError::CorruptFooter {
            segment_id: "?".into(),
            reason: "missing segments in batch header".into(),
        })?;

    let mut pointers = Vec::with_capacity(schema.fields().len());
    let mut cur = 0;

    for field in schema.fields() {
        let mut col_start_offset = None;
        let num_segs = match field.data_type {
            DataType::Utf8 => 3,
            _ => 2,
        };

        let mut col_len = 0;
        let mut segment_lengths = Vec::with_capacity(num_segs);
        for _ in 0..num_segs {
            if cur == fb_segments.len() {
                return Err(StorageError::Arrow("segment cursor is out-of-range".into()));
            }
            let seg = fb_segments.get(cur);
            let seg_len = seg.length() as u32;
            segment_lengths.push(seg_len);
            if seg_len > 0 {
                if col_start_offset.is_none() {
                    col_start_offset = Some(seg.offset() as u64);
                }
                col_len += seg_len;
            }
            cur += 1;
        }

        pointers.push(ColumnPointer {
            offset: (body_offset as u64) + col_start_offset.unwrap_or(0),
            length: col_len,
            segment_lengths,
        })
    }

    Ok((num_rows as usize, meta_buf, pointers))
}

#[cfg(test)]
mod tests {
    use super::*;
    use rs_arrow::{
        Field, RecordBatch, Schema,
        array::{PrimitiveArray, StringArray},
        ipc::record_batch::MessageType,
    };
    use std::sync::Arc;

    #[test]
    fn test_extract_meta_and_column_pointers() {
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

        let id_col = Arc::new(PrimitiveArray::from(vec![10i32, 20, 30]));
        let name_col = Arc::new(StringArray::from(vec!["alpha", "beta", "gamma"]));
        let batch = RecordBatch::try_new(schema.clone(), vec![id_col, name_col]).unwrap();

        let mut chunk = Vec::new();
        {
            let mut writer = Writer::new(&mut chunk);
            writer.write(&batch).unwrap();
        }

        // 1. Test extract_meta_chunk
        let (_, meta_bytes, _) = extract_meta_chunk(&chunk, 0, &schema).unwrap();
        let mut reader = MessageReader::new(&meta_bytes[..]);
        let fb_msg = reader.try_next_meta_only().unwrap().unwrap();
        assert_eq!(fb_msg.header_type(), MessageType::MetaOnly as u8);

        let fb_batch = fb_msg.header_as_fb_record_batch().unwrap();
        assert_eq!(fb_batch.length(), 3);
        let nodes = fb_batch.nodes().unwrap();
        assert_eq!(nodes.len(), 2);

        let stat0 = ColumnStat::decode(nodes.get(0).stats_byte().bytes()).unwrap();
        assert_eq!(stat0, ColumnStat::Int64 { min: 10, max: 30 });

        let stat1 = ColumnStat::decode(nodes.get(1).stats_byte().bytes()).unwrap();
        assert_eq!(
            stat1,
            ColumnStat::Utf8 {
                min: "alpha".to_string(),
                max: "gammc".to_string(),
            }
        );

        // 2. Test extract_column_pointer
        let chunk_file_offset = 5000;
        let (_, _, pointers) = extract_meta_chunk(&chunk, chunk_file_offset, &schema).unwrap();
        assert_eq!(pointers.len(), 2);

        // Column 0 should have 2 segments (nulls, values)
        assert_eq!(pointers[0].segment_lengths.len(), 2);
        // Column 1 should have 3 segments (nulls, offsets, data)
        assert_eq!(pointers[1].segment_lengths.len(), 3);

        // Crucial: Column 1 offset must be greater than Column 0 offset (proving col_start_offset was reset!)
        assert!(pointers[0].offset >= chunk_file_offset as u64);
        assert!(pointers[1].offset > pointers[0].offset);
        assert!(pointers[0].length > 0);
        assert!(pointers[1].length > 0);
    }
}
