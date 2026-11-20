use std::{
    fs::{self, File, OpenOptions},
    io::{Error, ErrorKind, Read, Seek, SeekFrom, Write},
    ops::Range,
    path::PathBuf,
};

use fd_lock::RwLock;
use uuid::Uuid;

use crate::StorageError;

/// Used for merging ranges separated by small gaps (< 64 KB) into
/// single sequential reads to reduce syscall overhead
const COALESCE_GAP_THRESHOLD: usize = 64 * 1024; // 64 KiB

pub struct Config {
    pub root_dir: PathBuf,
    pub fsync: bool,
}

pub struct FsBackend {
    pub root_dir: PathBuf,
    pub fsync: bool,
}

impl FsBackend {
    pub fn try_new(conf: Option<Config>) -> Result<Self, StorageError> {
        let conf = match conf {
            None => Config {
                root_dir: PathBuf::from("/tmp"),
                fsync: false,
            },
            Some(v) => v,
        };
        fs::create_dir_all(&conf.root_dir)?;
        Ok(Self {
            root_dir: conf.root_dir,
            fsync: conf.fsync,
        })
    }

    pub fn resolve(&self, rel: &str) -> PathBuf {
        self.root_dir.join(rel)
    }

    pub fn write_atomic_sync(&self, rel: &str, body: &[u8]) -> Result<(), StorageError> {
        let path = self.resolve(rel);
        let parent_dir = path.parent().ok_or_else(|| {
            StorageError::Io(Error::new(
                ErrorKind::InvalidInput,
                "Path has no parent directory",
            ))
        })?;
        if !parent_dir.exists() {
            fs::create_dir_all(parent_dir)?;
        }
        let tmp_path = parent_dir.join(format!("temp-{}", Uuid::new_v4()));
        let mut f = File::create(&tmp_path)?;
        f.write_all(body)?;
        if self.fsync {
            f.sync_all()?;
        }

        fs::rename(&tmp_path, &path)?;
        #[cfg(unix)]
        if self.fsync {
            if let Ok(dir_file) = File::open(parent_dir) {
                dir_file.sync_all()?;
            }
        }

        Ok(())
    }

    pub async fn write_atomic(&self, rel: &str, body: Vec<u8>) -> Result<(), StorageError> {
        self.write_atomic_sync(rel, &body)
    }

    pub async fn read_range_into(
        &self,
        rel: &str,
        range: Range<usize>,
        buf: &mut Vec<u8>,
    ) -> Result<(), StorageError> {
        let path = self.resolve(rel);
        let mut f = File::open(path)?;
        let file_len = f.metadata()?.len();
        if range.end as u64 > file_len || range.start > range.end {
            return Err(StorageError::InvalidRange {
                start: range.start,
                end: range.end,
                len: file_len,
            });
        }
        f.seek(SeekFrom::Start(range.start as u64))?;
        let len = range.end - range.start;
        buf.resize(len, 0);
        f.read_exact(buf)?;
        Ok(())
    }

    pub async fn read_range(
        &self,
        rel: &str,
        range: Range<usize>,
    ) -> Result<Vec<u8>, StorageError> {
        let mut res = Vec::new();
        self.read_range_into(rel, range, &mut res).await?;
        Ok(res)
    }

    /// ranges are expected to be disjoint and sorted in ascending order
    pub async fn read_ranges(
        &self,
        rel: &str,
        ranges: &[Range<usize>],
    ) -> Result<Vec<Vec<u8>>, StorageError> {
        if ranges.len() == 0 {
            return Ok(Vec::new());
        }
        let path = self.resolve(rel);
        let mut f = File::open(path)?;
        let file_len = f.metadata()?.len();
        let mut results = Vec::with_capacity(ranges.len());
        let mut start = ranges[0].start;
        let mut end = ranges[0].end;

        let mut do_read = |start: usize, end: usize| -> Result<(), StorageError> {
            if end as u64 > file_len || start > end {
                return Err(StorageError::InvalidRange {
                    start: start,
                    end: end,
                    len: file_len,
                });
            }

            f.seek(SeekFrom::Start(start as u64))?;
            let len = end - start;
            let mut res = vec![0u8; len];
            f.read_exact(&mut res)?;
            results.push(res);
            Ok(())
        };

        for i in 1..ranges.len() {
            if ranges[i].end - start < COALESCE_GAP_THRESHOLD {
                end = ranges[i].end;
                continue;
            }

            do_read(start, end)?;
            start = ranges[i].start;
            end = ranges[i].end;
        }

        do_read(start, end)?;
        Ok(results)
    }

    pub async fn with_table_lock<T: Send + 'static>(
        &self,
        table: &str,
        f: impl FnOnce() -> Result<T, StorageError> + Send + 'static,
    ) -> Result<T, StorageError> {
        let lock_path = self.resolve(&format!("{table}/.lock"));
        if let Some(parent) = lock_path.parent() {
            fs::create_dir_all(parent)?;
        }

        let file = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .open(&lock_path)?;

        // Acquire exclusive blocking advisory write lock
        let mut lock = RwLock::new(file);
        let _guard = lock
            .write()
            .map_err(|e| StorageError::LockError(e.to_string()))?;

        f()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_try_new_and_resolve() {
        let temp_dir = tempfile::tempdir().unwrap();
        let root = temp_dir.path().join("data_root");
        let backend = FsBackend::try_new(Some(Config {
            root_dir: root.clone(),
            fsync: false,
        }))
        .unwrap();

        assert!(root.exists());
        assert_eq!(
            backend.resolve("users/manifest.bin"),
            root.join("users/manifest.bin")
        );
    }

    #[tokio::test]
    async fn test_write_atomic_create_and_overwrite() {
        let temp_dir = tempfile::tempdir().unwrap();
        let backend = FsBackend::try_new(Some(Config {
            root_dir: temp_dir.path().to_path_buf(),
            fsync: true,
        }))
        .unwrap();

        let rel_path = "tables/users/manifest-000001.bin";

        // 1. Initial atomic write
        backend
            .write_atomic(rel_path, b"initial_manifest_data".to_vec())
            .await
            .unwrap();

        let read = backend.read_range(rel_path, 0..21).await.unwrap();
        assert_eq!(&read, b"initial_manifest_data");

        // 2. Overwrite atomically
        backend
            .write_atomic(rel_path, b"updated_manifest_data".to_vec())
            .await
            .unwrap();

        let read_updated = backend.read_range(rel_path, 0..21).await.unwrap();
        assert_eq!(&read_updated, b"updated_manifest_data");

        // 3. Verify no temporary files remain in directory
        let parent = backend.resolve("tables/users");
        let entries: Vec<_> = fs::read_dir(parent)
            .unwrap()
            .filter_map(|e| e.ok())
            .map(|e| e.file_name().to_string_lossy().into_owned())
            .collect();
        assert_eq!(entries, vec!["manifest-000001.bin"]);
    }

    #[tokio::test]
    async fn test_read_range_success_and_errors() {
        let temp_dir = tempfile::tempdir().unwrap();
        let backend = FsBackend::try_new(Some(Config {
            root_dir: temp_dir.path().to_path_buf(),
            fsync: false,
        }))
        .unwrap();

        let rel = "data/chunk.dat";
        backend
            .write_atomic(rel, b"0123456789abcdef".to_vec())
            .await
            .unwrap();

        // Success: read middle slice
        let slice = backend.read_range(rel, 4..10).await.unwrap();
        assert_eq!(&slice, b"456789");

        // Error: start > end
        let err_inverted = backend.read_range(rel, 10..5).await.unwrap_err();
        assert!(matches!(err_inverted, StorageError::InvalidRange { .. }));

        // Error: end > file_len (file has 16 bytes, range requests up to 20)
        let err_oob = backend.read_range(rel, 10..20).await.unwrap_err();
        match err_oob {
            StorageError::InvalidRange { start, end, len } => {
                assert_eq!(start, 10);
                assert_eq!(end, 20);
                assert_eq!(len, 16);
            }
            _ => panic!("Expected InvalidRange error"),
        }
    }

    #[tokio::test]
    async fn test_read_ranges_empty_and_coalesce() {
        let temp_dir = tempfile::tempdir().unwrap();
        let backend = FsBackend::try_new(Some(Config {
            root_dir: temp_dir.path().to_path_buf(),
            fsync: false,
        }))
        .unwrap();

        let rel = "data/seg.dat";

        // Empty ranges returns empty results
        let empty = backend.read_ranges(rel, &[]).await.unwrap();
        assert!(empty.is_empty());

        // Create a file large enough to test both coalesced and non-coalesced gaps
        // Buffer: 10 bytes at 0, 10 bytes at 100 (< 64KB gap), 10 bytes at 100_000 (>= 64KB gap)
        let mut data = vec![0u8; 100_020];
        data[0..10].copy_from_slice(b"chunk_zero");
        data[100..110].copy_from_slice(b"chunk_ones");
        data[100_000..100_010].copy_from_slice(b"chunk_twos");

        backend.write_atomic(rel, data).await.unwrap();

        // 1. Two ranges separated by small gap (< 64KB): should coalesce into 1 read buffer
        let small_gap_ranges = vec![0..10, 100..110];
        let coalesced = backend.read_ranges(rel, &small_gap_ranges).await.unwrap();
        assert_eq!(coalesced.len(), 1);
        assert_eq!(coalesced[0].len(), 110);
        assert_eq!(&coalesced[0][0..10], b"chunk_zero");
        assert_eq!(&coalesced[0][100..110], b"chunk_ones");

        // 2. Ranges separated by large gap (>= 64KB): should produce 2 read buffers
        let large_gap_ranges = vec![0..10, 100_000..100_010];
        let split = backend.read_ranges(rel, &large_gap_ranges).await.unwrap();
        assert_eq!(split.len(), 2);
        assert_eq!(&split[0], b"chunk_zero");
        assert_eq!(&split[1], b"chunk_twos");

        // 3. Out of bounds range returns error
        let oob_ranges = vec![0..10, 100_000..200_000];
        let oob_err = backend.read_ranges(rel, &oob_ranges).await.unwrap_err();
        assert!(matches!(oob_err, StorageError::InvalidRange { .. }));
    }

    #[tokio::test]
    async fn test_with_table_lock() {
        let temp_dir = tempfile::tempdir().unwrap();
        let backend = FsBackend::try_new(Some(Config {
            root_dir: temp_dir.path().to_path_buf(),
            fsync: false,
        }))
        .unwrap();

        let val = backend
            .with_table_lock("orders", || Ok(12345))
            .await
            .unwrap();
        assert_eq!(val, 12345);

        // Verify lock file was created in the table directory
        assert!(backend.resolve("orders/.lock").exists());
    }
}
