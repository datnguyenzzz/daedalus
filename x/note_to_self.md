# Project tracker 

## H2 - 2026 

On-working: Motivation, I would like to learn Rust :)  
- [ ] P0: Write a query engine from Rust 
- [ ] P1: Design the DB public APIs and skeleton the main flow
- [ ] P1:
  - [ ] Implement Size-Tier compaction. Reference for the improvement: 
    - [ ] https://nivdayan.github.io/dostoevsky.pdf
- [ ] P2 - Open DB from the leftover state, replay from WALs, stable versions, ...
- [ ] P2 - Emit metrics for monitoring

## H1 - 2026

- Implement go-fs with the basic file operations
  - [x] P0: On local disk
  - [ ] P0: Remote storage (S3, ...) -- use [ministack](https://github.com/ministackorg/ministack)
- Add an exhaustive functional tests (writer --> iterator) for the sstable
  - [x] P0: On-local disk
  - [ ] P0: Remote storage (S3, ...)
- [ ] P1: Add benchmark tests for Iterator + Writer
- [x] P0: Implement lock-free concurrent ART and benchmark against the current sequential adaptive radix tree
- [x] P0: Implement Clock-based eviction policy and benchmark against the LRU policy for the go-block-cache
-   [x] P0: Implement Sharding on the go-block-cache
- [x] P0: Implement columnar block format in the go-sstable
  - [x] P0: Research on the key prefix compression , similarly performed by rowblk
- [x] P1: Learn about the MVCC and how to apply it on the go-sstables
  - MVCC provides "an isolation level" called "snapshot isolation"
  - It does this by storing historical versions of key/value pairs. The version number is simply a number that's incremented for every new transaction
  - Each transaction has its own unique version number. When it writes a key/value pair it appends its version number to the key
- [ ] P2: Write Github workflow to schedule a microbenchmark task to each components
- [ ] P0 - **ongoing**: Design the DB public APIs and skeleton the main flow
  - [x] Implement [record format](https://github.com/facebook/rocksdb/wiki/Write-Ahead-Log-File-Format#record-format) to a common `lib`
- [x] P0 - (Re)implement WAL
  - https://github.com/facebook/rocksdb/wiki/Track-WAL-in-MANIFEST
  - https://github.com/facebook/rocksdb/wiki/Write-Ahead-Log-%28WAL%29
  - [ ] P1 - Recycable WAL files
- [ ] P0
  - [ ] Implement Tier + Leveled hybrid compaction. Reference: https://nivdayan.github.io/dostoevsky.pdf
  - [ ] To read, Fragmented LSM: https://www.cs.utexas.edu/~vijay/papers/sosp17-pebblesdb.pdf

- [ ] P0 - Open DB from the leftover state, replay from WALs, stable versions, ...
- [ ] P0 - Emit metrics for monitoring

## H2 - 2025

- [x] Finished implementing writer + iterator for the sstable  
- [ ] Implement go-fs with the basic file operations
  - [x] P0: In-mem
  - [ ] P1: On local disk
  - [ ] P2: Remote storage
- [x] P0: Wire the go-sstable/writer + reader to use the go-fs
- [ ] Add an exhaustive functional tests (writer --> iterator) for the sstable
  - [x] P0: In-mem
  - [ ] P1: On-local disk
- [ ] P2: Add benchmark tests for Iterator + Writer
- [x] P0: Refactor go-wal to use go-fs
- [ ] P1: Implement lock-free Skip list and benchmark against the adaptive radix tree for the MemTable
- [ ] P1: Implement Clock-based eviction policy and benchmark against the LRU policy for the go-block-cache