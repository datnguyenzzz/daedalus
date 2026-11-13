package go_fs

type wrapperFileReadable struct {
	File

	fs   FS
	path string
}

func (f *wrapperFileReadable) Size() uint64 {
	info, err := f.fs.Stat(f.path)
	if err != nil {
		return 0
	}
	return uint64(info.Size())
}
