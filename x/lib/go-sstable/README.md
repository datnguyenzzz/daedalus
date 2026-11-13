# A Sorted String Table (SST) Implementation in Golang

Tables are exclusively opened for reading or created for writing, but never both simultaneously.
During the initialization process for creating a table, clients will have several options to choose 
from based on their specific use cases.

## Table Format 

```
<beginning_of_file>
[data block 1]
[data block 2]
...
[data block N]
[meta block 1: filter block]                  
[meta block 2: index block]     
...
[meta block K: future extended block]  
[metaindex block]
[Footer]                               (fixed size; starts at file_size - sizeof(Footer))
<end_of_file>
```

The file contains internal pointers, called `BlockHandles`, containing the following information:
```
offset: int64
size:   int64
```

A `metaindex` block contains one entry for every meta block, where the key is the name of the meta block 
and the value is a BlockHandle pointing to that meta block. 
```
filterKey        : BlockHandle(FilterBlock)
2ndLevelIndexKey : BlockHandle(2ndLevelIndex)
```

Each block consists of some data and a 5 byte trailer: a 1 byte block type and a
4 byte checksum. The checksum is computed over the compressed data and the first
byte of the trailer (i.e. the block type), and is serialized as little-endian.
The block type gives the per-block compression used; each block is compressed
independently

Illustration of a physical block trailer:
```
+---------------------------+-------------------+
| compression type (1-byte) | checksum (4-byte) |
+---------------------------+-------------------+

The checksum is a CRC-32 computed using Castagnoli's polynomial. Compression 
type also included in the checksum.
```

Footer formats. Note that much of the existing Footer parsing code assumes that the version (for non-legacy formats) 
and magic number are at the end.

```
metaindex handle (varint64 offset, varint64 size)
index handle     (varint64 offset, varint64 size)
<padding> to make the total size 2 * BlockHandle::kMaxEncodedLength + 1
checksum: CRC over Footer data (4 bytes)
Footer version (4 bytes)
table_magic_number (8 bytes)
```

## Block Format

At the moment, NogoDB supports 2 formats Row-oriented and Columnar-oriented