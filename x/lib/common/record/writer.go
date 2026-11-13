package record

import (
	"encoding/binary"
	"errors"
	"io"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

const (
	auxilaryByte = 0x1
)

type flusher interface {
	Flush() error
}

type Writer struct {
	w          io.Writer
	checksumer nogodb_common.IChecksum
	// seq is the sequence number of the current record.
	seq int
	// f is w as a flusher.
	f flusher
	// buf[begin:end] is the bytes that will become the current chunk.
	// The low bound, begin, includes the chunk header.
	begin, end int
	// buf[:written] has already been written to w.
	// written is zero unless Flush has been called.
	written int
	// baseOffset is the base offset in w at which writing started. If
	// w implements io.Seeker, it's relative to the start of w, 0 otherwise.
	baseOffset int64
	// blockNumber is the zero based block number currently held in buf.
	blockNumber int64
	// lastRecordOffset is the offset in w where the last record was
	// written (including the chunk header). It is a relative offset to
	// baseOffset, thus the absolute offset of the last record is
	// baseOffset + lastRecordOffset.
	lastRecordOffset int64
	// first is whether the current chunk is the first chunk of the record.
	first bool
	// pending is whether a chunk is buffered but not yet written.
	pending bool

	err error
	buf [BlockSize]byte
}

func NewWriter(w io.Writer) *Writer {
	f, _ := w.(flusher)

	var o int64
	if s, ok := w.(io.Seeker); ok {
		var err error
		o, err = s.Seek(0, io.SeekCurrent)
		if err != nil {
			o = 0
		}
	}

	return &Writer{
		w:                w,
		f:                f,
		checksumer:       nogodb_common.NewChecksumer(nogodb_common.CRC32Checksum),
		baseOffset:       o,
		lastRecordOffset: -1,
	}
}

func (w *Writer) fillHeader(isLast bool) {
	if w.begin+headerSize > w.end || w.end > BlockSize {
		panic("writer has a bad state")
	}

	if isLast {
		if w.first {
			w.buf[w.begin+6] = fullChunkEncoding
		} else {
			w.buf[w.begin+6] = lastChunkEncoding
		}
	} else {
		if w.first {
			w.buf[w.begin+6] = firstChunkEncoding
		} else {
			w.buf[w.begin+6] = middleChunkEncoding
		}
	}

	binary.LittleEndian.PutUint32(w.buf[w.begin:w.begin+4], w.checksumer.Checksum(w.buf[w.begin+6:w.end], auxilaryByte))
	binary.LittleEndian.PutUint16(w.buf[w.begin+4:w.begin+6], uint16(w.end-w.begin-headerSize))
}

// writeBlock writes a whole block to the underlying writer, only triggered
// when the buffer is full
func (w *Writer) writeBlock() {
	_, w.err = w.w.Write(w.buf[w.written:])
	w.begin = 0
	w.end = headerSize
	w.written = 0
	w.blockNumber++
}

// writeBuffered finishes the current record and writes the current buffer
// , even if it isn't full block yet, to the underlying writer
func (w *Writer) writeBuffered() {
	if w.err != nil {
		return
	}

	if w.pending {
		w.fillHeader(true)
		w.pending = false
	}

	_, w.err = w.w.Write(w.buf[w.written:w.end])
	w.written = w.end
}

// Next returns a writer for the next record. The writer returned becomes stale
// after the next Close, Flush or Next call, and should no longer be used.
func (w *Writer) Next() (io.Writer, error) {
	w.seq++
	if w.err != nil {
		return nil, w.err
	}

	// there are remant bytes from last block in the buf
	// have not yet written and doesn't have header bytes
	if w.pending {
		w.fillHeader(true)
	}

	w.begin = w.end
	w.end = w.end + headerSize

	if w.end > BlockSize {
		// it is overflow, need to write with padding
		clear(w.buf[w.begin:])
		w.writeBlock()
		if w.err != nil {
			return nil, w.err
		}
	}

	w.lastRecordOffset = w.baseOffset + w.blockNumber*BlockSize + int64(w.begin)
	w.pending = true
	w.first = true

	return &bufferedWriter{w, w.seq}, nil
}

// Flush finishes the current record, writes to the underlying writer, and
// flushes it if that writer implements interface{ Flush() error }.
func (w *Writer) Flush() error {
	w.seq++
	w.writeBuffered()
	if w.err != nil {
		return w.err
	}

	if w.f != nil {
		w.err = w.f.Flush()
		return w.err
	}

	return nil
}

// Close finishes the current record and closes the writer.
func (w *Writer) Close() error {
	w.seq++
	w.writeBuffered()
	if w.err != nil {
		return w.err
	}
	w.err = errors.New("closed writer")
	return nil
}

func (w *Writer) Size() int64 {
	if w == nil {
		return 0
	}

	return w.blockNumber*BlockSize + int64(w.end)
}

type bufferedWriter struct {
	w   *Writer
	seq int
}

func (x *bufferedWriter) Write(p []byte) (n int, err error) {
	w := x.w
	if w.seq != x.seq {
		return 0, errors.New("record: stale writer")
	}

	if w.err != nil {
		return 0, w.err
	}

	n0 := len(p)
	for len(p) > 0 {
		// block is full, write it
		if w.end == BlockSize {
			w.fillHeader(false)
			w.writeBlock()
			if w.err != nil {
				return 0, w.err
			}
			w.first = false
		}

		n := copy(w.buf[w.end:], p)
		w.end += n
		p = p[n:]
	}

	return n0, nil
}
