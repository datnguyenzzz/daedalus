package go_fs

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

func Test_Create_And_Open(t *testing.T) {
	writeFunc := func(writable Writable, b []byte) error {
		n, err := writable.Write(b)
		assert.NoError(t, err)
		assert.Equal(t, len(b), n, "Can not write fully")
		return err
	}

	type param struct {
		name             string
		fileCountPerType map[nogodb_common.ObjectType]int
		fileSize         int
		async            bool
	}

	dummyByte := []byte{0x3A, 0x29}

	cases := []param{
		{
			name:  "async",
			async: true,
			fileCountPerType: map[nogodb_common.ObjectType]int{
				nogodb_common.TypeManifest: 1,
				nogodb_common.TypeTable:    3,
				nogodb_common.TypeWAL:      2,
			},
			fileSize: 5,
		},
		{
			name:  "sync",
			async: true,
			fileCountPerType: map[nogodb_common.ObjectType]int{
				nogodb_common.TypeManifest: 1,
				nogodb_common.TypeTable:    3,
				nogodb_common.TypeWAL:      2,
			},
			fileSize: 5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage := NewInmemStorage()
			writers := make(map[FileDesc]Writable)
			// create files
			for fileType, num := range tc.fileCountPerType {
				for i := range num {
					writer, fd, err := storage.Create(fileType, nogodb_common.DiskfileNum(i))
					assert.NoError(t, err, "can not create file")
					writers[fd] = writer
				}
			}

			// write to file
			eg := errgroup.Group{}
			if tc.async {
				eg.SetLimit(10)
			} else {
				eg.SetLimit(1)
			}

			for fileType, num := range tc.fileCountPerType {
				for i := range num {
					fd, err := storage.LookUp(fileType, nogodb_common.DiskfileNum(i))
					assert.NoError(t, err, "can not look up file")
					eg.Go(func() error {
						writer, ok := writers[fd]
						assert.True(t, ok, fmt.Sprintf("can not find writer for %#v", fd))
						err := writeFunc(writer, bytes.Repeat(dummyByte, tc.fileSize))
						assert.NoError(t, err)
						if err != nil {
							return err
						}

						err = writer.Finish()
						assert.NoError(t, err, "can not close writer")
						if err != nil {
							return err
						}

						return nil
					})
				}
			}

			err := eg.Wait()
			assert.NoError(t, err)

			// assert file content
			for fileType, num := range tc.fileCountPerType {
				for i := range num {
					reader, fd, err := storage.Open(fileType, nogodb_common.DiskfileNum(i))
					assert.NoError(t, err, "can not open file")
					_, ok := writers[fd]
					assert.True(t, ok, fmt.Sprintf("can not find writer for %#v", fd))

					var buf bytes.Buffer
					_, err = buf.ReadFrom(reader)
					assert.NoError(t, err)

					expectedBytes := bytes.Repeat(dummyByte, tc.fileSize)
					assert.Equal(t, expectedBytes, buf.Bytes())
				}
			}
		})
	}
}

func Test_Read_During_Write(t *testing.T) {
	storage := NewInmemStorage()
	fid := 1
	writer, _, err := storage.Create(nogodb_common.TypeWAL, nogodb_common.DiskfileNum(fid))
	require.NoError(t, err)
	reader, _, err := storage.Open(nogodb_common.TypeWAL, nogodb_common.DiskfileNum(fid))
	require.NoError(t, err)
	// first write
	n, err := writer.Write([]byte{1, 2, 3, 4, 5})
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	res := make([]byte, 5)
	n, err = reader.ReadAt(res, 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Zero(t, bytes.Compare(res, []byte{1, 2, 3, 4, 5}))

	// second write
	n, err = writer.Write([]byte{6, 7, 8, 9, 10})
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	res = make([]byte, 5)
	n, err = reader.ReadAt(res, 5)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Zero(t, bytes.Compare(res, []byte{6, 7, 8, 9, 10}))

	err = writer.Finish()
	assert.NoError(t, err)
	err = reader.Close()
	assert.NoError(t, err)
}
