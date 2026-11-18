use std::collections::HashMap;

use bincode::{Decode, Encode};
use rs_arrow::object::ColumnStat;

use crate::{
    StorageError,
    catalog::BINCODE_CONF,
    data::{DB_VERSION, footer::FooterEntry},
};

#[derive(Debug, Clone, PartialEq, Decode, Encode)]
pub struct SegmentFileRef {
    pub dat_path: String,
    pub footer_offset: usize,
    pub footer_len: u32,
    pub row_count: usize,
    pub chunk_count: usize,
    /// Set by Catalog during manifest registration under lock
    pub created_at_version: u64,
    /// Pre-parsed footers cached in memory for zero-disk-seek query planning
    pub footer_entries: Vec<FooterEntry>,
    pub validity_mask: Option<Vec<u64>>,
}

#[derive(Debug, Clone, PartialEq, Decode, Encode)]
pub struct TableManifest {
    /// Serialized table schema in bincode
    pub schema_bytes: Vec<u8>,
    pub segment_files: Vec<SegmentFileRef>,
    pub table_zonemap: HashMap<usize, ColumnStat>,
    pub total_rows: usize,
    pub version: u64,
}

impl TableManifest {
    pub fn new(schema_bytes: Vec<u8>) -> Self {
        Self {
            schema_bytes,
            segment_files: Vec::new(),
            table_zonemap: HashMap::new(),
            total_rows: 0,
            version: 1,
        }
    }
}

pub fn encode_manifest(manifest: &TableManifest) -> Result<Vec<u8>, StorageError> {
    let mut bytes = bincode::encode_to_vec(manifest, BINCODE_CONF).map_err(|e| {
        StorageError::CorruptFooter {
            segment_id: "?".into(),
            reason: format!("manifest serialization failed: {e}"),
        }
    })?;

    let len = bytes.len();
    bytes.reserve_exact(2);
    unsafe {
        bytes.set_len(len + 2);
    }
    bytes.copy_within(0..len, 2);
    bytes[0..2].copy_from_slice(&DB_VERSION.to_le_bytes());
    Ok(bytes)
}

pub fn decode_manifest(bytes: &[u8]) -> Result<TableManifest, StorageError> {
    if bytes.len() < 2 {
        return Err(StorageError::CorruptFooter {
            segment_id: "?".into(),
            reason: "manifest file too short".into(),
        });
    }

    let version = u16::from_le_bytes(bytes[0..2].try_into().unwrap());
    if version != DB_VERSION {
        return Err(StorageError::CorruptFooter {
            segment_id: "?".into(),
            reason: format!("unsupported manifest version: {version}"),
        });
    }

    let (manifest, _): (TableManifest, usize) =
        bincode::decode_from_slice(&bytes[2..], BINCODE_CONF).map_err(|e| {
            StorageError::CorruptFooter {
                segment_id: "?".into(),
                reason: format!("manifest deserialization failed: {e}"),
            }
        })?;

    Ok(manifest)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_manifest_encode_decode_roundtrip() {
        let mut manifest = TableManifest::new(vec![1, 2, 3, 4]);
        manifest.total_rows = 1000;
        manifest.version = 5;
        manifest
            .table_zonemap
            .insert(0, ColumnStat::Int64 { min: 10, max: 99 });
        manifest.segment_files.push(SegmentFileRef {
            dat_path: "users/segments/seg-1.dat".into(),
            footer_offset: 2048,
            footer_len: 120,
            row_count: 500,
            chunk_count: 1,
            created_at_version: 5,
            footer_entries: vec![],
            validity_mask: None,
        });

        let encoded = encode_manifest(&manifest).unwrap();
        assert!(encoded.len() > 2);
        assert_eq!(&encoded[0..2], &DB_VERSION.to_le_bytes());

        let decoded = decode_manifest(&encoded).unwrap();
        assert_eq!(decoded, manifest);
    }

    #[test]
    fn test_manifest_version_mismatch_and_short() {
        let manifest = TableManifest::new(vec![]);
        let mut encoded = encode_manifest(&manifest).unwrap();

        // Corrupt version
        encoded[0] = 0xFF;
        let err = decode_manifest(&encoded).unwrap_err();
        match err {
            StorageError::CorruptFooter { reason, .. } => {
                assert!(reason.contains("unsupported manifest version"));
            }
            _ => panic!("Expected CorruptFooter error"),
        }

        // Too short
        let err_short = decode_manifest(&[1]).unwrap_err();
        match err_short {
            StorageError::CorruptFooter { reason, .. } => {
                assert!(reason.contains("manifest file too short"));
            }
            _ => panic!("Expected CorruptFooter error"),
        }
    }
}
