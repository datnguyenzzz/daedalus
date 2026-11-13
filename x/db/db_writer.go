package db

func (d *DB) Delete(key []byte) error {
	panic("implement Delete, and read about Range Tombstone deletion article")
}

func (d *DB) Set(key, value []byte) error {
	b := newBatch(d, true)
	if err := b.Set(key, value); err != nil {
		return err
	}

	if err := d.apply(b); err != nil {
		return err
	}

	return b.Close()
}

func (d *DB) apply(b *Batch) error {
	if b.committing.Load() {
		panic("batch is already commiting")
	}

	if b.applied.Load() {
		panic("batch is applied")
	}

	b.committing.Store(true)
	return d.commit.Commit(b)
}
