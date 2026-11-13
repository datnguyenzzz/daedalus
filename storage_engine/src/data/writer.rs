use std::mem;

use rs_arrow::SchemaRef;

use crate::{FsBackend, StorageError, catalog::SegmentFileRef};

const SEAL_SIZE: usize = 64 * 1024 * 1024;

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

    pub fn should_flush(&self) -> bool {
        self.total_bytes > SEAL_SIZE
    }

    pub fn take(&mut self) -> Vec<Vec<u8>> {
        self.total_bytes = 0;
        mem::take(&mut self.chunks)
    }
}

pub struct SegmentWriter;

impl SegmentWriter {
    /// Seals an array of chunks into an immutable segment .dat file
    pub async fn seal(
        fs: &FsBackend,
        table_name: &str,
        chunks: Vec<Vec<u8>>,
        schema: SchemaRef,
        index_key_column: Option<usize>,
    ) -> Result<SegmentFileRef, StorageError> {
        todo!()
    }
}
