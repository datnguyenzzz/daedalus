use std::{cmp, sync::Arc};

use anyhow::{Result, anyhow, bail};

use crate::{
    Array, ArrayRef, DataType, RecordBatch, SchemaRef,
    array::{BooleanArray, NativeType, PrimitiveArray, StringArray},
    dispatch_native_type,
};

/// If the filter selects more than this fraction of rows, use
/// [`SlicesIterator`] to copy ranges of values. Otherwise iterate
/// over individual rows using [`IndexIterator`]
///
/// Threshold of 0.8 chosen based on <https://dl.acm.org/doi/abs/10.1145/3465998.3466009>
///
const FILTER_SLICES_SELECTIVITY_THRESHOLD: f64 = 0.8;

/// Take elements by index from [`Array`], creating a new [`Array`] from those indexes.
///
/// ```text
/// ┌─────────────────┐      ┌─────────┐                              ┌─────────────────┐
/// │        A        │      │    0    │                              │        A        │
/// ├─────────────────┤      ├─────────┤                              ├─────────────────┤
/// │        D        │      │    2    │                              │        B        │
/// ├─────────────────┤      ├─────────┤   take(values, indices)      ├─────────────────┤
/// │        B        │      │    3    │ ─────────────────────────▶   │        C        │
/// ├─────────────────┤      ├─────────┤                              ├─────────────────┤
/// │        C        │      │    1    │                              │        D        │
/// ├─────────────────┤      └─────────┘                              └─────────────────┘
/// │        E        │
/// └─────────────────┘
///    values array          indices array                              result
/// ```
pub fn take(values: &dyn Array, indexes: &dyn Array) -> Result<ArrayRef> {
    if indexes.data_type() != &DataType::Int32 {
        bail!(
            "Take indices must be of DataType::Int32, but found {:?}",
            indexes.data_type()
        )
    }

    let indexes_arr = indexes
        .as_any()
        .downcast_ref::<PrimitiveArray<i32>>()
        .ok_or_else(|| anyhow!("Failed to downcast indices to PrimitiveArray<i32>"))?;

    match values.data_type() {
        DataType::Boolean => {
            let typed_values = values
                .as_any()
                .downcast_ref::<BooleanArray>()
                .ok_or_else(|| anyhow!("Expected StringArray"))?;
            Ok(Arc::new(typed_values.take(indexes_arr)?))
        }
        DataType::Utf8 => {
            let typed_values = values
                .as_any()
                .downcast_ref::<StringArray>()
                .ok_or_else(|| anyhow!("Expected BooleanArray"))?;
            Ok(Arc::new(typed_values.take(indexes_arr)?))
        }

        primitive => {
            dispatch_native_type!(primitive, T, {
                let typed_values = values
                    .as_any()
                    .downcast_ref::<PrimitiveArray<T>>()
                    .ok_or_else(|| anyhow!("Expected PrimitiveArray"))?;
                Ok(Arc::new(typed_values.take(indexes_arr)?))
            })
        }
    }
}

/// Precomputed optimized strategy for selecting rows from arrays
pub enum IterationStrategy {
    /// A precomputed list of indices
    Indexes(PrimitiveArray<i32>),
    /// A precomputed array of ranges
    Slices(Vec<(usize, usize)>),
    /// Select all rows
    All,
    /// Select no rows
    None,
}

/// A filtering predicate that can be applied to any Array or RecordBatch
pub struct FilterPredicate {
    count: usize,
    /// Precomputed strategy for iterating over the selected rows of this predicate
    strategy: IterationStrategy,
}

impl FilterPredicate {
    pub fn new(predicate: &BooleanArray) -> Self {
        let count = predicate.true_count();
        let len = predicate.len();

        if len == 0 || count == 0 {
            return Self {
                count,
                strategy: IterationStrategy::None,
            };
        }

        if len == count {
            return Self {
                count,
                strategy: IterationStrategy::All,
            };
        }

        let mut slices = Vec::with_capacity(cmp::min(64, count));
        let mut indexes = Vec::with_capacity(count);
        let mut in_slice = false;
        let mut slice_start = 0;
        for i in 0..len {
            if predicate.value(i) {
                indexes.push(i as i32);
                if !in_slice {
                    in_slice = true;
                    slice_start = i
                }
            } else if in_slice {
                in_slice = false;
                slices.push((slice_start, i - slice_start))
            }
        }

        if in_slice {
            slices.push((slice_start, len - slice_start))
        }

        // This can then be used as a heuristic for the optimal iteration strategy
        let selectivity_frac = count as f64 / len as f64;
        if selectivity_frac > FILTER_SLICES_SELECTIVITY_THRESHOLD {
            return Self {
                count,
                strategy: IterationStrategy::Slices(slices),
            };
        }

        Self {
            count,
            strategy: IterationStrategy::Indexes(PrimitiveArray::from(indexes)),
        }
    }

    pub fn filter(&self, value: &dyn Array) -> Result<ArrayRef> {
        match &self.strategy {
            IterationStrategy::All => Ok(value.slice(0, self.count)),
            IterationStrategy::None => Ok(value.slice(0, 0)),
            IterationStrategy::Indexes(indexes) => take(value, indexes),
            IterationStrategy::Slices(slices) => {
                if slices.len() == 1 {
                    let (start, len) = slices[0];
                    Ok(value.slice(start, len))
                } else {
                    let indexes: Vec<i32> = slices
                        .iter()
                        .flat_map(|&(start, len)| (start..(start + len)).map(|i| i as i32))
                        .collect();

                    take(value, &PrimitiveArray::from(indexes))
                }
            }
        }
    }

    pub fn filter_record_batch(&self, batch: &RecordBatch) -> Result<RecordBatch> {
        let filtered_columns = batch
            .columns()
            .iter()
            .map(|c| self.filter(c.as_ref()))
            .collect::<Result<Vec<_>>>()?;

        RecordBatch::try_new(batch.schema().clone(), filtered_columns)
    }
}

/// Returns a filtered `values` [`Array`] where the corresponding elements of
/// `predicate` are `true`.
pub fn filter(value: &dyn Array, predicate: &BooleanArray) -> Result<ArrayRef> {
    let pred = FilterPredicate::new(predicate);
    pred.filter(value)
}

pub fn filter_record_batch(batch: &RecordBatch, predicate: &BooleanArray) -> Result<RecordBatch> {
    let pred = FilterPredicate::new(predicate);
    pred.filter_record_batch(batch)
}

/// Concatenates multiple [Array] slice references of the same type
/// into a single contiguous [ArrayRef]
pub fn concat(arrays: &[&dyn Array]) -> Result<ArrayRef> {
    if arrays.is_empty() {
        return Err(anyhow!("Cannot concatenate an empty list of arrays"));
    }

    if arrays.len() == 1 {
        return Ok(arrays[0].slice(0, arrays[0].len()));
    }

    let dt = arrays[0].data_type();
    let mut total_len = 0;
    for &arr in arrays {
        if arr.data_type() != dt {
            return Err(anyhow!(
                "Mismatched DataTypes in concat: expected {:?}, found {:?}",
                dt,
                arr.data_type()
            ));
        }
        total_len += arr.len();
    }

    match dt {
        DataType::Boolean => concat_boolean(arrays, total_len),
        DataType::Utf8 => concat_string(arrays, total_len),
        DataType::Int8 => concat_primitive::<i8>(arrays, total_len),
        DataType::Int16 => concat_primitive::<i16>(arrays, total_len),
        DataType::Int32 => concat_primitive::<i32>(arrays, total_len),
        DataType::Int64 => concat_primitive::<i64>(arrays, total_len),
        DataType::Float32 => concat_primitive::<f32>(arrays, total_len),
        DataType::Float64 => concat_primitive::<f64>(arrays, total_len),
    }
}

fn concat_string(arrays: &[&dyn Array], total_len: usize) -> Result<ArrayRef> {
    let mut values = Vec::with_capacity(total_len);
    for &arr in arrays {
        let typed_arr = arr.as_any().downcast_ref::<StringArray>().unwrap();
        for i in 0..typed_arr.len() {
            if !typed_arr.is_null(i) {
                values.push(Some(typed_arr.value(i)));
            } else {
                values.push(None);
            }
        }
    }

    let result = StringArray::from(values);
    Ok(Arc::new(result) as ArrayRef)
}

fn concat_boolean(arrays: &[&dyn Array], total_len: usize) -> Result<ArrayRef> {
    let mut values = Vec::with_capacity(total_len);
    for &arr in arrays {
        let typed_arr = arr.as_any().downcast_ref::<BooleanArray>().unwrap();
        for i in 0..typed_arr.len() {
            if !typed_arr.is_null(i) {
                values.push(Some(typed_arr.value(i)));
            } else {
                values.push(None);
            }
        }
    }

    let result = BooleanArray::from(values);
    Ok(Arc::new(result) as ArrayRef)
}

fn concat_primitive<T>(arrays: &[&dyn Array], total_len: usize) -> Result<ArrayRef>
where
    T: NativeType + 'static,
    PrimitiveArray<T>: Array,
{
    let mut values = Vec::with_capacity(total_len);
    for &arr in arrays {
        let typed_arr = arr.as_any().downcast_ref::<PrimitiveArray<T>>().unwrap();
        for i in 0..typed_arr.len() {
            if !typed_arr.is_null(i) {
                values.push(Some(typed_arr.value(i)));
            } else {
                values.push(None);
            }
        }
    }

    let result = PrimitiveArray::<T>::from(values);
    Ok(Arc::new(result) as ArrayRef)
}

pub fn concat_batches<'a>(
    schema: &SchemaRef,
    input_batches: impl IntoIterator<Item = &'a RecordBatch>,
) -> Result<RecordBatch> {
    let batches: Vec<&RecordBatch> = input_batches.into_iter().collect();
    if batches.is_empty() {
        return Ok(RecordBatch::new_empty(schema.clone()));
    }

    let field_num = schema.fields().len();
    let mut arrays = Vec::with_capacity(field_num);
    for i in 0..field_num {
        arrays.push(concat(
            &batches
                .iter()
                .map(|batch| batch.column(i).as_ref())
                .collect::<Vec<_>>(),
        )?);
    }

    RecordBatch::try_new(schema.clone(), arrays)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{DataType, Field, Schema, array::StringArray};

    #[test]
    fn test_filter_single_array() {
        let values = PrimitiveArray::from(vec![10i32, 20, 30, 40, 50]);
        let predicate =
            BooleanArray::from(vec![Some(true), None, Some(false), Some(true), Some(false)]);

        // Filter the values
        // Expected results: [10, 40] (since index 0 is true, 1 is null/false, 2 is false, 3 is true, 4 is false)
        let filtered = filter(&values, &predicate).unwrap();
        assert_eq!(filtered.len(), 2);

        let parsed = filtered
            .as_any()
            .downcast_ref::<PrimitiveArray<i32>>()
            .unwrap();
        assert_eq!(parsed.value(0), 10);
        assert_eq!(parsed.value(1), 40);
    }

    #[test]
    fn test_filter_record_batch() {
        let schema = Arc::new(Schema::new(vec![
            Field {
                name: "id".to_string(),
                data_type: DataType::Int32,
                nullable: false,
            },
            Field {
                name: "name".to_string(),
                data_type: DataType::Utf8,
                nullable: true,
            },
        ]));

        let id_col: ArrayRef = Arc::new(PrimitiveArray::from(vec![1i32, 2, 3, 4]));
        let name_col: ArrayRef = Arc::new(StringArray::from(vec![
            Some("Alice"),
            None,
            Some("Charlie"),
            Some("David"),
        ]));
        let batch = RecordBatch::try_new(schema, vec![id_col, name_col]).unwrap();

        let predicate = BooleanArray::from(vec![Some(true), Some(false), None, Some(true)]);

        // Filter the RecordBatch
        // Expected rows:
        // - Row 0 (id: 1, name: "Alice")
        // - Row 3 (id: 4, name: "David")
        let filtered_batch = filter_record_batch(&batch, &predicate).unwrap();
        assert_eq!(filtered_batch.num_rows(), 2);
        assert_eq!(filtered_batch.num_columns(), 2);

        // Verify column values are compacted correctly
        let out_id = filtered_batch
            .column(0)
            .as_any()
            .downcast_ref::<PrimitiveArray<i32>>()
            .unwrap();
        assert_eq!(out_id.value(0), 1);
        assert_eq!(out_id.value(1), 4);

        let out_name = filtered_batch
            .column(1)
            .as_any()
            .downcast_ref::<StringArray>()
            .unwrap();
        assert_eq!(out_name.value(0), "Alice");
        assert_eq!(out_name.value(1), "David");
    }
}
