### 1. Row Oriented BlockedBasedTable Format 

_Inspired by [LevelDB file format](https://github.com/google/leveldb/blob/main/doc/table_format.md)_

#### Data Block Format 

A Data Block is consist of one or more key/value entries and a block trailer. Block entry shares key prefix with its preceding 
key until a restart point reached. A block should contains at least one restart point. First restart point are always zero.

For example, if two adjacent keys are `"deck"` and `"dock"`, then the second key would be encoded as 
`{1,"ock"}`. The shared prefix length is varint encoded. The remainder string and the value are encoded as a varint-encoded length 
followed by the literal contents. To continue the example, suppose that the key `"dock"` mapped to the value
`"v2"`. The encoded key/value entry would be: `"\x01\x03\x02dockv2"`.

Every block has a restart interval I. Every I'th key/value entry in that block is called a restart point, and shares no key prefix with the previous entry.
Continuing the example above, if the key after `"dock"` was `"duck"`, but was part of a restart point, 
then that key would be encoded as `{0, "duck"}` instead of `{1, "uck"}`.

Illustration of a typical data block:

```
  + restart point                  + restart point (depends on restart interval)
 /                                /
+---------------+---------------+---------------+---------------+---------+
| block entry 1 | block entry 2 |      ...      | block entry n | trailer |
+---------------+---------------+---------------+---------------+---------+
```

Key/Value Entry layout:
```
          +---- key len ----+
         /                   \
+-------+---------+-----------+---------+--------------------+--------------+----------------+---------+
| shared (varint) | not shared (varint) | value len (varint) | key (varlen) | value (varlen) | trailer |
+-----------------+---------------------+--------------------+--------------+----------------+---------+

Block entry shares key prefix with its preceding key:
Conditions:
    restart_interval=2
    entry one  : key=deck,value=v1
    entry two  : key=dock,value=v2
    entry three: key=duck,value=v3
The entries will be encoded as follow:

  + restart point (offset=0)                                                 + restart point (offset=16)
 /                                                                          /
+-----+-----+-----+----------+--------+-----+-----+-----+---------+--------+-----+-----+-----+----------+--------+---------+
|  0  |  4  |  2  |  "deck"  |  "v1"  |  1  |  3  |  2  |  "ock"  |  "v2"  |  0  |  4  |  2  |  "duck"  |  "v3"  | trailer | 
+-----+-----+-----+----------+--------+-----+-----+-----+---------+--------+-----+-----+-----+----------+--------+---------+
 \                                   / \                                  / \                                   /
  +----------- entry one -----------+   +----------- entry two ----------+   +---------- entry three ----------+
```
```
The block trailer will contains two restart points:

+------------+-----------+--------+
|     0      |    16     |   2    |
+------------+-----------+---+----+
 \                      /     \
  +-- restart points --+       + restart points length

  +-- 4-bytes --+
 /               \
+-----------------+-----------------+-----------------+------------------------------+
| restart point 1 |       ....      | restart point n | restart points len (4-bytes) |
+-----------------+-----------------+-----------------+------------------------------+
```

#### c. Index block 

An index block is a block with N key/value entries, and they share the similar format
with the data block. It helps to faster locate the data block that might have a requested key

The index block `i'th` value is the encoded block handle of the `i'th` data block.
And the index block `i'th` key is a string `>=` last key in that data block 
and `<` the first key in the successive data block. The index block restart 
interval is `1`: every entry is a restart point. The index is added every time 
the data block is flushed to storage (when it exceeds a certain size).

By default, we use a two-level index. It consists of a sequence of lower-level 
index blocks with block handles for data blocks followed by a single top-level 
index block with block handles for the lower-level index blocks. Value of the 
top-level index block is the encoded block handle of the lower-level blocks,

```
2 Level Block format when stored in the stable storage:
[index block - 1st level]
[index block - 1st level]
...
[index block - 1st level]
[index block - 2nd level]


1st level index:
+-----------+---------------------------+
|    key    |           value           |
+-----------+---------------------------+
| index_key | data block handle encoded |
+-----------+---------------------------+

2nd level index:
+-----------+--------------------------------------+
|    key    |                value                 |
+-----------+--------------------------------------+
| index_key | 1st-level index block handle encoded |
+-----------+--------------------------------------+
```

```
Illustration of the layout 

Data Block Section

<- Block threshold ><- Block threshold ->
+---------+---------+---------+---------+---------+---------+---------+
| entry 1 | entry 2 |   ...   |   ...   |   ...   | entry N | trailer |
+---------+---------+---------+---------+---------+---------+---------+
                   /                   /                 
  +---------------+   +---------------+
  |   1st-level   |   |   1st-level   |
  |  index key 1  |   |  index key 2  |
  +---------------+   +---------------+
  | block handle  |   | block handle  |
  | start of entry|   | start of entry|   
  +---------------+   +---------------+
  
1st level Index Section

<---     Block threshold         --->
+-----------------+-----------------+--------+-----------------+
| 1st-level index | 1st-level index |   ...  | 1st-level index |
|     entry 1     |     entry 2     |        |     entry K     |
+-----------------+-----------------+--------+-----------------+
                          |                            |
                  +---------------+            +---------------+
                  |   2nd-level   |            |   2nd-level   |
                  |  index key 1  |            |  index key 2  |
                  +---------------+            +---------------+
                  | block handle  |            | block handle  |
                  | start of entry|            | start of entry|
                  +---------------+            +---------------+

2nd level Index Section
+-----------------+-----------------+--------+-----------------+
| 2nd-level index | 2nd-level index |   ...  | 2nd-level index |
|     entry 1     |     entry 2     |        |     entry K     |
+-----------------+-----------------+--------+-----------------+
 \
   +----------+
   |  Footer  |
   +----------+
   |  Offset  |
   +----------+
```