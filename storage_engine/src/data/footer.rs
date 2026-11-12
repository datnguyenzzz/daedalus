/*
One sealed segment file = many chunks + one footer:
    [chunk_0 bytes][chunk_1 bytes]...[chunk_N-1 bytes]
    [FOOTER]

On-disk layout for a sealed segment footer
    [MAGIC: 2 bytes]                    = 0x4E, 0x47 ("NG")
    [VERSION: 2 bytes]                  = 0x01, 0x00 (Version 1)
    [ENTRY_COUNT: 4 bytes]              = u32 little-endian
    --- Repeated for each [`FooterEntry`] (0..ENTRY_COUNT) ---
        [offset: 8 bytes]               = u64 chunk start in .dat file
        [len: 8 bytes]                  = u64 total chunk byte length
        [num_rows: 4 bytes]             = u32 rows in chunk
        [meta_len: 4 bytes]             = u32 length of meta_only_bytes
        [meta_bytes: meta_len bytes]    = raw MetaOnly FlatBuffer
        [num_cols: 2 bytes]             = u16 count of columns
    --- Repeated for each [`ColumnPointer`] (0..num_cols) ---
        [file_offset: 8 bytes]          = u64 column start in .dat file
        [length: 4 bytes]               = u32 column compressed bytes
        [num_segs: 2 bytes]             = u16 number of sub-segments
    --- Repeated for each segment (0..num_segs) ---
        [seg_len: 4 bytes]              = u32 segment length
    --- Trailing Checksum ---
        [CRC32: 4 bytes]                = u32 little-endian checksum over [MAGIC .. end of entries]
*/

use crc::{CRC_32_ISO_HDLC, Crc};

use crate::{StorageError, data::ColumnPointer};

const FOOTER_MAGIC: [u8; 2] = [0x4e, 0x47];
const FOOTER_VERSION: u16 = 1;
const CRC32: Crc<u32> = Crc::<u32>::new(&CRC_32_ISO_HDLC);
const FOOTER_FIXED_SIZE: usize = /*prefix*/ 8 + /*trailing*/4;
const ENTRY_FIXED_SIZE: usize = 24;
const POINTER_FIXED_SIZE: usize = 14;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FooterEntry {
    pub offset: u64,
    pub len: u64,
    pub num_rows: u32,       // internal bookkeeping only
    pub meta_bytes: Vec<u8>, // exactly what next_chunk_meta hands back
    pub column_pointers: Vec<ColumnPointer>,
}

pub fn encode_footer(entries: &[FooterEntry]) -> Vec<u8> {
    let mut est_size = FOOTER_FIXED_SIZE;
    for entry in entries {
        est_size += entry.meta_bytes.len() + ENTRY_FIXED_SIZE;
        for pointer in &entry.column_pointers {
            est_size += pointer.segment_lengths.len() * 4 + POINTER_FIXED_SIZE;
        }
    }
    let mut buf = Vec::with_capacity(est_size);
    buf.extend_from_slice(&FOOTER_MAGIC);
    buf.extend_from_slice(&FOOTER_VERSION.to_le_bytes());
    buf.extend_from_slice(&(entries.len() as u32).to_le_bytes());
    for entry in entries {
        buf.extend_from_slice(&entry.offset.to_le_bytes());
        buf.extend_from_slice(&entry.len.to_le_bytes());
        buf.extend_from_slice(&entry.num_rows.to_le_bytes());
        buf.extend_from_slice(&(entry.meta_bytes.len() as u32).to_le_bytes());
        buf.extend_from_slice(&entry.meta_bytes);

        buf.extend_from_slice(&(entry.column_pointers.len() as u16).to_le_bytes());
        for pointer in &entry.column_pointers {
            buf.extend_from_slice(&pointer.offset.to_le_bytes());
            buf.extend_from_slice(&pointer.length.to_le_bytes());
            buf.extend_from_slice(&(pointer.segment_lengths.len() as u16).to_le_bytes());
            for &slen in &pointer.segment_lengths {
                buf.extend_from_slice(&slen.to_le_bytes());
            }
        }
    }
    let checksum = CRC32.checksum(&buf);
    buf.extend_from_slice(&checksum.to_le_bytes());

    buf
}

pub fn decode_footer(bytes: &[u8]) -> Result<Vec<FooterEntry>, StorageError> {
    if bytes.len() < 12 {
        return Err(StorageError::CorruptFooter {
            segment_id: "?".into(),
            reason: "footer too short".into(),
        });
    }

    let payload_len = bytes.len() - 4;
    let expected_crc = u32::from_le_bytes(bytes[payload_len..].try_into().unwrap());
    let actual_crc = CRC32.checksum(&bytes[..payload_len]);
    if expected_crc != actual_crc {
        return Err(StorageError::CorruptFooter {
            segment_id: "?".into(),
            reason: format!("CRC32 mismatch: expected {expected_crc:#x}, found {actual_crc:#x}"),
        });
    }

    if bytes[0..2] != FOOTER_MAGIC {
        return Err(StorageError::CorruptFooter {
            segment_id: "?".into(),
            reason: "invalid footer magic bytes".into(),
        });
    }
    let version = u16::from_le_bytes(bytes[2..4].try_into().unwrap());
    if version != FOOTER_VERSION {
        return Err(StorageError::CorruptFooter {
            segment_id: "?".into(),
            reason: format!("unsupported footer version: {version}"),
        });
    }

    let mut cursor = 4;
    let entry_count = u32::from_le_bytes(bytes[cursor..cursor + 4].try_into().unwrap()) as usize;
    cursor += 4;

    let mut entries = Vec::with_capacity(entry_count);
    for _ in 0..entry_count {
        if cursor + ENTRY_FIXED_SIZE > payload_len {
            return Err(StorageError::CorruptFooter {
                segment_id: "?".into(),
                reason: "unexpected EOF reading entry header".into(),
            });
        }

        let offset = u64::from_le_bytes(bytes[cursor..cursor + 8].try_into().unwrap());
        cursor += 8;
        let len = u64::from_le_bytes(bytes[cursor..cursor + 8].try_into().unwrap());
        cursor += 8;
        let num_rows = u32::from_le_bytes(bytes[cursor..cursor + 4].try_into().unwrap());
        cursor += 4;
        let meta_len = u32::from_le_bytes(bytes[cursor..cursor + 4].try_into().unwrap()) as usize;
        cursor += 4;

        if cursor + meta_len > payload_len {
            return Err(StorageError::CorruptFooter {
                segment_id: "?".into(),
                reason: "unexpected EOF reading meta_only_bytes".into(),
            });
        }
        let meta_bytes = bytes[cursor..cursor + meta_len].to_vec();
        cursor += meta_len;

        if cursor + 2 > payload_len {
            return Err(StorageError::CorruptFooter {
                segment_id: "?".into(),
                reason: "unexpected EOF reading column count".into(),
            });
        }
        let num_cols = u16::from_le_bytes(bytes[cursor..cursor + 2].try_into().unwrap()) as usize;
        cursor += 2;

        let mut column_pointers = Vec::with_capacity(num_cols);
        for _ in 0..num_cols {
            if cursor + POINTER_FIXED_SIZE > payload_len {
                return Err(StorageError::CorruptFooter {
                    segment_id: "?".into(),
                    reason: "unexpected EOF reading column range header".into(),
                });
            }
            let offset = u64::from_le_bytes(bytes[cursor..cursor + 8].try_into().unwrap());
            cursor += 8;
            let length = u32::from_le_bytes(bytes[cursor..cursor + 4].try_into().unwrap());
            cursor += 4;
            let num_segs =
                u16::from_le_bytes(bytes[cursor..cursor + 2].try_into().unwrap()) as usize;
            cursor += 2;

            if cursor + num_segs * 4 > payload_len {
                return Err(StorageError::CorruptFooter {
                    segment_id: "?".into(),
                    reason: "unexpected EOF reading segment lengths".into(),
                });
            }
            let mut segment_lengths = Vec::with_capacity(num_segs);
            for _ in 0..num_segs {
                let slen = u32::from_le_bytes(bytes[cursor..cursor + 4].try_into().unwrap());
                cursor += 4;
                segment_lengths.push(slen);
            }

            column_pointers.push(ColumnPointer {
                offset,
                length,
                segment_lengths,
            });
        }

        entries.push(FooterEntry {
            offset,
            len,
            num_rows,
            meta_bytes,
            column_pointers,
        });
    }

    Ok(entries)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_footer_encode_decode_roundtrip() {
        let entries = vec![
            FooterEntry {
                offset: 0,
                len: 1024,
                num_rows: 50,
                meta_bytes: b"meta_bytes_chunk_0".to_vec(),
                column_pointers: vec![
                    ColumnPointer {
                        offset: 128,
                        length: 256,
                        segment_lengths: vec![32, 224],
                    },
                    ColumnPointer {
                        offset: 384,
                        length: 512,
                        segment_lengths: vec![32, 64, 416],
                    },
                ],
            },
            FooterEntry {
                offset: 1024,
                len: 2048,
                num_rows: 100,
                meta_bytes: b"meta_bytes_chunk_1".to_vec(),
                column_pointers: vec![ColumnPointer {
                    offset: 1152,
                    length: 800,
                    segment_lengths: vec![100, 700],
                }],
            },
        ];

        let encoded = encode_footer(&entries);
        assert!(encoded.len() > 12);

        let decoded = decode_footer(&encoded).unwrap();
        assert_eq!(decoded, entries);
    }

    #[test]
    fn test_footer_crc_corruption_detection() {
        let entries = vec![FooterEntry {
            offset: 0,
            len: 500,
            num_rows: 10,
            meta_bytes: vec![1, 2, 3, 4],
            column_pointers: vec![],
        }];

        let mut encoded = encode_footer(&entries);
        // Corrupt a byte in the payload
        encoded[5] ^= 0xFF;

        let err = decode_footer(&encoded).unwrap_err();
        match err {
            StorageError::CorruptFooter { reason, .. } => {
                assert!(reason.contains("CRC32 mismatch"));
            }
            _ => panic!("Expected CorruptFooter error"),
        }
    }

    #[test]
    fn test_footer_magic_and_version_validation() {
        let entries = vec![FooterEntry {
            offset: 0,
            len: 500,
            num_rows: 10,
            meta_bytes: vec![1, 2, 3, 4],
            column_pointers: vec![],
        }];

        // Bad magic bytes
        let mut encoded = encode_footer(&entries);
        encoded[0] = 0x00;
        // Recompute valid CRC so it gets past CRC check
        let payload_len = encoded.len() - 4;
        let new_crc = CRC32.checksum(&encoded[..payload_len]);
        encoded[payload_len..].copy_from_slice(&new_crc.to_le_bytes());

        let err = decode_footer(&encoded).unwrap_err();
        match err {
            StorageError::CorruptFooter { reason, .. } => {
                assert!(reason.contains("invalid footer magic bytes"));
            }
            _ => panic!("Expected CorruptFooter error"),
        }
    }

    #[test]
    fn test_footer_too_short() {
        let short = vec![0u8; 8];
        let err = decode_footer(&short).unwrap_err();
        match err {
            StorageError::CorruptFooter { reason, .. } => {
                assert!(reason.contains("footer too short"));
            }
            _ => panic!("Expected CorruptFooter error"),
        }
    }
}
