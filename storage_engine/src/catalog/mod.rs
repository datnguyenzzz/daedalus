use crate::{CatalogStorage, StorageError, TableMetadata, data::footer::FooterEntry};
use async_trait::async_trait;

pub struct Catalog {}

#[async_trait]
impl CatalogStorage for Catalog {
    async fn fetch_table_meta(&self, _table_name: &str) -> Result<TableMetadata, StorageError> {
        todo!()
    }
    async fn register_table_meta(
        &self,
        _table_name: &str,
        _metadata: TableMetadata,
    ) -> Result<(), StorageError> {
        todo!()
    }
    async fn drop_table_meta(&self, _table_name: &str) -> Result<(), StorageError> {
        todo!()
    }
}

pub struct SegmentFileRef {
    pub dat_path: String,
    pub footer_offset: u64,
    pub footer_len: u32,
    pub row_count: usize,
    pub chunk_count: usize,
    /// Set by Catalog during manifest registration under lock
    pub created_at_version: u64,
    /// Pre-parsed footers cached in memory for zero-disk-seek query planning
    pub footer_entries: Vec<FooterEntry>,
    pub validity_mask: Option<Vec<u64>>,
}
