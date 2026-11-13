package record

import (
	"encoding/binary"
	"errors"
	"io"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

var (
	// ErrInvalidChunk is returned if a chunk is encountered with an invalid
	// header, length, or checksum.
	ErrInvalidChunk = errors.New("record with invalid chunk")

	// ErrUnexpectedEOF is returned if a log file ends unexpectedly. It
	// indicates the unexpected end of the log file while reading or an
	// in-progress record envelope itself
	ErrUnexpectedEOF = errors.New("record with unexpected EOF")

	// ErrZeroedChunk is returned if a chunk is encountered that is zeroed.
	ErrZeroedChunk = errors.New("record with zeroed chunk")
)

const (
	invalidChunkEncoding byte = iota
	fullChunkEncoding
	firstChunkEncoding
	middleChunkEncoding
	lastChunkEncoding
)

const (
	BlockSize  = 32 * 1024
	headerSize = 7
)

type Reader struct {
	r          io.Reader
	checksumer nogodb_common.IChecksum
	// blockNum is the block number (0-indexed) currently held in buf
	blockNum int64
	// seq is the sequence number of the current record
	seq int
	// buf[begin:end] is the unread record of the current chunk's payload. The
	// low bound, begin, excludes the chunk header.
	begin, end int
	// n is the number of bytes of buf that are valid. Once reading has started,
	// only the final block can have n < blockSize.
	n int
	// last is whether the current chunk is the last chunk of the record.
	last bool
	// err is any accumulated error.
	err error

	// invalidOffset is the first encountered chunk offset found during nextChunk()
	// that had garbage values.
	invalidOffset uint64

	buf [BlockSize]byte
}

// nextChunk sets r.buf[r.begin:r.end] to hold the next chunk's payload,
// and reading the next block into the buffer if necessary.
func (r *Reader) nextChunk(wantFirst bool) error {
	for {
		if r.end+headerSize <= r.n {
			// Slide the [r.begin:r.end] window to the next record within r.buf[]
			checksum := binary.LittleEndian.Uint32(r.buf[r.end : r.end+4])
			length := binary.LittleEndian.Uint16(r.buf[r.end+4 : r.end+6])
			chunkEncoding := r.buf[r.end+6]

			if chunkEncoding > lastChunkEncoding {
				r.invalidOffset = uint64(r.blockNum)*BlockSize + uint64(r.begin)
				return ErrInvalidChunk
			}

			if checksum == 0 && length == 0 && chunkEncoding == invalidChunkEncoding {
				r.invalidOffset = uint64(r.blockNum)*BlockSize + uint64(r.begin)
				return ErrZeroedChunk
			}

			r.begin = r.end + headerSize
			r.end = r.begin + int(length)

			if r.end > r.n {
				// The chunk straddles a 32KB boundary (or the end of file).
				r.invalidOffset = uint64(r.blockNum)*BlockSize + uint64(r.begin)
				return ErrInvalidChunk
			}

			data := r.buf[r.begin-headerSize+6 : r.end]
			if checksum != r.checksumer.Checksum(data, auxilaryByte) {
				r.invalidOffset = uint64(r.blockNum)*BlockSize + uint64(r.begin)
				return ErrInvalidChunk
			}

			if wantFirst {
				if chunkEncoding != fullChunkEncoding && chunkEncoding != firstChunkEncoding {
					// skip the orphan chunk
					continue
				}
			}

			r.last = chunkEncoding == fullChunkEncoding || chunkEncoding == lastChunkEncoding
			return nil
		}

		// reading next block to r.buf[:]

		if r.n < BlockSize && r.n > 0 {
			// already at the last block in a file
			if !wantFirst || r.end != r.n {
				// This can happen if the previous instance of the log ended with a
				// partial block at the same blockNum as the new log but extended
				// beyond the partial block of the new log.
				r.invalidOffset = uint64(r.blockNum)*BlockSize + uint64(r.begin)
				return ErrInvalidChunk
			}
			return io.EOF
		}

		n, err := io.ReadFull(r.r, r.buf[:])
		if err != nil && err != io.ErrUnexpectedEOF {
			if err == io.EOF && !wantFirst {
				r.invalidOffset = uint64(r.blockNum)*BlockSize + uint64(r.begin)
				return ErrUnexpectedEOF
			}

			return err
		}

		r.begin, r.end, r.n = 0, 0, n
		r.blockNum++
	}
}

// NewReader returns a new reader. The log number in those records must
// match the specified logNum
func NewReader(r io.Reader) *Reader {
	return &Reader{
		r:          r,
		checksumer: nogodb_common.NewChecksumer(nogodb_common.CRC32Checksum),
		blockNum:   -1,
	}
}

func (r *Reader) Next() (io.Reader, error) {
	r.seq++
	if r.err != nil {
		return nil, r.err
	}

	r.begin = r.end
	r.err = r.nextChunk(true)
	if r.err != nil {
		return nil, r.err
	}

	return &bufferedReader{r, r.seq}, nil
}

func (r *Reader) Offset() int64 {
	if r.blockNum < 0 {
		return 0
	}

	return r.blockNum*BlockSize + int64(r.end)
}

type bufferedReader struct {
	r   *Reader
	seq int
}

func (b *bufferedReader) Read(p []byte) (int, error) {
	r := b.r
	if r.seq != b.seq {
		return 0, errors.New("stale reader")
	}
	if r.err != nil {
		return 0, r.err
	}
	for r.begin == r.end {
		if r.last { // last chunk
			return 0, io.EOF
		}

		r.err = r.nextChunk(false)
		if r.err != nil {
			return 0, r.err
		}
	}

	n := copy(p, r.buf[r.begin:r.end])
	r.begin += n
	return n, nil
}
