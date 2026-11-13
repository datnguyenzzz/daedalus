### What is it

Package manages reading and writing sequences of records. Each `record` is a stream of bytes that completes before the next record starts.

> Neither Readers or Writers are safe to use concurrently.

Example code:

```
func read(r io.Reader) ([]string, error) {
    var ss []string
    records := record.NewReader(r)
    for {
        rec, err := records.Next()
        if err == io.EOF {
            break
        }
        s, err := io.ReadAll(rec)
        ss = append(ss, string(s))
    }
    return ss, nil
}

func write(w io.Writer, ss []string) error {
    records := record.NewWriter(w)
    for _, s := range ss {
        rec, err := records.Next()
        if err != nil {
            return err
        }
        if _, err := rec.Write([]byte(s)), err != nil {
            return err
        }
    }
    return records.Close()
}
```

### Data layout

- https://github.com/facebook/rocksdb/wiki/Write-Ahead-Log-File-Format#log-file-format

The wire format is that the stream is divided into 32KiB blocks, and each block contains a number of tightly packed chunks. Chunks cannot cross block boundaries. The last block may be shorter than 32 KiB. Any unused bytes (padding) in a block must be zero.

```
       +-----+-------------+--+----+----------+------+-- ... ----+
 File  | r0  |        r1   |P | r2 |    r3    |  r4  |           |
       +-----+-------------+--+----+----------+------+-- ... ----+
       <---  BlockSize ------>|<--  BlockSize ------>|

  rn = variable size records
  P = Padding
```

A record maps to one or more chunks. A (recyclable) chunk has a format as below 

```
	+----------+-----------+-----------+----------------+--- ... ---+
	| CRC (4B) | Size (2B) | Type (1B) | Log number (4B)| Payload   |
	+----------+-----------+-----------+----------------+--- ... ---+

    CRC is computed over the type and payload

    Log number allows reuse (recycling) of log files which can provide 
    significantly better performance when syncing frequently as it avoids 
    needing to update the file metadata

    Size is the length of the payload in bytes

    Type is the chunk type
    There are 4 chunk types: whether the chunk is the full record, or the
    first, middle or last chunk of a multi-chunk record. A multi-chunk record
    has one first chunk, zero or more middle chunks, and one last chunk.
```
