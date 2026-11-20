use std::io;

use async_trait::async_trait;

pub use backend::{Config, FsBackend};
pub use catalog::Catalog;
pub use data::Data;
pub use rs_arrow as arrow;
use rs_arrow::DataType;
pub mod backend;
pub mod catalog;
pub mod data;
pub mod index;
pub mod maintenance;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TableMetadata {
    pub name: String,
    pub columns: Vec<(String, DataType)>,
}

/// Trait governing all Metadata Catalog operations (fetching, writing, dropping schemas).
#[async_trait]
pub trait CatalogStorage: Send + Sync {
    async fn fetch_table_meta(&self, table_name: &str) -> Result<TableMetadata, StorageError>;
    async fn register_table_meta(
        &self,
        table_name: &str,
        metadata: TableMetadata,
    ) -> Result<(), StorageError>;
    async fn drop_table_meta(&self, table_name: &str) -> Result<(), StorageError>;
}

/// State machine cursor for iterating over a table
#[async_trait]
pub trait StorageIterator: Send + Sync {
    /// Reads ONLY the metadata (zonemap min/max) for the next storage chunk.
    /// Returns `Ok(None)` when the table scan reaches EOF.
    /// The MessageType should be [`MessageType::MetaOnly`]
    async fn next_chunk_meta(&mut self) -> Result<Option<&[u8]>, StorageError>;

    /// Skips the data block for the current chunk (zero data I/O).
    fn skip_current_chunk(&mut self) -> Result<(), StorageError>;

    /// Reads and returns the compressed data bytes ONLY if the chunk survives pruning
    /// The MessageType should be [`MessageType::RecordBatch`]
    async fn read_current_chunk_data(&mut self) -> Result<&[u8], StorageError>;
}

/// Trait governing all Columnar Data operations (scanning, writing, appending batches).
#[async_trait]
pub trait DataStorage: Send + Sync {
    /// This method always returns the complete chunk the query engine
    /// need to do all column pruning itself.
    /// len(projection) == 0 means client want to request all columns
    async fn new_iterator(
        &self,
        table_name: &str,
        projection: &[usize],
    ) -> Result<Box<dyn StorageIterator>, StorageError>;

    /// Forces whatever is currently buffered for `table_name` to seal
    /// into a durable, catalog-visible segment, regardless of whether
    /// the normal seal threshold has been reached
    async fn flush(&self, table_name: &str) -> Result<(), StorageError>;

    /// Append chunk into the Data Storage engine. The chunk must be
    /// IPC encoded arrow, and has [`MessageType::RecordBatch`]
    async fn append_chunk(&self, table_name: &str, chunk: Vec<u8>) -> Result<(), StorageError>;

    /// Returns the total number of storage chunks in the table
    async fn get_chunk_count(&self, table_name: &str) -> Result<usize, StorageError>;
}

#[derive(thiserror::Error, Debug)]
pub enum StorageError {
    #[error("table not found: {0}")]
    TableNotFound(String),

    #[error("manifest CAS conflict: expected version was modified concurrently")]
    ManifestConflict { attempts: u32 },

    #[error("corrupt footer in segment {segment_id}: {reason}")]
    CorruptFooter { segment_id: String, reason: String },

    #[error("arrow IPC error: {0}")]
    Arrow(String),

    #[error("iterator used after EOF")]
    IteratorExhausted,

    #[error("invalid byte range: {start}..{end} exceeds file length {len}")]
    InvalidRange { start: usize, end: usize, len: u64 },

    #[error("table lock error: {0}")]
    LockError(String),

    #[error("io error: {0}")]
    Io(#[from] io::Error),

    #[error("uncategorised error")]
    Unknown,
}
