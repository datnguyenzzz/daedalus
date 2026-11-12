use serde::{Deserialize, Serialize};

use crate::{
    Array, DataType,
    aggregation::{self, max_str, min_str},
    array::{BooleanArray, PrimitiveArray, StringArray},
    ord::Op,
};

pub const MAX_STAT_STRING_BYTES: usize = 32;

#[derive(Clone, PartialEq, Debug, Serialize, Deserialize)]
pub enum ColumnStat {
    Int64 { min: i64, max: i64 },
    Float64 { min: f64, max: f64 },
    Utf8 { min: String, max: String },
    Bool { has_true: bool, has_false: bool },
}

impl ColumnStat {
    pub fn encode(&self) -> Vec<u8> {
        match self {
            ColumnStat::Int64 { min, max } => {
                let mut bytes = Vec::with_capacity(17);
                bytes.push(0);
                bytes.extend_from_slice(&min.to_le_bytes());
                bytes.extend_from_slice(&max.to_le_bytes());
                bytes
            }
            ColumnStat::Float64 { min, max } => {
                let mut bytes = Vec::with_capacity(17);
                bytes.push(1);
                bytes.extend_from_slice(&min.to_bits().to_le_bytes());
                bytes.extend_from_slice(&max.to_bits().to_le_bytes());
                bytes
            }
            ColumnStat::Utf8 { min, max } => {
                let mut bytes = Vec::with_capacity(69);
                bytes.push(2);
                let min_b = min.as_bytes();
                let min_len = min_b.len().min(MAX_STAT_STRING_BYTES);
                bytes.extend_from_slice(&(min_len as u16).to_le_bytes());
                bytes.extend_from_slice(&min_b[..min_len]);

                // truncate and increament for max value, to avoid incorrectly discard chunks
                let max_b = max.as_bytes();
                let ptr = max_b.as_ptr() as *mut u8;
                let max_len = max_b.len().min(MAX_STAT_STRING_BYTES);
                bytes.extend_from_slice(&(max_len as u16).to_le_bytes());
                for i in (0..max_len).rev() {
                    if max_b[i] != 0xff {
                        unsafe { *ptr.add(i) = max_b[i] + 1 }
                        break;
                    }
                    unsafe { *ptr.add(i) = 0 }
                }
                bytes.extend_from_slice(&max_b[..max_len]);
                bytes
            }
            ColumnStat::Bool {
                has_true,
                has_false,
            } => {
                let mut bytes = Vec::with_capacity(3);
                bytes.push(3);
                bytes.push(if *has_false { 1 } else { 0 });
                bytes.push(if *has_true { 1 } else { 0 });
                bytes
            }
        }
    }

    pub fn decode(slice: &[u8]) -> Option<Self> {
        if slice.is_empty() {
            return None;
        }

        match slice[0] {
            0 => {
                // Int64
                if slice.len() != 17 {
                    return None;
                }
                let min = i64::from_le_bytes(slice[1..9].try_into().ok()?);
                let max = i64::from_le_bytes(slice[9..17].try_into().ok()?);
                Some(ColumnStat::Int64 { min, max })
            }
            1 => {
                // Float64
                if slice.len() != 17 {
                    return None;
                }
                let min = f64::from_bits(u64::from_le_bytes(slice[1..9].try_into().ok()?));
                let max = f64::from_bits(u64::from_le_bytes(slice[9..17].try_into().ok()?));
                Some(ColumnStat::Float64 { min, max })
            }
            2 => {
                // utf8
                if slice.len() < 3 {
                    return None;
                }
                let min_len = u16::from_le_bytes(slice[1..3].try_into().ok()?) as usize;
                let min_end = 3 + min_len;
                if slice.len() < min_end + 2 {
                    return None;
                }
                let min = str::from_utf8(&slice[3..min_end]).ok()?.to_string();

                let max_len =
                    u16::from_le_bytes(slice[min_end..min_end + 2].try_into().ok()?) as usize;
                let max_end = min_end + 2 + max_len;
                if slice.len() < max_end {
                    return None;
                }
                let max = str::from_utf8(&slice[min_end + 2..max_end])
                    .ok()?
                    .to_string();

                Some(ColumnStat::Utf8 { min, max })
            }
            3 => {
                // bool
                if slice.len() < 3 {
                    return None;
                }
                let has_false = slice[1] != 0;
                let has_true = slice[2] != 0;
                Some(ColumnStat::Bool {
                    has_true,
                    has_false,
                })
            }
            _ => None,
        }
    }

    pub fn is_disjoint(&self, other: &ColumnStat) -> bool {
        match (self, other) {
            (
                ColumnStat::Int64 {
                    min: min_a,
                    max: max_a,
                },
                ColumnStat::Int64 {
                    min: min_b,
                    max: max_b,
                },
            ) => max_a < min_b || min_a > max_b,
            (
                ColumnStat::Float64 {
                    min: min_a,
                    max: max_a,
                },
                ColumnStat::Float64 {
                    min: min_b,
                    max: max_b,
                },
            ) => max_a < min_b || min_a > max_b,
            (
                ColumnStat::Utf8 {
                    min: min_a,
                    max: max_a,
                },
                ColumnStat::Utf8 {
                    min: min_b,
                    max: max_b,
                },
            ) => max_a < min_b || min_a > max_b,
            (
                ColumnStat::Bool {
                    has_true: t_a,
                    has_false: f_a,
                },
                ColumnStat::Bool {
                    has_true: t_b,
                    has_false: f_b,
                },
            ) => {
                // Disjoint if one set is exclusively true and the other is exclusively false
                (*t_a && !*f_a && !*t_b && *f_b) || (!*t_a && *f_a && *t_b && !*f_b)
            }
            _ => false,
        }
    }
}

pub fn compute_column_stat(array: &dyn Array) -> Option<ColumnStat> {
    if array.len() == 0
        || array
            .nulls()
            .map(|n| n.null_count() == array.len())
            .unwrap_or(false)
    {
        return None;
    }

    match array.data_type() {
        DataType::Int8 => {
            let arr = array.as_any().downcast_ref::<PrimitiveArray<i8>>()?;
            Some(ColumnStat::Int64 {
                min: aggregation::min(arr)? as i64,
                max: aggregation::max(arr)? as i64,
            })
        }
        DataType::Int16 => {
            let arr = array.as_any().downcast_ref::<PrimitiveArray<i16>>()?;
            Some(ColumnStat::Int64 {
                min: aggregation::min(arr)? as i64,
                max: aggregation::max(arr)? as i64,
            })
        }
        DataType::Int32 => {
            let arr = array.as_any().downcast_ref::<PrimitiveArray<i32>>()?;
            Some(ColumnStat::Int64 {
                min: aggregation::min(arr)? as i64,
                max: aggregation::max(arr)? as i64,
            })
        }
        DataType::Int64 => {
            let arr = array.as_any().downcast_ref::<PrimitiveArray<i64>>()?;
            Some(ColumnStat::Int64 {
                min: aggregation::min(arr)?,
                max: aggregation::max(arr)?,
            })
        }
        DataType::Float64 => {
            let arr = array.as_any().downcast_ref::<PrimitiveArray<f64>>()?;
            Some(ColumnStat::Float64 {
                min: aggregation::min_float(arr)?,
                max: aggregation::max_float(arr)?,
            })
        }
        DataType::Utf8 => {
            let arr = array.as_any().downcast_ref::<StringArray>()?;
            Some(ColumnStat::Utf8 {
                min: min_str(arr)?.to_string(),
                max: max_str(arr)?.to_string(),
            })
        }
        DataType::Boolean => {
            let arr = array.as_any().downcast_ref::<BooleanArray>()?;
            let has_false = aggregation::min_boolean(arr).map(|v| !v).unwrap_or(false);
            let has_true = aggregation::max_boolean(arr).map(|v| v).unwrap_or(false);
            Some(ColumnStat::Bool {
                has_true,
                has_false,
            })
        }
        _ => None,
    }
}

macro_rules! could_match_int_all {
    ($ty:ty, $array:expr, $min:expr, $max:expr, $op:expr) => {{
        let arr = $array
            .as_any()
            .downcast_ref::<PrimitiveArray<$ty>>()
            .unwrap();
        arr.iter().all(|opt| match opt {
            Some(v) => {
                let v = v as i64;
                match $op {
                    Op::Equal => v >= *$min && v <= *$max,
                    Op::Less => *$min < v,
                    Op::LessEqual => *$min <= v,
                    Op::Greater => *$max > v,
                    Op::GreaterEqual => *$max >= v,
                    Op::NotEqual => true,
                }
            }
            None => false,
        })
    }};
}

macro_rules! could_match_float_all {
    ($ty:ty, $array:expr, $min:expr, $max:expr, $op:expr) => {{
        let arr = $array
            .as_any()
            .downcast_ref::<PrimitiveArray<$ty>>()
            .unwrap();
        arr.iter().all(|opt| match opt {
            Some(v) => {
                let v = v as f64;
                match $op {
                    Op::Equal => v >= *$min && v <= *$max,
                    Op::Less => *$min < v,
                    Op::LessEqual => *$min <= v,
                    Op::Greater => *$max > v,
                    Op::GreaterEqual => *$max >= v,
                    Op::NotEqual => true,
                }
            }
            None => false,
        })
    }};
}

/// true = chunk might satisfy `col {op} value` for ALL elements in the array.
/// false = at least one element CANNOT be satisfied, safe to prune/skip.
pub fn could_match(stat: &ColumnStat, op: &Op, array: &dyn Array) -> bool {
    if array.len() == 0 {
        return true;
    }

    match stat {
        ColumnStat::Int64 { min, max } => {
            let matches_all = match array.data_type() {
                DataType::Int64 => could_match_int_all!(i64, array, min, max, op),
                DataType::Int32 => could_match_int_all!(i32, array, min, max, op),
                DataType::Int16 => could_match_int_all!(i16, array, min, max, op),
                DataType::Int8 => could_match_int_all!(i8, array, min, max, op),
                _ => true, // Incompatible type: cannot prove disjoint, must read
            };
            matches_all
        }

        ColumnStat::Float64 { min, max } => {
            let matches_all = match array.data_type() {
                DataType::Float64 => could_match_float_all!(f64, array, min, max, op),
                DataType::Float32 => could_match_float_all!(f32, array, min, max, op),
                _ => true,
            };
            matches_all
        }

        ColumnStat::Utf8 { min, max } => {
            if let Some(arr) = array.as_any().downcast_ref::<StringArray>() {
                let matches_all = arr.iter().all(|opt| match opt {
                    Some(v) => match op {
                        Op::Equal => v >= min.as_str() && v <= max.as_str(),
                        Op::Less => min.as_str() < v,
                        Op::LessEqual => min.as_str() <= v,
                        Op::Greater => max.as_str() > v,
                        Op::GreaterEqual => max.as_str() >= v,
                        Op::NotEqual => true,
                    },
                    None => false,
                });
                matches_all
            } else {
                true
            }
        }

        ColumnStat::Bool {
            has_true,
            has_false,
        } => {
            if let Some(arr) = array.as_any().downcast_ref::<BooleanArray>() {
                let matches_all = arr.iter().all(|opt| match opt {
                    Some(v) => match op {
                        Op::Equal => {
                            if v {
                                *has_true
                            } else {
                                *has_false
                            }
                        }
                        Op::NotEqual => {
                            if v {
                                *has_false
                            } else {
                                *has_true
                            }
                        }
                        _ => true,
                    },
                    None => false,
                });
                matches_all
            } else {
                true
            }
        }
    }
}

pub fn merge(a: &ColumnStat, b: &ColumnStat) -> ColumnStat {
    match (a, b) {
        (ColumnStat::Int64 { min: a0, max: a1 }, ColumnStat::Int64 { min: b0, max: b1 }) => {
            ColumnStat::Int64 {
                min: *a0.min(b0),
                max: *a1.max(b1),
            }
        }
        (ColumnStat::Float64 { min: a0, max: a1 }, ColumnStat::Float64 { min: b0, max: b1 }) => {
            ColumnStat::Float64 {
                min: a0.min(*b0),
                max: a1.max(*b1),
            }
        }
        (ColumnStat::Utf8 { min: a0, max: a1 }, ColumnStat::Utf8 { min: b0, max: b1 }) => {
            ColumnStat::Utf8 {
                min: a0.min(b0).clone(),
                max: a1.max(b1).clone(),
            }
        }
        (
            ColumnStat::Bool {
                has_true: a0,
                has_false: a1,
            },
            ColumnStat::Bool {
                has_true: b0,
                has_false: b1,
            },
        ) => ColumnStat::Bool {
            has_true: a0 | b0,
            has_false: a1 | b1,
        },
        _ => a.clone(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;
    use crate::ArrayRef;

    #[test]
    fn test_column_stat_int64_roundtrip_and_disjoint() {
        let stat = ColumnStat::Int64 {
            min: -100,
            max: 200,
        };
        let encoded = stat.encode();
        let decoded = ColumnStat::decode(&encoded).expect("Failed to decode Int64 stat");
        assert_eq!(stat, decoded);

        let non_overlapping = ColumnStat::Int64 { min: 201, max: 500 };
        assert!(stat.is_disjoint(&non_overlapping));

        let overlapping = ColumnStat::Int64 { min: 150, max: 300 };
        assert!(!stat.is_disjoint(&overlapping));
    }

    #[test]
    fn test_column_stat_float64_roundtrip_and_disjoint() {
        let stat = ColumnStat::Float64 {
            min: -15.5,
            max: 99.75,
        };
        let encoded = stat.encode();
        let decoded = ColumnStat::decode(&encoded).expect("Failed to decode Float64 stat");
        assert_eq!(stat, decoded);

        let disjoint = ColumnStat::Float64 {
            min: 100.0,
            max: 200.0,
        };
        assert!(stat.is_disjoint(&disjoint));
    }

    #[test]
    fn test_column_stat_utf8_capping_and_disjoint() {
        let long_str = "a".repeat(100);
        let stat = ColumnStat::Utf8 {
            min: long_str.clone(),
            max: "zzzz".to_string(),
        };
        let encoded = stat.encode();
        let decoded = ColumnStat::decode(&encoded).expect("Failed to decode Utf8 stat");
        match decoded {
            ColumnStat::Utf8 { min, max } => {
                assert_eq!(min.len(), MAX_STAT_STRING_BYTES);
                // max was incremented on the last byte from 'z' (0x7A) to '{' (0x7B)
                assert_eq!(max, "zzz{");
            }
            _ => panic!("Expected Utf8 stat"),
        }
    }

    #[test]
    fn test_column_stat_utf8_max_truncation_and_increment() {
        // A long string exceeding 32 bytes:
        // Truncated alone would be "hello_world_this_is_a_very_long"
        // With increment, it becomes "hello_world_this_is_a_very_lonh"
        // which guarantees that any probe matching "hello_world_this_is_a_very_long_xyz" is NOT discarded!
        let min_str = "a".to_string();
        let original_max = "hello_world_this_is_a_very_long_string_12345".to_string();

        let stat = ColumnStat::Utf8 {
            min: min_str,
            max: original_max.clone(),
        };
        let encoded = stat.encode();
        let decoded = ColumnStat::decode(&encoded).expect("Failed to decode Utf8 stat");

        match decoded {
            ColumnStat::Utf8 { min, max } => {
                assert_eq!(min, "a");
                assert_eq!(max.len(), MAX_STAT_STRING_BYTES);
                let raw_prefix = &original_max[..MAX_STAT_STRING_BYTES];
                assert!(
                    max.as_str() > raw_prefix,
                    "Truncated max must be incremented to avoid false-positive pruning"
                );

                // Verify that the probe with the original long max string is correctly NOT disjoint!
                let probe = ColumnStat::Utf8 {
                    min: original_max.clone(),
                    max: original_max.clone(),
                };
                assert!(
                    !stat.is_disjoint(&probe),
                    "Original item must not be pruned by truncated/incremented bound"
                );
            }
            _ => panic!("Expected Utf8 stat"),
        }
    }

    #[test]
    fn test_column_stat_bool_disjoint() {
        let true_only = ColumnStat::Bool {
            has_true: true,
            has_false: false,
        };
        let false_only = ColumnStat::Bool {
            has_true: false,
            has_false: true,
        };
        let both = ColumnStat::Bool {
            has_true: true,
            has_false: true,
        };

        assert!(true_only.is_disjoint(&false_only));
        assert!(!true_only.is_disjoint(&both));
    }

    #[test]
    fn test_could_match_int64_all_semantics() {
        let stat = ColumnStat::Int64 { min: 10, max: 50 };

        // 1. Op::Equal: All elements within [10, 50]
        let all_match: ArrayRef = Arc::new(PrimitiveArray::from(vec![15i64, 25, 45]));
        assert!(could_match(&stat, &Op::Equal, all_match.as_ref()));

        // 2. Op::Equal: One element outside [10, 50] -> must return false!
        let one_fails: ArrayRef = Arc::new(PrimitiveArray::from(vec![15i64, 25, 100]));
        assert!(!could_match(&stat, &Op::Equal, one_fails.as_ref()));

        // 3. Widening from Int32
        let int32_arr: ArrayRef = Arc::new(PrimitiveArray::from(vec![20i32, 30]));
        assert!(could_match(&stat, &Op::Equal, int32_arr.as_ref()));

        // 4. Op::Less: col < 60 (min < 60 is true)
        let less_arr: ArrayRef = Arc::new(PrimitiveArray::from(vec![60i64]));
        assert!(could_match(&stat, &Op::Less, less_arr.as_ref()));

        // Op::Less: col < 5 (min < 5 is false)
        let less_fails: ArrayRef = Arc::new(PrimitiveArray::from(vec![5i64]));
        assert!(!could_match(&stat, &Op::Less, less_fails.as_ref()));

        // 5. Op::Greater: col > 5 (max > 5 is true)
        let greater_arr: ArrayRef = Arc::new(PrimitiveArray::from(vec![5i64]));
        assert!(could_match(&stat, &Op::Greater, greater_arr.as_ref()));

        // Op::Greater: col > 60 (max > 60 is false)
        let greater_fails: ArrayRef = Arc::new(PrimitiveArray::from(vec![60i64]));
        assert!(!could_match(&stat, &Op::Greater, greater_fails.as_ref()));
    }

    #[test]
    fn test_could_match_float64_all_semantics() {
        let stat = ColumnStat::Float64 { min: 0.0, max: 100.0 };

        let matches: ArrayRef = Arc::new(PrimitiveArray::from(vec![25.5f64, 75.0]));
        assert!(could_match(&stat, &Op::Equal, matches.as_ref()));

        let partial_fail: ArrayRef = Arc::new(PrimitiveArray::from(vec![25.5f64, 150.0]));
        assert!(!could_match(&stat, &Op::Equal, partial_fail.as_ref()));

        // Widening from Float32
        let f32_arr: ArrayRef = Arc::new(PrimitiveArray::from(vec![50.0f32]));
        assert!(could_match(&stat, &Op::Equal, f32_arr.as_ref()));
    }

    #[test]
    fn test_could_match_utf8_all_semantics() {
        let stat = ColumnStat::Utf8 {
            min: "banana".to_string(),
            max: "mango".to_string(),
        };

        // All between banana and mango
        let matches: ArrayRef = Arc::new(StringArray::from(vec!["cherry", "date"]));
        assert!(could_match(&stat, &Op::Equal, matches.as_ref()));

        // One before banana ("apple") -> false
        let fails: ArrayRef = Arc::new(StringArray::from(vec!["cherry", "apple"]));
        assert!(!could_match(&stat, &Op::Equal, fails.as_ref()));

        // Op::Less: col < "orange" -> min ("banana") < "orange" is true
        let less_ok: ArrayRef = Arc::new(StringArray::from(vec!["orange"]));
        assert!(could_match(&stat, &Op::Less, less_ok.as_ref()));

        // Op::Less: col < "apple" -> min ("banana") < "apple" is false
        let less_fail: ArrayRef = Arc::new(StringArray::from(vec!["apple"]));
        assert!(!could_match(&stat, &Op::Less, less_fail.as_ref()));
    }

    #[test]
    fn test_could_match_bool_and_empty() {
        let stat_true_only = ColumnStat::Bool {
            has_true: true,
            has_false: false,
        };

        let true_arr: ArrayRef = Arc::new(BooleanArray::from(vec![true]));
        assert!(could_match(&stat_true_only, &Op::Equal, true_arr.as_ref()));

        let false_arr: ArrayRef = Arc::new(BooleanArray::from(vec![false]));
        assert!(!could_match(&stat_true_only, &Op::Equal, false_arr.as_ref()));

        // Empty array returns true
        let empty_arr: ArrayRef = Arc::new(PrimitiveArray::<i64>::from(Vec::<i64>::new()));
        assert!(could_match(&stat_true_only, &Op::Equal, empty_arr.as_ref()));
    }

    #[test]
    fn test_merge_column_stat() {
        // Int64 merge
        let i1 = ColumnStat::Int64 { min: 10, max: 50 };
        let i2 = ColumnStat::Int64 { min: 5, max: 40 };
        assert_eq!(merge(&i1, &i2), ColumnStat::Int64 { min: 5, max: 50 });

        // Float64 merge
        let f1 = ColumnStat::Float64 { min: 1.5, max: 10.0 };
        let f2 = ColumnStat::Float64 { min: 0.5, max: 12.0 };
        assert_eq!(merge(&f1, &f2), ColumnStat::Float64 { min: 0.5, max: 12.0 });

        // Utf8 merge
        let u1 = ColumnStat::Utf8 { min: "b".into(), max: "m".into() };
        let u2 = ColumnStat::Utf8 { min: "a".into(), max: "k".into() };
        assert_eq!(merge(&u1, &u2), ColumnStat::Utf8 { min: "a".into(), max: "m".into() });

        // Bool merge
        let b1 = ColumnStat::Bool { has_true: true, has_false: false };
        let b2 = ColumnStat::Bool { has_true: false, has_false: true };
        assert_eq!(merge(&b1, &b2), ColumnStat::Bool { has_true: true, has_false: true });
    }
}
