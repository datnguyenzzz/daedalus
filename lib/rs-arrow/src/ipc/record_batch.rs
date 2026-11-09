/*
+----------------------------------------------------------------------------------------------------------------------+
|                                                    IPC MESSAGE                                                       |
+-----------------------------------+---------------------------------------------------+------------------------------+
| Prefix (8 bytes)                  | Metadata Header (8-byte aligned)                  | Body (8-byte aligned)        |
+-----------------+-----------------+---------------------------------------------------+------------------------------+
| Continuation    | Metadata Length | FlatBuffer `FbMessage`                            | Contiguous Column Buffers    |
| 0xFFFFFFFF      | (4 bytes, LE)   | (version, header_type, FbRecordBatch, body_length)| (Zstd-compressed or raw)     |
| (4 bytes)       |                 | + Padding to 8-byte boundary                      |                              |
+-----------------+-----------------+---------------------------------------------------+------------------------------+

METADATA HEADER (`FbRecordBatch`):
   +-------------------------------------------------------------------------------------------------------------------+
   | length: i32       | Row count of the RecordBatch                                                                  |
   | compression: i8   | 0 = None, 1 = Zstd                                                                            |
   | nodes: Vector     | Per-Field Logical Info:                                                                       |
   |                   | | length (i64) | null_count (i64) | stats_byte (ColumnStat min/max zonemap) |                  |
   | segments: Vector  | Per-Buffer Physical Offsets in Body:                                                          |
   |                   | | offset (i64, rel to body start) | length (i64, bytes in body) |                             |
   +-------------------------------------------------------------------------------------------------------------------+

BODY LAYOUT (Concatenation of all column segments, aligned to 8-byte boundaries):
   +-------------------------------------------------------------------------------------------------------------------+
   | Field 0 (e.g. Primitive / Boolean):                                                                               |
   |   - Segment 0 (Null Bitmap):   [uncompressed_len: i64][zstd compressed bytes][padding to 8B]                      |
   |   - Segment 1 (Values):        [uncompressed_len: i64][zstd compressed bytes][padding to 8B]                      |
   | Field 1 (e.g. Utf8 String):                                                                                       |
   |   - Segment 2 (Null Bitmap):   [uncompressed_len: i64][zstd compressed bytes][padding to 8B]                      |
   |   - Segment 3 (Offsets):       [uncompressed_len: i64][zstd compressed bytes][padding to 8B]                      |
   |   - Segment 4 (Data bytes):    [uncompressed_len: i64][zstd compressed bytes][padding to 8B]                      |
   | ...                                                                                                               |
   +-------------------------------------------------------------------------------------------------------------------+
*/

use std::{
    io::{Read, Write},
    sync::Arc,
};

use anyhow::{Result, anyhow};
use flatbuffers::{FlatBufferBuilder, ForwardsUOffset, Vector, WIPOffset, root};

use crate::{
    ArrayRef, BooleanBuffer, Buffer, DataType, NullBuffer, RecordBatch, Schema, SchemaRef,
    array::{BooleanArray, PrimitiveArray, StringArray},
    ipc::{
        FbMessage, FbMessageBuilder, FbMetaNode, FbMetaNodeBuilder, FbMetaSegment,
        FbMetaSegmentBuilder, FbRecordBatch, FbRecordBatchBuilder,
        schema::{fb_to_schema, schema_to_fb_offset},
    },
    object::{ColumnStat, compute_column_stat},
};

const ALIGNMENT: usize = 8 - 1;
const CONTINUATION_MARKER: [u8; 4] = [0xff; 4];
const PADDING: [u8; 8] = [0u8; 8];

pub enum MessageType {
    Schema = 1,
    RecordBatch = 2,
    MetaOnly = 3,
}

/// Arrow Writer
/// Writes Arrow [`RecordBatch`] to bytes using the [IPC Streaming Format].
/// https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format
pub struct Writer<W: Write> {
    writer: W,
    compress: bool,
}

impl<W: Write> Writer<W> {
    #[inline]
    fn align(x: usize) -> usize {
        (x + ALIGNMENT) & !ALIGNMENT
    }

    pub fn new(writer: W) -> Self {
        Self {
            writer,
            compress: true,
        }
    }

    /// Write a record batch to the stream
    pub fn write(&mut self, batch: &RecordBatch) -> Result<()> {
        let (meta_buf, body_buf) = self.encode_batch(batch, self.compress)?;
        self.write_buf(meta_buf, body_buf)?;
        Ok(())
    }

    /// Write only meta node (num rows, null count, Stats) to the stream
    pub fn write_meta_node(
        &mut self,
        nodes: Vec<(i64, i64, ColumnStat)>,
        num_rows: i32,
    ) -> Result<()> {
        let mut fbb = FlatBufferBuilder::new();
        let fb_nodes = self.encode_node(&mut fbb, nodes);
        let header_offset = {
            let nodes_vec = fbb.create_vector(&fb_nodes);
            let mut batch_builder = FbRecordBatchBuilder::new(&mut fbb);
            batch_builder.push_length(num_rows);
            batch_builder.push_nodes(nodes_vec);
            batch_builder.finish()
        };

        let union_header = header_offset.as_union_value();

        let mut message_builder = FbMessageBuilder::new(&mut fbb);
        message_builder.push_version(1);
        message_builder.push_header_type(MessageType::MetaOnly as u8);
        message_builder.push_header(union_header);
        message_builder.push_body_length(0);
        let root = message_builder.finish();

        fbb.finish(root, None);
        let meta_fb = fbb.finished_data();

        self.write_buf(Buffer::from(meta_fb.to_vec()), Buffer::from(vec![]))?;
        Ok(())
    }

    fn encode_node<'a>(
        &self,
        mut fbb: &mut FlatBufferBuilder<'a>,
        nodes: Vec<(i64, i64, ColumnStat)>,
    ) -> Vec<WIPOffset<FbMetaNode<'a>>> {
        let mut fb_nodes = Vec::new();
        for (len, null_count, stat) in nodes {
            let stat_offset = fbb.create_vector(&stat.encode());
            let mut node_builder = FbMetaNodeBuilder::new(&mut fbb);
            node_builder.push_length(len);
            node_builder.push_null_count(null_count);
            node_builder.push_stats_byte(stat_offset);
            fb_nodes.push(node_builder.finish());
        }

        fb_nodes
    }

    /// Compresses and encodes a RecordBatch into its FlatBuffer metadata header buffer
    /// and Zstd-compressed body buffer
    fn encode_batch(&self, batch: &RecordBatch, compress: bool) -> Result<(Buffer, Buffer)> {
        let mut body_bytes: Vec<u8> = Vec::new();
        // represents (column length, null count, [`ColumnStat`]) of column
        let mut nodes: Vec<(i64, i64, ColumnStat)> = Vec::new();
        // represents (offset, length) of a continuous decoded block
        let mut segments: Vec<(i64, i64)> = Vec::new();

        let mut compress_and_encode = |segments: &mut Vec<(i64, i64)>, buf: Option<Buffer>| {
            let start = body_bytes.len();

            if let Some(buf) = buf {
                if compress {
                    let uncompressed_len = buf.len();
                    let compressed = zstd::stream::encode_all(buf.as_slice(), 0)
                        .expect("Failed to compress buffer with Zstd");

                    if compressed.len() < uncompressed_len {
                        // Prepend uncompressed length (i64, little-endian) per spec
                        body_bytes.extend_from_slice(&(uncompressed_len as i64).to_le_bytes());
                        body_bytes.extend_from_slice(&compressed);
                    } else {
                        // Compression was not effective; fall back to uncompressed and write -1 as prefix
                        body_bytes.extend_from_slice(&(-1i64).to_le_bytes());
                        body_bytes.extend_from_slice(buf.as_slice());
                    }
                } else {
                    body_bytes.extend_from_slice(buf.as_slice());
                }

                let len = body_bytes.len() - start;
                segments.push((start as i64, len as i64));

                // Padding each buffer chunk to an 8-byte boundary per Arrow spec
                let aligned_len = Self::align(body_bytes.len());
                let padding = aligned_len - body_bytes.len();
                body_bytes.extend_from_slice(&PADDING[..padding]);
            } else {
                // Buffer is None (e.g. nullable column with 0 nulls, validity mask omitted)
                segments.push((start as i64, 0));
            }
        };

        // Gather all nodes (logical properties) and segments (physical offsets)
        for col_ref in batch.columns() {
            let stats = compute_column_stat(col_ref.as_ref()).unwrap();
            nodes.push((
                col_ref.len() as i64,
                col_ref.nulls().map(|n| n.null_count()).unwrap_or_default() as i64,
                stats,
            ));

            compress_and_encode(
                &mut segments,
                col_ref.nulls().map(|n| n.buffer.buffer.clone()),
            );

            for buf in col_ref.buffers() {
                compress_and_encode(&mut segments, Some(buf));
            }
        }

        let mut fbb = FlatBufferBuilder::new();
        let fb_nodes = self.encode_node(&mut fbb, nodes);

        let mut fb_segments = Vec::new();
        for (offset, length) in segments {
            let mut segment_builder = FbMetaSegmentBuilder::new(&mut fbb);
            segment_builder.push_offset(offset);
            segment_builder.push_length(length);
            fb_segments.push(segment_builder.finish());
        }

        let header_offset = {
            let nodes_vec = fbb.create_vector(&fb_nodes);
            let segments_vec = fbb.create_vector(&fb_segments);

            let mut batch_builder = FbRecordBatchBuilder::new(&mut fbb);
            batch_builder.push_length(batch.num_rows() as i32);
            batch_builder.push_nodes(nodes_vec);
            batch_builder.push_segments(segments_vec);
            if compress {
                batch_builder.push_compression(1 /* Zstd */);
            }
            batch_builder.finish()
        };
        let union_header = header_offset.as_union_value();

        let mut message_builder = FbMessageBuilder::new(&mut fbb);
        message_builder.push_version(1);
        message_builder.push_header_type(MessageType::RecordBatch as u8);
        message_builder.push_header(union_header);
        message_builder.push_body_length(body_bytes.len() as i64);
        let root = message_builder.finish();

        fbb.finish(root, None);
        let meta_fb = fbb.finished_data();

        Ok((Buffer::from(meta_fb.to_vec()), Buffer::from(body_bytes)))
    }

    fn write_buf(&mut self, meta_buf: Buffer, body_buf: Buffer) -> Result<()> {
        self.write_continuation(meta_buf.len())?;
        self.writer.write_all(meta_buf.as_slice())?;

        // Metadata alignment padding to 8-byte boundary before body
        let written_bytes = 8 + meta_buf.len();
        self.write_padding(Self::align(written_bytes) - written_bytes)?;

        // Write the contiguous raw body bytes
        self.writer.write_all(body_buf.as_slice())?;
        Ok(())
    }
    /// Writes a schema to an IPC message, returning metadata written
    pub fn write_schema(&mut self, schema: &Schema) -> Result<usize> {
        let mut fbb = FlatBufferBuilder::new();

        let schema_offset = schema_to_fb_offset(&mut fbb, schema);
        let union_header = schema_offset.as_union_value();

        let mut message_builder = FbMessageBuilder::new(&mut fbb);
        message_builder.push_version(1);
        message_builder.push_header_type(MessageType::Schema as u8);
        message_builder.push_header(union_header);
        message_builder.push_body_length(0);
        let root = message_builder.finish();

        fbb.finish(root, None);
        let bytes = fbb.finished_data();

        let len = bytes.len();
        let padded_header_len = Self::align(8 + len);
        let padded_metadata_len = padded_header_len - 8;
        let padding_needed = padded_metadata_len - len;

        self.write_continuation(padded_metadata_len)?;
        self.writer.write_all(bytes)?;
        self.write_padding(padding_needed)?;

        Ok(padded_header_len)
    }

    /// Writes the IPC continuation marker and metadata length prefix.
    fn write_continuation(&mut self, metadata_len: usize) -> Result<()> {
        let mut buffer = [0u8; 8];
        buffer[..4].copy_from_slice(&CONTINUATION_MARKER);
        buffer[4..].copy_from_slice(&(metadata_len as i32).to_le_bytes());
        self.writer.write_all(&buffer)?;
        Ok(())
    }

    /// Zero-allocation padding writer
    fn write_padding(&mut self, len: usize) -> Result<()> {
        if len > 0 {
            self.writer.write_all(&PADDING[..len])?;
        }
        Ok(())
    }
}

const MAX_PREALLOC_BYTES: usize = 64 * 1024 * 1024;

/// Reads exactly `len` bytes of message body, without reserving `len` before reading it.
pub fn read_bounded<R: Read>(reader: &mut R, len: usize) -> Result<Buffer> {
    let mut buf = Buffer::from_zeroed(len.min(MAX_PREALLOC_BYTES));
    let mut filled = 0;
    while filled < len {
        let end = buf.len();
        reader.read_exact(&mut buf.as_slice_mut()[filled..end])?;
        filled = end;
        if filled < len {
            buf.resize(len.min(end.saturating_mul(2)), 0);
        }
    }
    Ok(buf)
}

// see Writer.encode_batch() to understand the encoding logic
fn segments_per_field(data_type: &DataType) -> usize {
    match data_type {
        DataType::Utf8 => 3, // validity + offset + actual values
        _ => 2,              // validity + values
    }
}

fn compute_field_segment_indexes(schema: &Schema) -> Vec<usize> {
    let mut indexes = Vec::with_capacity(schema.fields().len());
    let mut idx = 0;
    for field in schema.fields() {
        indexes.push(idx);
        idx += segments_per_field(&field.data_type);
    }
    indexes
}

/// Read [`FbMessage`] from a reader while re-using a buffer for metadata
pub struct MessageReader<R: Read> {
    reader: R,
    buf: Vec<u8>,
}

impl<R: Read> MessageReader<R> {
    pub fn new(reader: R) -> Self {
        Self {
            reader,
            buf: Vec::new(),
        }
    }

    /// Reads the entire next message from the underlying reader which includes
    /// the metadata length, the metadata
    pub fn try_next_meta_only(&mut self) -> Result<Option<FbMessage<'_>>> {
        let meta_len = self.read_meta_len()?;
        let Some(meta_len) = meta_len else {
            return Ok(None);
        };

        self.buf.clear();
        let read = (&mut self.reader)
            .take(meta_len as u64)
            .read_to_end(&mut self.buf)?;

        if read != meta_len {
            return Err(anyhow!(
                "Unexpected end of stream: expected {meta_len} metadata bytes, got {read}"
            ));
        }

        Ok(Some(root::<FbMessage>(&self.buf.as_slice())?))
    }

    /// Reads the entire next message from the underlying reader which includes
    /// the metadata length, the metadata, and the body
    pub fn try_next(&mut self) -> Result<Option<(FbMessage<'_>, Buffer)>> {
        let meta_len = self.read_meta_len()?;
        let Some(meta_len) = meta_len else {
            return Ok(None);
        };

        self.buf.clear();
        let read = (&mut self.reader)
            .take(meta_len as u64)
            .read_to_end(&mut self.buf)?;

        if read != meta_len {
            return Err(anyhow!(
                "Unexpected end of stream: expected {meta_len} metadata bytes, got {read}"
            ));
        }

        let message = root::<FbMessage>(&self.buf.as_slice())?;
        let body_length = usize::try_from(message.body_length())?;
        let buf = read_bounded(&mut self.reader, body_length)?;

        Ok(Some((message, buf)))
    }

    /// Read the metadata length for the next message from the underlying stream.
    fn read_meta_len(&mut self) -> Result<Option<usize>> {
        let mut meta_len: [u8; 4] = [0; 4];
        match self.reader.read_exact(&mut meta_len) {
            Ok(_) => {}
            Err(e) => {
                return if e.kind() == std::io::ErrorKind::UnexpectedEof {
                    Ok(None)
                } else {
                    Err(e.into())
                };
            }
        };
        let meta_len = {
            if meta_len == CONTINUATION_MARKER {
                self.reader.read_exact(&mut meta_len)?
            }
            i32::from_le_bytes(meta_len)
        };
        if meta_len == 0 {
            return Ok(None);
        }
        let meta_len = usize::try_from(meta_len)?;
        Ok(Some(meta_len))
    }
}
struct RecordBatchDecoder<'a, 'b> {
    pub buf: &'a Buffer,
    pub schema: SchemaRef,
    projection: &'b [usize],
    segments: Vector<'a, ForwardsUOffset<FbMetaSegment<'a>>>,
    is_compressed: bool,
    row_count: i32,
}

impl<'a, 'b> RecordBatchDecoder<'a, 'b> {
    pub fn new(
        buf: &'a Buffer,
        fb_batch: FbRecordBatch<'a>,
        schema: SchemaRef,
        projection: &'b [usize],
    ) -> Self {
        let segments = fb_batch.segments().unwrap();
        let is_compressed = fb_batch.compression() != 0;
        let row_count = fb_batch.length();
        Self {
            buf,
            schema,
            projection,
            segments,
            is_compressed,
            row_count,
        }
    }

    fn decompress_segment(&self, idx: usize) -> Result<Option<Buffer>> {
        if idx >= self.segments.len() {
            return Err(anyhow!("Segment index {idx} out of bounds"));
        }

        let segment = self.segments.get(idx);
        let length = segment.length() as usize;
        if length == 0 {
            return Ok(None);
        }

        let offset = segment.offset() as usize;
        let raw_slice = self.buf.slice(offset, length);

        if self.is_compressed {
            let uncompressed_len = unsafe { raw_slice.get_uncheck::<i64>(0) } as usize;

            if uncompressed_len == usize::MAX {
                // -1 as signed 64-bit int
                Ok(Some(raw_slice.slice(8, length - 8)))
            } else {
                // Decompress the payload starting from byte offset 8
                let compressed_payload = &raw_slice.as_slice()[8..];
                let decompressed = zstd::stream::decode_all(compressed_payload)?;
                assert_eq!(decompressed.len(), uncompressed_len);
                Ok(Some(Buffer::from(decompressed)))
            }
        } else {
            Ok(Some(raw_slice))
        }
    }

    pub fn decode_field(&self, idx: usize, segment_idx: usize) -> Result<ArrayRef> {
        let field = &self.schema.fields()[idx];
        let validity_buf = self.decompress_segment(segment_idx)?;
        let nulls = validity_buf
            .map(|v_buf| NullBuffer::new(BooleanBuffer::new(v_buf, 0, self.row_count as usize)));

        match field.data_type {
            DataType::Boolean => {
                let val_buf = self.decompress_segment(segment_idx + 1)?.ok_or_else(|| {
                    anyhow!("Malformed IPC: missing values buffer for BooleanArray")
                })?;
                let boolean_buf = BooleanBuffer::new(val_buf, 0, self.row_count as usize);
                Ok(Arc::new(BooleanArray::new(boolean_buf, nulls)))
            }
            DataType::Utf8 => {
                let offset_buf = self.decompress_segment(segment_idx + 1)?.ok_or_else(|| {
                    anyhow!("Malformed IPC: missing offsets buffer for StringArray")
                })?;
                let data_buf = self
                    .decompress_segment(segment_idx + 2)?
                    .ok_or_else(|| anyhow!("Malformed IPC: missing data buffer for StringArray"))?;
                Ok(Arc::new(StringArray::new(offset_buf, data_buf, nulls)))
            }
            // Primitive numerical types
            DataType::Int8 => {
                let val_buf = self
                    .decompress_segment(segment_idx + 1)?
                    .ok_or_else(|| anyhow!("Missing values buffer"))?;
                Ok(Arc::new(PrimitiveArray::<i8>::new(
                    field.data_type,
                    val_buf,
                    nulls,
                )))
            }
            DataType::Int16 => {
                let val_buf = self
                    .decompress_segment(segment_idx + 1)?
                    .ok_or_else(|| anyhow!("Missing values buffer"))?;
                Ok(Arc::new(PrimitiveArray::<i16>::new(
                    field.data_type,
                    val_buf,
                    nulls,
                )))
            }
            DataType::Int32 => {
                let val_buf = self
                    .decompress_segment(segment_idx + 1)?
                    .ok_or_else(|| anyhow!("Missing values buffer"))?;
                Ok(Arc::new(PrimitiveArray::<i32>::new(
                    field.data_type,
                    val_buf,
                    nulls,
                )))
            }
            DataType::Int64 => {
                let val_buf = self
                    .decompress_segment(segment_idx + 1)?
                    .ok_or_else(|| anyhow!("Missing values buffer"))?;
                Ok(Arc::new(PrimitiveArray::<i64>::new(
                    field.data_type,
                    val_buf,
                    nulls,
                )))
            }
            DataType::Float32 => {
                let val_buf = self
                    .decompress_segment(segment_idx + 1)?
                    .ok_or_else(|| anyhow!("Missing values buffer"))?;
                Ok(Arc::new(PrimitiveArray::<f32>::new(
                    field.data_type,
                    val_buf,
                    nulls,
                )))
            }
            DataType::Float64 => {
                let val_buf = self
                    .decompress_segment(segment_idx + 1)?
                    .ok_or_else(|| anyhow!("Missing values buffer"))?;
                Ok(Arc::new(PrimitiveArray::<f64>::new(
                    field.data_type,
                    val_buf,
                    nulls,
                )))
            }
        }
    }

    pub fn try_decode(&mut self) -> Result<RecordBatch> {
        let field_segment_indexes = compute_field_segment_indexes(&self.schema);
        match self.projection.is_empty() {
            false => {
                let mut columns = Vec::with_capacity(self.projection.len());
                let mut projected_fields = Vec::with_capacity(self.projection.len());

                for &field_idx in self.projection {
                    if field_idx >= self.schema.fields().len() {
                        return Err(anyhow!(
                            "Projection index {} out of bounds for schema with {} fields",
                            field_idx,
                            self.schema.fields().len()
                        ));
                    }
                    columns.push(self.decode_field(field_idx, field_segment_indexes[field_idx])?);
                    projected_fields.push(self.schema.fields()[field_idx].clone());
                }

                let projected_schema = Arc::new(Schema::new(projected_fields));
                RecordBatch::try_new(projected_schema, columns)
            }
            true => {
                let schema = Arc::clone(&self.schema);
                let mut columns = Vec::with_capacity(schema.fields.iter().count());
                for (idx, _) in schema.fields().iter().enumerate() {
                    columns.push(self.decode_field(idx, field_segment_indexes[idx])?);
                }

                RecordBatch::try_new(schema, columns)
            }
        }
    }
}

pub struct Reader<'a, R: Read> {
    reader: MessageReader<R>,
    schema: SchemaRef,
    projection: &'a [usize],
    /// finished=true indicates we already reached to the end of a stream
    finished: bool,
}

impl<'a, R: Read> Reader<'a, R> {
    /// Returns the schema of this stream
    #[inline]
    pub fn schema(&self) -> &SchemaRef {
        &self.schema
    }

    pub fn with_projection(mut self, projection: &'a [usize]) -> Self {
        self.projection = projection;
        self
    }

    /// Creates a Reader for a stream that starts with a Schema message
    pub fn try_new(reader: R) -> Result<Self> {
        let mut msg_reader = MessageReader::new(reader);
        let message = msg_reader.try_next()?;

        let Some((message, _)) = message else {
            return Err(anyhow!("Expected schema message, found empty stream."));
        };

        // For schema message, the schema is written in the header block
        let fb_schema = message
            .header_as_fb_schema()
            .ok_or_else(|| anyhow!("Failed to parse schema from message header"))?;

        let schema = fb_to_schema(fb_schema);

        Ok(Self {
            reader: msg_reader,
            schema: Arc::new(schema),
            finished: false,
            projection: &[],
        })
    }

    /// Creates a Reader for a stream that contains only RecordBatch messages, using a known Schema
    pub fn try_new_with_schema(reader: R, schema: SchemaRef) -> Result<Self> {
        let msg_reader = MessageReader::new(reader);
        Ok(Self {
            reader: msg_reader,
            schema,
            finished: false,
            projection: &[],
        })
    }

    pub fn try_next(&mut self) -> Result<Option<RecordBatch>> {
        if self.finished {
            return Ok(None);
        }

        loop {
            let message = self.reader.try_next()?;
            let Some((message, body)) = message else {
                // we already reached to the end of the stream
                self.finished = true;
                return Ok(None);
            };

            match message.header_type() {
                /* Schema */
                1 => return Err(anyhow!("expected body bytes, but found schema")),
                /* Record Batch */
                2 => {
                    let batch_header = message
                        .header_as_fb_record_batch()
                        .ok_or_else(|| anyhow!("Unable to read IPC message as record batch"))?;

                    let mut decoder = RecordBatchDecoder::new(
                        &body,
                        batch_header,
                        self.schema.clone(),
                        self.projection,
                    );
                    let batch = decoder.try_decode()?;
                    return Ok(Some(batch));
                }
                /* Meta Node only, no actual record batch data */
                3 => return Ok(None),
                e => return Err(anyhow!("corrupted: unrecognise header type: {e}")),
            };
        }
    }
}

impl<R: Read> Iterator for Reader<'_, R> {
    type Item = Result<RecordBatch>;

    fn next(&mut self) -> Option<Self::Item> {
        self.try_next().transpose()
    }
}

pub unsafe fn root_as_message(buf: &[u8]) -> Result<FbMessage<'_>, flatbuffers::InvalidFlatbuffer> {
    flatbuffers::root::<FbMessage>(buf)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::array::PrimitiveArray;
    use crate::{Array, DataType, Field, Schema};
    use std::sync::Arc;

    #[test]
    fn test_record_batch_writer_roundtrip() {
        // 1. Define Schema: [id: Int32]
        let schema = Arc::new(Schema::new(vec![Field {
            name: "id".to_string(),
            data_type: DataType::Int32,
            nullable: false,
        }]));

        // 2. Build column id = [1000, 2000, 3000, 4000] (4 * i32 = 16 bytes)
        let id_column: ArrayRef = Arc::new(PrimitiveArray::from(vec![1000i32, 2000, 3000, 4000]));
        let batch = RecordBatch::try_new(schema, vec![id_column]).unwrap();

        // 3. Serialize utilizing Writer into a Vec<u8> buffer (with Zstd compression enabled!)
        let mut buffer = Vec::new();
        {
            let mut writer = Writer::new(&mut buffer);
            writer.write_schema(batch.schema()).unwrap();
            writer.write(&batch).unwrap();
        }

        // 4. Decode and Verify the serialized packet
        assert!(buffer.len() > 8);

        // --- SKIP SCHEMA MESSAGE BLOCK ---
        assert_eq!(&buffer[..4], &[0xFF, 0xFF, 0xFF, 0xFF]); // Schema continuation
        let schema_meta_len = i32::from_le_bytes(buffer[4..8].try_into().unwrap()) as usize;
        let schema_meta_bytes = &buffer[8..8 + schema_meta_len];

        // Parse first message as FbMessage (Schema)
        let fb_schema_msg = unsafe { root_as_message(schema_meta_bytes).unwrap() };
        assert_eq!(fb_schema_msg.header_type(), 1); // 1 = Schema
        assert_eq!(fb_schema_msg.body_length(), 0); // Schemas have no body

        let schema_message_size = Writer::<Vec<u8>>::align(8 + schema_meta_len);

        // --- READ RECORDBATCH MESSAGE BLOCK ---
        let batch_msg = &buffer[schema_message_size..];
        assert_eq!(&batch_msg[..4], &[0xFF, 0xFF, 0xFF, 0xFF]); // Batch continuation

        let metadata_len = i32::from_le_bytes(batch_msg[4..8].try_into().unwrap()) as usize;
        let metadata_bytes = &batch_msg[8..8 + metadata_len];

        // Parse second message as FbMessage (RecordBatch)
        let fb_msg = unsafe { root_as_message(metadata_bytes).unwrap() };
        assert_eq!(fb_msg.header_type(), 2); // 2 = RecordBatch
        assert!(fb_msg.body_length() > 0);

        // Extract the inner FbRecordBatch table!
        let header_table = fb_msg.header().unwrap();
        let fb_batch = FbRecordBatch::from_table(header_table);
        assert_eq!(fb_batch.length(), 4);
        assert_eq!(fb_batch.compression(), 1); // 1 = Zstd

        let fb_nodes = fb_batch.nodes().unwrap();
        assert_eq!(fb_nodes.len(), 1);
        assert_eq!(fb_nodes.get(0).length(), 4);
        assert_eq!(fb_nodes.get(0).null_count(), 0);

        let fb_segments = fb_batch.segments().unwrap();
        // 1 segment for validity null bitmap (which is empty length 0) and 1 for values
        assert_eq!(fb_segments.len(), 2);

        let nulls_segment = fb_segments.get(0);
        assert_eq!(nulls_segment.length(), 0); // null bitmap is empty

        let values_segment = fb_segments.get(1);
        assert!(values_segment.length() > 0);

        // 5. Decompress and verify values in the body
        let body_start = Writer::<Vec<u8>>::align(8 + metadata_len);
        let body_bytes = &batch_msg[body_start..];

        let val_offset = values_segment.offset() as usize;
        let val_len = values_segment.length() as usize;
        let compressed_val_bytes = &body_bytes[val_offset..val_offset + val_len];

        // Read uncompressed length prefix (i64, little-endian) per spec
        let uncompressed_size = i64::from_le_bytes(compressed_val_bytes[0..8].try_into().unwrap());

        let decompressed_bytes = if uncompressed_size == -1 {
            // Buffer was left uncompressed per spec! Read the raw 16 bytes starting at offset 8
            compressed_val_bytes[8..].to_vec()
        } else {
            assert_eq!(uncompressed_size, 16);
            let compressed_payload = &compressed_val_bytes[8..];
            zstd::stream::decode_all(compressed_payload).unwrap()
        };

        assert_eq!(decompressed_bytes.len(), 16);

        // Cast uncompressed bytes directly back to i32 values (Zero-Copy)
        let decompressed_buffer = Buffer::from(decompressed_bytes);
        unsafe {
            assert_eq!(decompressed_buffer.get_uncheck::<i32>(0), 1000);
            assert_eq!(decompressed_buffer.get_uncheck::<i32>(1), 2000);
            assert_eq!(decompressed_buffer.get_uncheck::<i32>(2), 3000);
            assert_eq!(decompressed_buffer.get_uncheck::<i32>(3), 4000);
        }
    }

    #[test]
    fn test_record_batch_writer_reader_roundtrip() {
        use crate::array::{BooleanArray, StringArray};

        // 1. Define Diverse Schema: [id: Int32, name: Utf8, active: Boolean]
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
            Field {
                name: "active".to_string(),
                data_type: DataType::Boolean,
                nullable: false,
            },
        ]));

        // 2. Build mock columns
        let id_column: ArrayRef = Arc::new(PrimitiveArray::from(vec![1i32, 2, 3]));
        let name_column: ArrayRef =
            Arc::new(StringArray::from(vec![Some("Alice"), None, Some("Bob")]));
        let active_column: ArrayRef = Arc::new(BooleanArray::from(vec![true, false, true]));

        let original_batch =
            RecordBatch::try_new(schema.clone(), vec![id_column, name_column, active_column])
                .unwrap();

        // 3. Serialize utilizing StreamWriter into a Vec<u8> stream buffer
        let mut stream_buffer = Vec::new();
        {
            let mut writer = Writer::new(&mut stream_buffer);
            writer.write_schema(&schema).unwrap();
            writer.write(&original_batch).unwrap();
        }

        // 4. Deserialize utilizing StreamReader (eagerly consumes Schema)
        let mut reader = Reader::try_new(&stream_buffer[..]).unwrap();

        // Assert Schema matches
        assert_eq!(reader.schema().fields.iter().count(), 3);
        assert_eq!(reader.schema().fields.iter().nth(1).unwrap().name, "name");

        // Read next batch
        let opt_batch = reader.try_next().unwrap();
        assert!(opt_batch.is_some());

        let deserialized_batch = opt_batch.unwrap();
        assert_eq!(deserialized_batch.num_rows(), 3);
        assert_eq!(deserialized_batch.num_columns(), 3);

        // Verify values are preserved exactly
        let id_col = deserialized_batch
            .column(0)
            .as_any()
            .downcast_ref::<PrimitiveArray<i32>>()
            .unwrap();
        assert_eq!(id_col.value(0), 1);
        assert_eq!(id_col.value(1), 2);
        assert_eq!(id_col.value(2), 3);

        let name_col = deserialized_batch
            .column(1)
            .as_any()
            .downcast_ref::<StringArray>()
            .unwrap();
        assert_eq!(name_col.value(0), "Alice");
        assert!(name_col.is_null(1));
        assert_eq!(name_col.value(2), "Bob");

        let active_col = deserialized_batch
            .column(2)
            .as_any()
            .downcast_ref::<BooleanArray>()
            .unwrap();
        assert!(active_col.value(0));
        assert!(!active_col.value(1));
        assert!(active_col.value(2));

        // Next batch should return None (clean EOF)
        assert!(reader.try_next().unwrap().is_none());
    }

    #[test]
    fn test_record_batch_reader_iterator() {
        use crate::array::StringArray;

        // 1. Define Schema: [id: Int32, label: Utf8]
        let schema = Arc::new(Schema::new(vec![
            Field {
                name: "id".to_string(),
                data_type: DataType::Int32,
                nullable: false,
            },
            Field {
                name: "label".to_string(),
                data_type: DataType::Utf8,
                nullable: false,
            },
        ]));

        // 2. Build 2 mock batches
        let id_column1: ArrayRef = Arc::new(PrimitiveArray::from(vec![10i32, 20]));
        let label_column1: ArrayRef = Arc::new(StringArray::from(vec!["A", "B"]));
        let batch1 = RecordBatch::try_new(schema.clone(), vec![id_column1, label_column1]).unwrap();

        let id_column2: ArrayRef = Arc::new(PrimitiveArray::from(vec![30i32, 40]));
        let label_column2: ArrayRef = Arc::new(StringArray::from(vec!["C", "D"]));
        let batch2 = RecordBatch::try_new(schema.clone(), vec![id_column2, label_column2]).unwrap();

        // 3. Serialize both batches into a stream buffer using StreamWriter
        let mut stream_buffer = Vec::new();
        {
            let mut writer = Writer::new(&mut stream_buffer);
            writer.write_schema(&schema).unwrap();
            writer.write(&batch1).unwrap();
            writer.write(&batch2).unwrap();
        }

        // 4. Deserialize utilizing StreamReader as an Iterator!
        let reader = Reader::try_new(&stream_buffer[..]).unwrap();

        // Collect the batches using standard Iterator::collect!
        let batches: Result<Vec<RecordBatch>> = reader.collect();
        let batches = batches.unwrap();

        assert_eq!(batches.len(), 2);

        // Verify batch 1 values
        let b1 = &batches[0];
        assert_eq!(b1.num_rows(), 2);
        let id_col1 = b1
            .column(0)
            .as_any()
            .downcast_ref::<PrimitiveArray<i32>>()
            .unwrap();
        assert_eq!(id_col1.value(0), 10);
        assert_eq!(id_col1.value(1), 20);

        // Verify batch 2 values
        let b2 = &batches[1];
        assert_eq!(b2.num_rows(), 2);
        let id_col2 = b2
            .column(0)
            .as_any()
            .downcast_ref::<PrimitiveArray<i32>>()
            .unwrap();
        assert_eq!(id_col2.value(0), 30);
        assert_eq!(id_col2.value(1), 40);
    }

    #[test]
    fn test_record_batch_statistics_serialization() {
        use crate::array::BooleanArray;

        // 1. Define Schema: [id: Int32, amount: Float64, flag: Boolean]
        let schema = Arc::new(Schema::new(vec![
            Field {
                name: "id".to_string(),
                data_type: DataType::Int32,
                nullable: false,
            },
            Field {
                name: "amount".to_string(),
                data_type: DataType::Float64,
                nullable: false,
            },
            Field {
                name: "flag".to_string(),
                data_type: DataType::Boolean,
                nullable: false,
            },
        ]));

        // 2. Build columns
        let id_col: ArrayRef = Arc::new(PrimitiveArray::from(vec![10i32, -5, 30, 2]));
        let amount_col: ArrayRef =
            Arc::new(PrimitiveArray::from(vec![100.5f64, -20.25, 450.0, 0.0]));
        let flag_col: ArrayRef = Arc::new(BooleanArray::from(vec![true, false, true, false]));

        let batch = RecordBatch::try_new(schema, vec![id_col, amount_col, flag_col]).unwrap();

        // 3. Serialize into buffer
        let mut buffer = Vec::new();
        {
            let mut writer = Writer::new(&mut buffer);
            writer.write(&batch).unwrap();
        }

        // 4. Decode the FlatBuffer headers and verify statistics!
        assert_eq!(&buffer[..4], &[0xFF, 0xFF, 0xFF, 0xFF]);
        let meta_len = i32::from_le_bytes(buffer[4..8].try_into().unwrap()) as usize;
        let fb_msg_slice = &buffer[8..8 + meta_len]; // skip continuation + len
        let fb_msg = unsafe { root_as_message(fb_msg_slice).unwrap() };
        let fb_batch = fb_msg.header_as_fb_record_batch().unwrap();
        let fb_nodes = fb_batch.nodes().unwrap();

        assert_eq!(fb_nodes.len(), 3);

        // --- Column 0 (id: Int32) ---
        let node_id = fb_nodes.get(0);
        assert_eq!(node_id.length(), 4);
        let stat_id = ColumnStat::decode(node_id.stats_byte().bytes()).unwrap();
        assert_eq!(stat_id, ColumnStat::Int64 { min: -5, max: 30 });

        // --- Column 1 (amount: Float64) ---
        let node_amount = fb_nodes.get(1);
        assert_eq!(node_amount.length(), 4);
        let stat_amount = ColumnStat::decode(node_amount.stats_byte().bytes()).unwrap();
        assert_eq!(
            stat_amount,
            ColumnStat::Float64 {
                min: -20.25,
                max: 450.0
            }
        );

        // --- Column 2 (flag: Boolean) ---
        let node_flag = fb_nodes.get(2);
        assert_eq!(node_flag.length(), 4);
        let stat_flag = ColumnStat::decode(node_flag.stats_byte().bytes()).unwrap();
        assert_eq!(
            stat_flag,
            ColumnStat::Bool {
                has_true: true,
                has_false: true
            }
        );
    }

    #[test]
    fn test_writer_write_meta_only_roundtrip() {
        let mut buffer = Vec::new();
        let mut writer = Writer::new(&mut buffer);

        let nodes = vec![
            (100, 0, ColumnStat::Int64 { min: 10, max: 50 }),
            (
                100,
                5,
                ColumnStat::Utf8 {
                    min: "alpha".to_string(),
                    max: "omega".to_string(),
                },
            ),
        ];

        writer.write_meta_node(nodes, 100).unwrap();
        assert!(buffer.len() > 8);

        // Verify MessageReader can decode the MetaOnly message
        let mut msg_reader = MessageReader::new(&buffer[..]);
        let msg = msg_reader.try_next_meta_only().unwrap().unwrap();
        assert_eq!(msg.header_type(), MessageType::MetaOnly as u8);

        let fb_batch = msg.header_as_fb_record_batch().unwrap();
        assert_eq!(fb_batch.length(), 100);

        let fb_nodes = fb_batch.nodes().unwrap();
        assert_eq!(fb_nodes.len(), 2);

        let stat0 = ColumnStat::decode(fb_nodes.get(0).stats_byte().bytes()).unwrap();
        assert_eq!(stat0, ColumnStat::Int64 { min: 10, max: 50 });

        let stat1 = ColumnStat::decode(fb_nodes.get(1).stats_byte().bytes()).unwrap();
        assert_eq!(
            stat1,
            ColumnStat::Utf8 {
                min: "alpha".to_string(),
                max: "omegb".to_string()
            }
        );
    }

    #[test]
    fn test_record_batch_selective_decompression_projection() {
        let schema = Arc::new(Schema::new(vec![
            Field {
                name: "id".to_string(),
                data_type: DataType::Int32,
                nullable: false,
            },
            Field {
                name: "name".to_string(),
                data_type: DataType::Utf8,
                nullable: false,
            },
            Field {
                name: "score".to_string(),
                data_type: DataType::Float64,
                nullable: false,
            },
            Field {
                name: "active".to_string(),
                data_type: DataType::Boolean,
                nullable: false,
            },
        ]));

        let id_col: ArrayRef = Arc::new(PrimitiveArray::from(vec![101i32, 102, 103]));
        let name_col: ArrayRef = Arc::new(StringArray::from(vec!["Alice", "Bob", "Charlie"]));
        let score_col: ArrayRef = Arc::new(PrimitiveArray::from(vec![95.5f64, 88.0, 99.25]));
        let active_col: ArrayRef = Arc::new(BooleanArray::from(vec![true, false, true]));

        let batch = RecordBatch::try_new(
            schema.clone(),
            vec![id_col, name_col, score_col, active_col],
        )
        .unwrap();

        // Serialize into compressed buffer
        let mut buffer = Vec::new();
        {
            let mut writer = Writer::new(&mut buffer);
            writer.write(&batch).unwrap();
        }

        // 1. Project only [name, active] (Indices 1 and 3)
        // Selective decompression must decompress only segments for columns 1 and 3,
        // completely skipping column 0 (Int32) and column 2 (Float64)!
        let mut reader = Reader::try_new_with_schema(&buffer[..], schema.clone())
            .unwrap()
            .with_projection(&[1, 3]);

        let projected_batch = reader.try_next().unwrap().unwrap();
        assert_eq!(projected_batch.num_columns(), 2);
        assert_eq!(projected_batch.num_rows(), 3);
        assert_eq!(projected_batch.schema().fields().len(), 2);
        assert_eq!(projected_batch.schema().fields()[0].name, "name");
        assert_eq!(projected_batch.schema().fields()[1].name, "active");

        let out_name = projected_batch
            .column(0)
            .as_any()
            .downcast_ref::<StringArray>()
            .unwrap();
        assert_eq!(out_name.value(0), "Alice");
        assert_eq!(out_name.value(1), "Bob");
        assert_eq!(out_name.value(2), "Charlie");

        let out_active = projected_batch
            .column(1)
            .as_any()
            .downcast_ref::<BooleanArray>()
            .unwrap();
        assert_eq!(out_active.value(0), true);
        assert_eq!(out_active.value(1), false);
        assert_eq!(out_active.value(2), true);
    }

    #[test]
    fn test_record_batch_projection_single_last_column() {
        let schema = Arc::new(Schema::new(vec![
            Field {
                name: "id".to_string(),
                data_type: DataType::Int32,
                nullable: false,
            },
            Field {
                name: "name".to_string(),
                data_type: DataType::Utf8,
                nullable: false,
            },
            Field {
                name: "score".to_string(),
                data_type: DataType::Float64,
                nullable: false,
            },
        ]));

        let id_col: ArrayRef = Arc::new(PrimitiveArray::from(vec![1i32, 2]));
        let name_col: ArrayRef = Arc::new(StringArray::from(vec!["X", "Y"]));
        let score_col: ArrayRef = Arc::new(PrimitiveArray::from(vec![123.456f64, 789.012]));

        let batch =
            RecordBatch::try_new(schema.clone(), vec![id_col, name_col, score_col]).unwrap();

        let mut buffer = Vec::new();
        {
            let mut writer = Writer::new(&mut buffer);
            writer.write(&batch).unwrap();
        }

        // Project ONLY the last column (index 2: score)
        let mut reader = Reader::try_new_with_schema(&buffer[..], schema.clone())
            .unwrap()
            .with_projection(&[2]);

        let projected_batch = reader.try_next().unwrap().unwrap();
        assert_eq!(projected_batch.num_columns(), 1);
        assert_eq!(projected_batch.schema().fields()[0].name, "score");

        let out_score = projected_batch
            .column(0)
            .as_any()
            .downcast_ref::<PrimitiveArray<f64>>()
            .unwrap();
        assert_eq!(out_score.value(0), 123.456);
        assert_eq!(out_score.value(1), 789.012);
    }

    #[test]
    fn test_record_batch_projection_out_of_order() {
        let schema = Arc::new(Schema::new(vec![
            Field {
                name: "col_a".to_string(),
                data_type: DataType::Int32,
                nullable: false,
            },
            Field {
                name: "col_b".to_string(),
                data_type: DataType::Utf8,
                nullable: false,
            },
            Field {
                name: "col_c".to_string(),
                data_type: DataType::Int64,
                nullable: false,
            },
        ]));

        let a: ArrayRef = Arc::new(PrimitiveArray::from(vec![10i32, 20]));
        let b: ArrayRef = Arc::new(StringArray::from(vec!["first", "second"]));
        let c: ArrayRef = Arc::new(PrimitiveArray::from(vec![1000i64, 2000]));

        let batch = RecordBatch::try_new(schema.clone(), vec![a, b, c]).unwrap();

        let mut buffer = Vec::new();
        {
            let mut writer = Writer::new(&mut buffer);
            writer.write(&batch).unwrap();
        }

        // Project in reverse order [col_c (2), col_a (0)]
        let mut reader = Reader::try_new_with_schema(&buffer[..], schema.clone())
            .unwrap()
            .with_projection(&[2, 0]);

        let projected_batch = reader.try_next().unwrap().unwrap();
        assert_eq!(projected_batch.num_columns(), 2);
        assert_eq!(projected_batch.schema().fields()[0].name, "col_c");
        assert_eq!(projected_batch.schema().fields()[1].name, "col_a");

        let out_c = projected_batch
            .column(0)
            .as_any()
            .downcast_ref::<PrimitiveArray<i64>>()
            .unwrap();
        assert_eq!(out_c.value(0), 1000);
        assert_eq!(out_c.value(1), 2000);

        let out_a = projected_batch
            .column(1)
            .as_any()
            .downcast_ref::<PrimitiveArray<i32>>()
            .unwrap();
        assert_eq!(out_a.value(0), 10);
        assert_eq!(out_a.value(1), 20);
    }

    #[test]
    fn test_record_batch_projection_out_of_bounds() {
        let schema = Arc::new(Schema::new(vec![Field {
            name: "id".to_string(),
            data_type: DataType::Int32,
            nullable: false,
        }]));

        let batch = RecordBatch::try_new(
            schema.clone(),
            vec![Arc::new(PrimitiveArray::from(vec![1i32]))],
        )
        .unwrap();

        let mut buffer = Vec::new();
        {
            let mut writer = Writer::new(&mut buffer);
            writer.write(&batch).unwrap();
        }

        // Project out-of-bounds column index 5
        let mut reader = Reader::try_new_with_schema(&buffer[..], schema.clone())
            .unwrap()
            .with_projection(&[5]);

        let res = reader.try_next();
        match res {
            Err(e) => assert!(e.to_string().contains("out of bounds")),
            Ok(_) => panic!("Expected error for out-of-bounds projection"),
        }
    }
}
