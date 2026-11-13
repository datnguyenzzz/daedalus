## Columnar block format 

Referrence: https://www.pdl.cmu.edu/PDL-FTP/Database/pax.pdf

### Block layout
Every columnar block begins with a header describing the structure and schema of the block. Then columns values are encoded in sequence. The block ends with a single padding byte.

The block header begins with:
- Version number (1 byte)
- The number of columns in the block (2 bytes)
- The number of rows in the block (4 bytes)

Then follows a column-header for each column. Each column header encodes the data type (1 byte) and the offset into the block where the column data begins (4 bytes). Depends on each data type we have different way to endcode the column data. 

```
+-----------+----------------+---------------------+
| Vers (1B) | # columns (2B) | # of rows (4B).     |
+-----------+----------------+---------------------+
| DataType (1B) | Page offset points to col 0 (4B) |              
+-----------+--------------------------------------+
| DataType (1B) | Page offset points to col 1 (4B) |            
+-----------+--------------------------------------+
| ...	    | ...                                  |
+-----------+--------------------------------------+
| Type (1B) | Page offset points to col N-1 (4B)   |            
+-----------+--------------------------------------+
|  column 0 data                                ...
+--------------------------------------------------+
|  column 1 data                                ...
+--------------------------------------------------+
| ...
+--------------------------------------------------+
|  column n-1 data                              ...
+-------------+------------------------------------+
| Unused (1B) |
+-------------+
```

The trailing padding byte exists to allow columns to represent the end of
column data using a pointer to the byte after the end of the column. The
padding byte ensures that the pointer will always fall within the allocated
memory. Without the padding byte, such a pointer would be invalid according
to Go's pointer rules.

Columns data order 

| No | Type                            | Data type               |
| -- | ------------------------------- | ----------------------- |
| 0  | InternalKey.UserKey             | Prefix-compressed bytes |
| 1  | InternalKey.UserKey suffix MVCC | Bytes                   |
| 2  | Prefix changed position.        | Bitmap                  |
| 3  | InternalKey.Trailer             | Uint                    |
| 4  | Value                           | Bytes                   |
| .. | ...                             | ...                     |

For example we have 3 entries:

```json
[
    {"key": "apple.123.1", "value": "green"},
    {"key": "banana.456.2", "value": "yellow"},
    {"key": "rashberry.789.3", "value": "red"}
]
```

Then the data layout after encoding will be following (for illustration purpose only)

```go
1   v1 | 4 | 3
2   0 | 6
3   1 | 7
4   2 | 8
5   3 | 9
6   apple|banana|rashberry 
7   123|456|789 
8   1|2|3 
9   green|yellow|yellow     
```

### Encoding detail methods 
1. Prefix-compressed bytes data type 

Prefix-compressed encodes a list of bytes that are lexicographically ordered, with compressing their prefix (similar idea with how do we encode the row_block keys).
Note that the key are not unique, because in the MVCC model, a same key will have multiple version, they will be distincted by the suffix ID number (auto-increased id, timestamp, ...)

Let's review how does the encoder work in the below example, where we want to encode a list of 16 keys 

```
     0123456789
0    aaaaaaa
1    aaaaaab
2    aaaaabb
3    aaaaaba
4    aabbbc
5    aabbbcc
6    aabbccd
7    aabbce
8    aabbe
9    aabbe
10   aabbef
11   aabbee
12   aacde
13   aacdf
14   aacdf
15   aacdgg
```

The table below shows the encoded for these 16 keys when using a bundle size of 4 (which results in 4 bundles in toal)

There are encoded into 21 slices: 1 block prefix, 4 bundle prefixes, and 16 suffixes. The first slice in the table is the block prefix that is shared by all keys in the block. The first slice in each bundle is the bundle prefix
which is shared by all keys in the bundle.

```text
 idx | offset | data 
-----+--------+------
                aa 
                ..aaa
    0           .....aa
    1           .....ab
    2           .....bb
    3           .....ba
                ..bb
    4           ....bc
    5           ....bcc
    6           ....ccd
    7           ....ce
                ..bbe
    8           .....
    9           .....
    10          .....f
    11          .....e
                ..cd
    12          ....e
    13          ....f
    14          ....f
    15          ....gg                
-----+--------+------ 
```

Physical representation

```text
+------------------+---------------------------------------------------------------------------------+
| Bundle size (1B) | Raw bytes encoding (1 block prefix, (n+bs-1)/(bs) bundle prefixes, n suffixes ) | 
+------------------+---------------------------------------------------------------------------------+
```


2. Bytes 

Layout
```
+-----------------------------------------------------+-------+-------+-----
| Uint Encoded of [offset(buf_1), offset(buf_2), ...] | buf_1 | buf_2 | ...
+-----------------------------------------------------+-------+-------+-----
```

3. Uint column encoder uses for storing any unsigned integer with 8, 16, 32, 64 bit

- The low bits indicate how many bytes per integer are used, with allowed values 0, 1, 2, 4, or 8.

- A base (8-byte) value is encoded separately and each encoded value is a delta from that base.

Layout:
```
+----------+----------------+--------------+--------------+--------------+
| Type(1B) | Min Value (8B) | Delta 1 (xB) | Delta 2 (xB) | .....
+----------+----------------+--------------+--------------+--------------+
```

4. Bitmap uses for storing numbers that are within range [0,N-1]

- Bitmap is a bitmap structure built on a []uint64. The bitmap is encoded into an 8-byte aligned array of 64-bit words which is (nRows+63)/64 words in length.

- A summary bitmap is stored after the primary bitmap in which each bit in the summary bitmap corresponds to 1 word in the primary bitmap. A bit is set in the summary bitmap if the corresponding word in the primary bitmap is
non-zero.

Example of the bitmap that holds [0..N-1]

```
masks: masks[i][j] = 1 means number = [64*i + j] exists
        <--64bit--> 
     0: 10101...101
     1: 10101...101
        ...
[N/64]: 10101...101

index_masks: index_masks[i][j] = 1 means masks[i*64+j] <> 0
           <--64bit-->
        0: 10101...101
        1: 10101...101
           ...
[N/64/64]: 10101...101
```