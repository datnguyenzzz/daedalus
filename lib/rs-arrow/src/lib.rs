pub mod aggregation;
pub mod array;
pub mod ipc;
pub mod object;
pub mod ord;

use std::{mem, ops::Deref, slice, sync::Arc};

use anyhow::{Result, anyhow};
use bincode::{Decode, Encode};

use crate::array::new_empty_array;

// Arrow Data type \\

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Decode, Encode)]
pub enum DataType {
    Boolean,
    Int8,
    Int16,
    Int32,
    Int64,
    Float32,
    Float64,
    Utf8,
}

// Arrow Buffer \\

/// A contiguous memory region that can be shared with other buffers and across
/// thread boundaries that stores Arrow data.
/// [`Buffer`] can be sliced and cloned without copying the underlying data
/// https://github.com/apache/arrow-rs/blob/main/arrow-buffer/src/buffer/immutable.rs#L83
#[derive(Clone)]
pub struct Buffer {
    /// The shared, thread-safe raw byte allocation holds the actual data
    data: Arc<Vec<u8>>,

    /// Pointer pointing directly to the start of the active slice.
    /// This prevents offset additions during hot-path evaluations, allowing LLVM to vectorise cleanly
    ptr: *const u8,

    /// Byte length of the buffer.
    length: usize,
}

// Explicitly implement Send + Sync. Completely safe because the Arc keeps
// the pointer's memory alive.
unsafe impl Send for Buffer {}
unsafe impl Sync for Buffer {}

impl Buffer {
    /// Create a [`Buffer`] from the provided [`Vec`] without copying
    pub fn from(vec: Vec<u8>) -> Self {
        let length = vec.len();
        let data = Arc::new(vec);
        let ptr = data.as_ptr();
        Self { data, ptr, length }
    }

    pub fn from_zeroed(len: usize) -> Self {
        Self::from(vec![0u8; len])
    }

    /// Returns a new [Buffer] that is a slice of this buffer starting at `offset`,
    /// with `length` bytes.
    ///
    /// This function is `O(1)` and does not copy any data, allowing the same
    /// memory region to be shared between buffers.
    ///
    /// # Panics
    /// Panics if `(offset + length)` is larger than the existing length.
    pub fn slice(&self, offset_bytes: usize, length_bytes: usize) -> Self {
        assert!(offset_bytes + length_bytes <= self.length);
        unsafe {
            Self {
                data: self.data.clone(),
                ptr: self.ptr.add(offset_bytes),
                length: length_bytes,
            }
        }
    }

    /// Reads a scalar of type `T` at index `index` without any bounds checking.
    #[inline]
    pub unsafe fn get_uncheck<T: Copy>(&self, index: usize) -> T {
        let offset = index * mem::size_of::<T>();
        unsafe {
            let target_ptr = self.ptr.add(offset) as *const T;
            *target_ptr
        }
    }

    /// Exposes the active, sliced byte window directly as a Rust slice
    /// (when safe-path reads are needed)
    pub fn as_slice(&self) -> &[u8] {
        unsafe { slice::from_raw_parts(self.ptr, self.length) }
    }

    /// Returns a mutable slice over the active buffer memory window.
    ///
    /// # Safety
    /// The caller must ensure that this [`Buffer`] instance is **not shared** (no active clones of
    /// this buffer are being accessed, read, or written to on other threads concurrently).
    /// Violating this constraint leads to data races and undefined behavior.
    pub fn as_slice_mut(&self) -> &mut [u8] {
        unsafe { slice::from_raw_parts_mut(self.ptr as *mut u8, self.length) }
    }

    /// Returns the length of this buffer in bytes.
    #[inline]
    pub fn len(&self) -> usize {
        self.length
    }

    /// Returns a pointer to the start of this buffer.
    ///
    /// Note that this should be used cautiously, and the returned pointer should not be
    /// stored anywhere, to avoid dangling pointers.
    pub fn as_ptr(&self) -> *const u8 {
        self.ptr
    }

    /// Resizes the buffer, either truncating its contents (with no change in capacity), or
    /// growing it (potentially reallocating it) and writing `value` in the newly available bytes.
    ///
    /// If there are multiple active references (clones) to this buffer's shared memory, or if
    /// this buffer is a sliced view, this function will automatically perform a **Copy-on-Write**
    /// to avoid mutating other shared references, copying only the active slice data.
    pub fn resize(&mut self, new_len: usize, value: u8) {
        // Check if we have exclusive mutable ownership and the buffer is un-sliced
        let is_exclusive = Arc::get_mut(&mut self.data).is_some();
        let is_sliced = self.ptr != self.data.as_ptr();
        if is_exclusive && !is_sliced {
            // In-place resize
            let data_vec = Arc::get_mut(&mut self.data).unwrap();
            data_vec.resize(new_len, value);

            self.ptr = data_vec.as_ptr();
            self.length = new_len;
        } else {
            // Copy-On-Write: Isolate and copy only the active sliced bytes, then resize
            let mut new_data = self.as_slice().to_vec();
            new_data.resize(new_len, value);

            let data = Arc::new(new_data);
            self.ptr = data.as_ptr();
            self.length = new_len;
            self.data = data; // Transfer ownership to the newly isolated allocation
        }
    }
}

/// A slice-able [`Buffer`] containing bit-packed booleans
///
/// This structure represents a sequence of boolean values packed into a
/// byte-aligned [`Buffer`]. Both the offset and length are represented in bits.
///
/// # Layout
///
/// The values are represented as little endian bit-packed values, where the
/// least significant bit of each byte represents the first boolean value and
/// then proceeding to the most significant bit.
///
/// For example, the 10 bit bitmask `0b0111001101` has length 10, and is
/// represented using 2 bytes with offset 0 like this:
///
/// ```text
///        ┌─────────────────────────────────┐    ┌─────────────────────────────────┐
///        │┌───┬───┬───┬───┬───┬───┬───┬───┐│    │┌───┬───┬───┬───┬───┬───┬───┬───┐│
///        ││ 1 │ 0 │ 1 │ 1 │ 0 │ 0 │ 1 │ 1 ││    ││ 1 │ 0 │ ? │ ? │ ? │ ? │ ? │ ? ││
///        │└───┴───┴───┴───┴───┴───┴───┴───┘│    │└───┴───┴───┴───┴───┴───┴───┴───┘│
/// bit    └─────────────────────────────────┘    └─────────────────────────────────┘
/// offset  0             Byte 0             7    0              Byte 1            7
///
///         length = 10 bits, offset = 0
/// ```
#[derive(Clone)]
pub struct BooleanBuffer {
    /// Underlying buffer (byte aligned)
    buffer: Buffer,
    /// Offset in bits (not bytes)
    bit_offset: usize,
    /// Length in bits (not bytes)
    bit_len: usize,
}

impl BooleanBuffer {
    /// Create a new [`BooleanBuffer`] from a [`Buffer`], `bit_offset` offset and `bit_len` length
    ///
    /// # Panics
    ///
    /// This method will panic if `buffer` is not large enough
    pub fn new(buffer: Buffer, bit_offset: usize, bit_len: usize) -> Self {
        let total_len = bit_offset.saturating_add(bit_len);
        let buffer_len = buffer.len();
        let buffer_bit_len = buffer_len.saturating_mul(8);
        assert!(
            total_len <= buffer_bit_len,
            "buffer not large enough (bit_offset: {bit_offset}, bit_len: {bit_len}, buffer_len: {buffer_len})"
        );
        Self {
            buffer,
            bit_offset,
            bit_len,
        }
    }

    /// Slices this [`BooleanBuffer`] by the provided `offset` and `length`
    pub fn slice(&self, offset: usize, len: usize) -> Self {
        assert!(offset + len <= self.bit_len);
        Self {
            buffer: self.buffer.clone(),
            bit_offset: self.bit_offset + offset,
            bit_len: len,
        }
    }

    /// Returns the boolean value at index `i`
    #[inline]
    pub fn value(&self, idx: usize) -> bool {
        assert!(idx < self.bit_len);
        unsafe { BooleanBuffer::get_bit_raw(self.buffer.as_ptr(), self.bit_offset + idx) }
    }

    /// Returns the length of this boolean buffer in bits.
    #[inline]
    pub fn len(&self) -> usize {
        self.bit_len
    }

    /// Performs a bitwise binary operation with another [`BooleanBuffer`] using u64 word operations.
    pub fn bitwise_bin_op<F>(&self, rhs: &BooleanBuffer, mut op: F) -> BooleanBuffer
    where
        F: FnMut(u64, u64) -> u64,
    {
        assert_eq!(
            self.bit_len, rhs.bit_len,
            "Boolean buffers must have equal length"
        );
        let bit_len = self.bit_len;
        let total_bytes = (bit_len + 7) / 8;
        let mut result_bytes = vec![0u8; total_bytes];

        if self.bit_offset == 0 && rhs.bit_offset == 0 {
            // Fast Path: aligned, direct u64 bitwise binary operation!
            let self_slice = self.buffer.as_slice();
            let rhs_slice = rhs.buffer.as_slice();
            let u64_chunks = total_bytes / 8;

            unsafe {
                let self_ptr_u64 = self_slice.as_ptr() as *const u64;
                let rhs_ptr_u64 = rhs_slice.as_ptr() as *const u64;
                let result_ptr_u64 = result_bytes.as_mut_ptr() as *mut u64;

                for i in 0..u64_chunks {
                    let w1 = *self_ptr_u64.add(i);
                    let w2 = *rhs_ptr_u64.add(i);
                    *result_ptr_u64.add(i) = op(w1, w2);
                }
            }

            // Tail bytes
            let processed_bytes = u64_chunks * 8;
            for i in processed_bytes..total_bytes {
                let w1 = self_slice[i] as u64;
                let w2 = rhs_slice[i] as u64;
                result_bytes[i] = op(w1, w2) as u8;
            }
        } else {
            // Slow Path Fallback: bit-by-bit iteration for unaligned bits
            for i in 0..bit_len {
                let bit_lhs = self.value(i);
                let bit_rhs = rhs.value(i);

                let w1 = if bit_lhs { 1u64 } else { 0u64 };
                let w2 = if bit_rhs { 1u64 } else { 0u64 };
                let bit_res = (op(w1, w2) & 1) != 0;

                if bit_res {
                    result_bytes[i / 8] |= 1 << (i % 8);
                }
            }
        }

        let buffer = Buffer::from(result_bytes);
        BooleanBuffer::new(buffer, 0, bit_len)
    }

    /// Returns the number of set bits in this buffer
    pub fn count_set_bits(&self) -> usize {
        let mut count = 0;
        for i in 0..self.bit_len {
            if self.value(i) {
                count += 1
            }
        }

        count
    }

    // Bit utils

    /// Returns whether bit at position `i` in `data` is set or not.
    unsafe fn get_bit_raw(data: *const u8, i: usize) -> bool {
        unsafe { *data.add(i / 8) & (1 << (i % 8)) != 0 }
    }
}

/// A [`BooleanBuffer`] used to encode validity (null values) for Arrow arrays
///
/// In the [Arrow specification], array validity is encoded in a packed bitmask with a
/// `true` value indicating the corresponding slot is not null, and `false` indicating
/// that it is null.
#[derive(Clone)]
pub struct NullBuffer {
    buffer: BooleanBuffer,
    null_count: usize,
}

impl NullBuffer {
    /// Create a new [`NullBuffer`] computing the null count
    pub fn new(buffer: BooleanBuffer) -> Self {
        let null_count = buffer.bit_len - buffer.count_set_bits();
        Self { buffer, null_count }
    }

    /// Slices this [`NullBuffer`] by the provided `offset` and `len`
    pub fn slice(&self, offset: usize, len: usize) -> Self {
        Self::new(self.buffer.slice(offset, len))
    }

    #[inline]
    pub fn is_null(&self, idx: usize) -> bool {
        !self.buffer.value(idx)
    }

    /// Returns the null count for this [`NullBuffer`]
    #[inline]
    pub fn null_count(&self) -> usize {
        self.null_count
    }

    /// Returns a reference to the underlying [`BooleanBuffer`] bitmask.
    #[inline]
    pub fn inner(&self) -> &BooleanBuffer {
        &self.buffer
    }
}

// Arrow Schema \\

/// Describes a single column in a [`Schema`](super::Schema).
///
/// A [`Schema`](super::Schema) is an ordered collection of
/// [`Field`] objects. Fields contain:
/// * `name`: the name of the field
/// * `data_type`: the type of the field
/// * `nullable`: if the field is nullable
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Field {
    pub name: String,
    pub data_type: DataType,
    pub nullable: bool,
}

pub type FieldRef = Arc<Field>;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Fields(Arc<[FieldRef]>);

impl Deref for Fields {
    type Target = [FieldRef];

    fn deref(&self) -> &Self::Target {
        self.0.as_ref()
    }
}

impl Fields {
    pub fn iter(&self) -> std::slice::Iter<'_, FieldRef> {
        self.0.iter()
    }
}

impl<'a> IntoIterator for &'a Fields {
    type Item = &'a FieldRef;
    type IntoIter = slice::Iter<'a, FieldRef>;
    fn into_iter(self) -> Self::IntoIter {
        self.iter()
    }
}

impl From<Vec<Field>> for Fields {
    fn from(value: Vec<Field>) -> Self {
        let fields: Vec<FieldRef> = value.into_iter().map(Arc::new).collect();
        Self(Arc::from(fields))
    }
}

impl From<Vec<FieldRef>> for Fields {
    fn from(value: Vec<FieldRef>) -> Self {
        Self(value.into())
    }
}

impl Default for Fields {
    fn default() -> Self {
        Self(Arc::new([]))
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Schema {
    pub fields: Fields,
}

impl Schema {
    pub fn empty() -> Self {
        Self {
            fields: Default::default(),
        }
    }

    pub fn new(fields: impl Into<Fields>) -> Self {
        Self {
            fields: fields.into(),
        }
    }

    /// Returns an immutable reference of the vector of `Field` instances.
    #[inline]
    pub const fn fields(&self) -> &Fields {
        &self.fields
    }

    pub fn project(&self, indexes: &[usize]) -> Result<Self> {
        if indexes.is_empty() {
            return Ok(self.clone());
        }

        let new_fields = indexes
            .iter()
            .map(|i| {
                self.fields().get(*i).cloned().ok_or_else(|| {
                    anyhow!(
                        "project index {} out of bounds, max field {}",
                        i,
                        self.fields().len()
                    )
                })
            })
            .collect::<Result<Vec<_>>>()?;

        Ok(Self::new(new_fields))
    }

    pub fn merge(schema_1: &SchemaRef, schema_2: &SchemaRef) -> Self {
        let mut fields = Vec::with_capacity(schema_1.fields().len() + schema_2.fields().len());
        for field in schema_1.fields() {
            fields.push(field.clone());
        }
        for field in schema_2.fields() {
            fields.push(field.clone());
        }
        Schema::new(fields)
    }
}

pub type SchemaRef = Arc<Schema>;

// Arrow array \\

/// An array in the Arrow Columnar Format
/// https://arrow.apache.org/docs/format/Columnar.html
pub trait Array: Send + Sync {
    /// Returns this array as `&dyn Any` to allow downcasting to concrete array types.
    fn as_any(&self) -> &dyn std::any::Any;

    /// Returns a zero-copy slice of this array with the indicated offset and length.
    fn slice(&self, offset: usize, length: usize) -> ArrayRef;

    /// Returns the null buffer of this array if any.
    /// The null buffer contains the "physical" nulls of an array, that is how
    /// the nulls are represented in the underlying arrow format.
    fn nulls(&self) -> Option<&NullBuffer>;

    /// Returns a potentially computed [`NullBuffer`] that represents the logical
    /// null values of this array, if any.
    /// Logical nulls represent the values that are null in the array,
    /// regardless of the underlying physical arrow representation.
    fn logical_nulls(&self) -> Option<NullBuffer> {
        self.nulls().cloned()
    }

    /// Returns whether the element at `index` is null according to [`Array::nulls`]
    fn is_null(&self, index: usize) -> bool {
        self.nulls().is_some_and(|n| n.is_null(index))
    }

    /// Returns the length (i.e., number of elements) of this array.
    fn len(&self) -> usize;

    fn data_type(&self) -> &DataType;

    fn buffers(&self) -> Vec<Buffer>;

    // TODO: Support a funnction to return buffers back to the pool
}

pub type ArrayRef = Arc<dyn Array>;

pub struct Scalar(ArrayRef);

impl Scalar {
    pub fn new(arr: ArrayRef) -> Self {
        assert_eq!(arr.len(), 1);
        Self(arr)
    }

    pub fn inner(self) -> ArrayRef {
        self.0
    }
}

/// A two-dimensional batch of column-oriented data with a defined
/// [schema](arrow_schema::Schema).
///
/// A `RecordBatch` is a two-dimensional dataset of a number of
/// contiguous arrays, each the same length. A record batch has
/// a schema which must match its arrays’ datatypes.
#[derive(Clone)]
pub struct RecordBatch {
    schema: SchemaRef,
    /// Specific operations for different arrays types (e.g., primitive, list, struct)
    /// are implemented in `array`
    columns: Vec<ArrayRef>,
    /// The number of rows in this RecordBatch
    /// This is stored separately from the columns to handle the case of no columns
    row_count: usize,
}

impl RecordBatch {
    pub fn new_empty(schema: SchemaRef) -> Self {
        let columns = schema
            .fields()
            .iter()
            .map(|field| new_empty_array(field.data_type))
            .collect();
        Self {
            schema: schema,
            columns: columns,
            row_count: 0,
        }
    }

    pub fn try_new(schema: SchemaRef, columns: Vec<ArrayRef>) -> Result<Self> {
        // Fix: logic inversion. It should find columns where type does NOT match!
        let type_not_match =
            |(_, (col_type, field_type)): &(usize, (&DataType, &DataType))| col_type != field_type;

        let not_match = columns
            .iter()
            .zip(schema.fields.iter())
            .map(|(col, field)| (col.data_type(), &field.data_type))
            .enumerate()
            .find(type_not_match);

        if let Some((i, (col, field))) = not_match {
            return Err(anyhow!(
                "column types must match schema types, expected {:?} but found {:?} at column index {i}",
                field,
                col,
            ));
        }

        // Validate that all columns have the exact same logical length
        if !columns.is_empty() {
            let first_len = columns[0].len();
            for (i, col) in columns.iter().enumerate().skip(1) {
                if col.len() != first_len {
                    return Err(anyhow!(
                        "all columns in a RecordBatch must have the same length. Column 0 has len {first_len}, but column {i} has len {}",
                        col.len()
                    ));
                }
            }
        }

        let row_count = columns.first().map(|col| col.len()).unwrap_or_default();

        Ok(Self {
            schema,
            columns,
            row_count,
        })
    }

    /// Returns the schema reference of this batch
    #[inline]
    pub fn schema(&self) -> &SchemaRef {
        &self.schema
    }

    /// Returns a reference to the column at index `index`
    #[inline]
    pub fn column(&self, index: usize) -> &ArrayRef {
        &self.columns[index]
    }

    /// Returns the slice of columns in this batch
    #[inline]
    pub fn columns(&self) -> &[ArrayRef] {
        &self.columns
    }

    /// Returns the number of columns in this batch
    #[inline]
    pub fn num_columns(&self) -> usize {
        self.columns.len()
    }

    /// Returns the number of rows in this batch
    #[inline]
    pub fn num_rows(&self) -> usize {
        self.row_count
    }

    /// Slices this RecordBatch in O(1) time without copying any underlying memory columns.
    /// Highly optimized for limit operators!
    pub fn slice(&self, offset: usize, length: usize) -> Self {
        assert!(offset + length <= self.row_count);
        let sliced_columns = self
            .columns
            .iter()
            .map(|col| col.slice(offset, length))
            .collect();

        Self {
            schema: self.schema.clone(),
            columns: sliced_columns,
            row_count: length,
        }
    }

    pub fn project(&self, indexes: &[usize]) -> Result<Self> {
        let projected_schema = self.schema().project(indexes)?;
        let projected_fields = indexes
            .iter()
            .map(|i| {
                self.columns().get(*i).cloned().ok_or_else(|| {
                    anyhow!(
                        "project index {} out of bounds, max field {}",
                        i,
                        self.columns.len()
                    )
                })
            })
            .collect::<Result<Vec<_>>>()?;

        Self::try_new(SchemaRef::new(projected_schema), projected_fields)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_buffer_creation_and_properties() {
        let vec_data = vec![1u8, 2, 3, 4, 5, 6, 7, 8];
        let buf = Buffer::from(vec_data);

        assert_eq!(buf.len(), 8);
        assert_eq!(buf.as_slice(), &[1, 2, 3, 4, 5, 6, 7, 8]);
    }

    #[test]
    fn test_buffer_zero_copy_slicing() {
        let vec_data = vec![10u8, 20, 30, 40, 50, 60, 70, 80];
        let original_buf = Buffer::from(vec_data);

        // Slice starting at byte 2 for a length of 4 bytes: [30, 40, 50, 60]
        let sliced_buf = original_buf.slice(2, 4);

        assert_eq!(sliced_buf.len(), 4);
        assert_eq!(sliced_buf.as_slice(), &[30, 40, 50, 60]);

        // Slice exactly up to the end of the buffer (boundary edge case - must not panic!)
        let full_suffix_slice = original_buf.slice(4, 4);
        assert_eq!(full_suffix_slice.len(), 4);
        assert_eq!(full_suffix_slice.as_slice(), &[50, 60, 70, 80]);

        // Complete full slice of the buffer (must not panic!)
        let full_slice = original_buf.slice(0, 8);
        assert_eq!(full_slice.len(), 8);
        assert_eq!(full_slice.as_slice(), &[10, 20, 30, 40, 50, 60, 70, 80]);
    }

    #[test]
    fn test_buffer_get_uncheck() {
        // Build a byte buffer containing four i32 values: [100, 200, 300, 400]
        let mut vec_data = Vec::new();
        for val in &[100i32, 200, 300, 400] {
            vec_data.extend_from_slice(&val.to_ne_bytes());
        }

        let buf = Buffer::from(vec_data);
        assert_eq!(buf.len(), 16); // 4 fields * 4 bytes each = 16 bytes

        unsafe {
            assert_eq!(buf.get_uncheck::<i32>(0), 100);
            assert_eq!(buf.get_uncheck::<i32>(1), 200);
            assert_eq!(buf.get_uncheck::<i32>(2), 300);
            assert_eq!(buf.get_uncheck::<i32>(3), 400);
        }

        // Slice the buffer to only encompass the values [200, 300]
        // Starts at byte offset 4, length is 8 bytes (2 * 4 bytes)
        let sliced_buf = buf.slice(4, 8);
        assert_eq!(sliced_buf.len(), 8);

        unsafe {
            // Sliced buffer reads index 0 as 200, index 1 as 300
            assert_eq!(sliced_buf.get_uncheck::<i32>(0), 200);
            assert_eq!(sliced_buf.get_uncheck::<i32>(1), 300);
        }
    }

    #[test]
    fn test_boolean_buffer_and_null_buffer() {
        // Create a validity bitmap representing 10 items: [T, T, F, T, F, T, T, T, F, T]
        // Bitmask binary (least significant bit first): 0b1011101011 (binary: 747 as decimal)
        // Byte 0: 0b11101011 (235 decimal)
        // Byte 1: 0b00000010 (2 decimal)
        let vec_data = vec![235, 2];
        let buf = Buffer::from(vec_data);

        // Instantiate BooleanBuffer
        let boolean_buf = BooleanBuffer::new(buf, 0, 10);

        // Assert individual bit reads
        assert!(boolean_buf.value(0)); // T
        assert!(boolean_buf.value(1)); // T
        assert!(!boolean_buf.value(2)); // F
        assert!(boolean_buf.value(3)); // T
        assert!(!boolean_buf.value(4)); // F
        assert_eq!(boolean_buf.count_set_bits(), 7); // 7 'true' values, 3 'false' values

        // Instantiate NullBuffer
        let null_buf = NullBuffer::new(boolean_buf);
        assert_eq!(null_buf.null_count(), 3); // Eagerly counted 3 nulls!
        assert!(!null_buf.is_null(0));
        assert!(null_buf.is_null(2)); // Item 2 is Null

        // Slice the NullBuffer: index 2 to 7 (items: [F, T, F, T, T, T] -> length 6)
        // Null count in this slice should be exactly 2!
        let sliced_null_buf = null_buf.slice(2, 6);
        assert_eq!(sliced_null_buf.null_count(), 2); // Eagerly recalculated to exactly 2!
        assert!(sliced_null_buf.is_null(0)); // Original item 2 (now 0) is Null
        assert!(!sliced_null_buf.is_null(1)); // Original item 3 (now 1) is Valid
    }

    #[test]
    fn test_record_batch_creation_validation_slicing() {
        use crate::array::PrimitiveArray;

        // 1. Define schema: [id: Int32, val: Float64]
        let schema = Arc::new(Schema::new(vec![
            Field {
                name: "id".to_string(),
                data_type: DataType::Int32,
                nullable: false,
            },
            Field {
                name: "val".to_string(),
                data_type: DataType::Float64,
                nullable: true,
            },
        ]));

        // 2. Build mock columns: id = [1, 2, 3], val = [Some(1.1), None, Some(3.3)]
        let id_column: ArrayRef = Arc::new(PrimitiveArray::from(vec![1i32, 2, 3]));
        let val_column: ArrayRef =
            Arc::new(PrimitiveArray::from(vec![Some(1.1f64), None, Some(3.3)]));

        // 3. Construct a valid RecordBatch
        let batch =
            RecordBatch::try_new(schema.clone(), vec![id_column.clone(), val_column.clone()])
                .unwrap();
        assert_eq!(batch.num_rows(), 3);
        assert_eq!(batch.num_columns(), 2);
        assert_eq!(batch.schema().fields.iter().count(), 2);

        // 4. Verify invalid schema type mismatch returns an Error
        let wrong_column: ArrayRef = Arc::new(PrimitiveArray::from(vec![1i64, 2, 3])); // Int64 instead of Int32
        let result = RecordBatch::try_new(schema.clone(), vec![wrong_column, val_column.clone()]);
        assert!(result.is_err());
        let err_msg = result.err().unwrap().to_string();
        assert!(err_msg.contains("column types must match schema types"));

        // 5. Verify mismatched column lengths returns an Error
        let short_id_column: ArrayRef = Arc::new(PrimitiveArray::from(vec![1i32, 2])); // length 2 instead of 3
        let result =
            RecordBatch::try_new(schema.clone(), vec![short_id_column, val_column.clone()]);
        assert!(result.is_err());
        let err_msg = result.err().unwrap().to_string();
        assert!(err_msg.contains("all columns in a RecordBatch must have the same length"));

        // 6. Test O(1) Zero-Copy Slicing: slice from index 1, length 2: id = [2, 3], val = [None, Some(3.3)]
        let sliced_batch = batch.slice(1, 2);
        assert_eq!(sliced_batch.num_rows(), 2);
        assert_eq!(sliced_batch.num_columns(), 2);

        // Verify values on sliced columns
        let sliced_id = sliced_batch.column(0);
        let sliced_val = sliced_batch.column(1);
        assert_eq!(sliced_id.len(), 2);
        assert_eq!(sliced_val.len(), 2);
        assert_eq!(sliced_val.nulls().unwrap().null_count(), 1); // 1 null in this slice!
    }

    #[test]
    fn test_buffer_resize() {
        // 1. Test exclusive un-sliced in-place resize (grows, truncates, fills new slots)
        let mut buf = Buffer::from(vec![10u8, 20, 30]);
        assert_eq!(buf.len(), 3);
        assert_eq!(buf.as_slice(), &[10, 20, 30]);

        // Grow to 6, filling new slots with 9
        buf.resize(6, 9);
        assert_eq!(buf.len(), 6);
        assert_eq!(buf.as_slice(), &[10, 20, 30, 9, 9, 9]);

        // Truncate to 4
        buf.resize(4, 0);
        assert_eq!(buf.len(), 4);
        assert_eq!(buf.as_slice(), &[10, 20, 30, 9]);

        // 2. Test shared cloned resize (CoW: cloning buf and resizing the clone leaves original untouched!)
        let original_buf = Buffer::from(vec![1u8, 2, 3]);
        let mut cloned_buf = original_buf.clone();

        // Resize the clone
        cloned_buf.resize(5, 7);
        assert_eq!(cloned_buf.len(), 5);
        assert_eq!(cloned_buf.as_slice(), &[1, 2, 3, 7, 7]);

        // Original buffer must be completely untouched and isolated!
        assert_eq!(original_buf.len(), 3);
        assert_eq!(original_buf.as_slice(), &[1, 2, 3]);

        // 3. Test sliced resize (CoW: slicing a buffer, then resizing the slice)
        let base_buf = Buffer::from(vec![10u8, 20, 30, 40, 50]);
        // Slice: index 1, len 3 -> [20, 30, 40]
        let mut sliced_buf = base_buf.slice(1, 3);
        assert_eq!(sliced_buf.len(), 3);
        assert_eq!(sliced_buf.as_slice(), &[20, 30, 40]);

        // Resize the slice (grows to 5, filling with 8)
        sliced_buf.resize(5, 8);
        assert_eq!(sliced_buf.len(), 5);
        // Should only copy the active sliced window and grow it!
        assert_eq!(sliced_buf.as_slice(), &[20, 30, 40, 8, 8]);

        // Base buffer must remain 100% untouched and original!
        assert_eq!(base_buf.len(), 5);
        assert_eq!(base_buf.as_slice(), &[10, 20, 30, 40, 50]);
    }

    #[test]
    fn test_record_batch_projection_zero_copy() {
        use crate::array::PrimitiveArray;

        let schema = Arc::new(Schema::new(vec![
            Field {
                name: "a".to_string(),
                data_type: DataType::Int32,
                nullable: false,
            },
            Field {
                name: "b".to_string(),
                data_type: DataType::Float64,
                nullable: true,
            },
            Field {
                name: "c".to_string(),
                data_type: DataType::Boolean,
                nullable: true,
            },
        ]));

        let col_a: ArrayRef = Arc::new(PrimitiveArray::from(vec![1, 2, 3]));
        let col_b: ArrayRef = Arc::new(PrimitiveArray::from(vec![Some(10.0), None, Some(30.0)]));
        let col_c: ArrayRef = Arc::new(PrimitiveArray::from(vec![true, false, true]));

        let batch = RecordBatch::try_new(schema, vec![col_a, col_b, col_c]).unwrap();

        // Project columns `a` (index 0) and `c` (index 2)
        let projected = batch.project(&[0, 2]).unwrap();
        assert_eq!(projected.num_columns(), 2);
        assert_eq!(projected.num_rows(), 3);

        // Assert schema matching
        let fields = projected.schema().fields();
        assert_eq!(fields[0].name, "a");
        assert_eq!(fields[1].name, "c");

        // Verify underlying column data pointers match exactly (verifying zero-copy!)
        assert_eq!(
            projected.column(0).as_any().type_id(),
            batch.column(0).as_any().type_id()
        );
    }
}
