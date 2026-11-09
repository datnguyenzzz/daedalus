pub mod embed;
pub mod external;

pub use embed::{WrappedCatalog, WrappedStorage, WrappedStorageIterator};

use anyhow::Result;
use async_trait::async_trait;
use rs_arrow::{DataType, ipc::record_batch::MessageReader};

pub use crate::arrow::object::ColumnStat;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TableMetadata {
    pub name: String,
    pub columns: Vec<(String, DataType)>,
}

#[derive(Clone)]
pub struct ChunkMetaData {
    pub num_rows: usize,
    pub column_stats: Vec<Option<ColumnStat>>,
}

impl ChunkMetaData {
    pub fn try_new(chunk: Vec<u8>) -> Result<Self> {
        let mut msg_reader = MessageReader::new(&chunk[..]);
        while let Some(msg) = msg_reader.try_next_meta_only()? {
            if let Some(batch) = msg.header_as_fb_record_batch() {
                let num_rows = batch.length() as usize;
                let mut column_stats = Vec::new();
                if let Some(nodes) = batch.nodes() {
                    column_stats.reserve(nodes.len());
                    for node in nodes.iter() {
                        let stat = ColumnStat::decode(node.stats_byte().bytes());
                        column_stats.push(stat);
                    }
                }
                return Ok(Self {
                    num_rows,
                    column_stats,
                });
            }
        }
        anyhow::bail!("Failed to parse RecordBatch header from chunk");
    }

    #[inline]
    pub fn get_column_stat(&self, col_idx: usize) -> Option<&ColumnStat> {
        self.column_stats.get(col_idx).and_then(|s| s.as_ref())
    }
}

/// Trait governing all Metadata Catalog operations (fetching, writing, dropping schemas).
#[async_trait]
pub trait CatalogStorage: Send + Sync {
    async fn fetch_table_meta(&self, table_name: &str) -> Result<TableMetadata>;
    async fn register_table_meta(&self, table_name: &str, metadata: TableMetadata) -> Result<()>;
    async fn drop_table_meta(&self, table_name: &str) -> Result<()>;
}

/// State machine cursor for iterating over a table
#[async_trait]
pub trait StorageIterator: Send + Sync {
    /// Reads ONLY the metadata (zonemap min/max) for the next storage chunk.
    /// Returns `Ok(None)` when the table scan reaches EOF.
    /// The MessageType should be [`MessageType::MetaOnly`]
    async fn next_chunk_meta(&mut self) -> Result<Option<Vec<u8>>>;

    /// Skips the data block for the current chunk (zero data I/O).
    fn skip_current_chunk(&mut self) -> Result<()>;

    /// Reads and returns the compressed data bytes ONLY if the chunk survives pruning
    /// The MessageType should be [`MessageType::RecordBatch`]
    async fn read_current_chunk_data(&mut self) -> Result<Vec<u8>>;
}

/// Trait governing all Columnar Data operations (scanning, writing, appending batches).
#[async_trait]
pub trait DataStorage: Send + Sync {
    /// This method always returns the complete chunk the query engine
    /// need to do all column pruning itself
    async fn new_iterator(
        &self,
        table_name: &str,
        projection: &[usize],
    ) -> Result<Box<dyn StorageIterator>>;

    /// Forces whatever is currently buffered for `table_name` to seal
    /// into a durable, catalog-visible segment, regardless of whether
    /// the normal seal threshold has been reached
    async fn flush(&self, table_name: &str) -> Result<()>;

    /// Append chunk into the Data Storage engine. The chunk must be
    /// IPC encoded arrow, and has [`MessageType::RecordBatch`]
    async fn append_chunk(&self, table_name: &str, chunk: Vec<u8>) -> Result<()>;

    /// Returns the total number of storage chunks in the table
    async fn get_chunk_count(&self, table_name: &str) -> Result<usize>;
}
