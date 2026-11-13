package prefixbytescodex

// GetBundleFromRow returns the bundle index for a given row
func GetBundleFromRow(rows uint32, bundleSize byte) uint32 {
	return rows / uint32(bundleSize)
}

// GetBundlePrefixPos returns the offset of the bundle prefix for a given row
func GetBundleFromOffset(offset uint32, bundleSize byte) uint32 {
	return (offset - 1) / (uint32(bundleSize) + 1)
}

// GetBundlePrefixPos returns the offset of the bundle prefix for a given row
func GetBundlePrefixPos(rows uint32, bundleSize byte) uint32 {
	return GetBundleFromRow(rows, bundleSize)*(uint32(bundleSize)+1) + 1
}

// GetBundleStartOffset returns the offset of the first key of a given bundle
func GetBundleStartOffset(bundleId uint32, bundleSize byte) uint32 {
	return bundleId*(uint32(bundleSize)+1) + 1
}

// GetOffsetFromRow returns the offset of a given row
func GetOffsetFromRow(rows uint32, bundleSize byte) uint32 {
	return GetBundlePrefixPos(rows, bundleSize) + rows%uint32(bundleSize) + 1
}

// GetRowFromOffset returns the row index for a given offset
func GetRowFromOffset(offset uint32, bundleSize byte) uint32 {
	return offset - 1 - GetBundleFromOffset(offset, bundleSize) - 1
}
