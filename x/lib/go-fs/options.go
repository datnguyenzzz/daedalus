package go_fs

type VfsOption func(*vfsProvider)

func WithDirName(dirName string) VfsOption {
	return func(vp *vfsProvider) {
		vp.dirName = dirName
	}
}

func WithBytesPerSync(bytesPerSync int64) VfsOption {
	return func(vp *vfsProvider) {
		vp.bytesPerSync = bytesPerSync
	}
}

func WithFS(fs FS) VfsOption {
	return func(vp *vfsProvider) {
		vp.fs = fs
	}
}
