package block

import (
	"encoding/binary"
	"fmt"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/storage"
)

type Footer struct {
	Version     common.TableVersion
	MetaIndexBH common.BlockHandle
}

func (f *Footer) Serialise() []byte {
	footerSize := common.TableFooterSize[f.Version]
	buf := make([]byte, footerSize)
	f.MetaIndexBH.EncodeInto(buf[:])
	binary.LittleEndian.PutUint32(buf[cap(buf)-common.MagicNumberLen-common.TableVersionLen:], uint32(f.Version))
	copy(buf[cap(buf)-common.MagicNumberLen:], common.MagicNumber)
	return buf
}

func (f *Footer) GetMetaIndex() *common.BlockHandle {
	return &f.MetaIndexBH
}

func ReadFooter(
	reader storage.ILayoutReader,
	size uint64,
) (*Footer, error) {
	buf := make([]byte, common.MaxPossibleFooterSize)
	offset := max(0, size-uint64(common.MaxPossibleFooterSize))
	if err := reader.ReadAt(buf, offset); err != nil {
		return nil, err
	}

	// Parse the footer from the read buffer, refer to
	// the function Footer.Serialise() or README to understand how the foot
	// is constructed
	switch magic := buf[len(buf)-common.MagicNumberLen:]; string(magic) {
	case common.MagicNumber:
		version := common.TableVersion(binary.LittleEndian.Uint32(buf[len(buf)-common.MagicNumberLen-common.TableVersionLen:]))
		if len(buf) < common.TableFooterSize[version] {
			return nil, fmt.Errorf("footer is too short")
		}

		metaIndexBuf := buf[len(buf)-common.TableFooterSize[version]:]
		metaIndexBH := &common.BlockHandle{}
		if n := metaIndexBH.DecodeFrom(metaIndexBuf); n <= 0 {
			return nil, fmt.Errorf("%w failed to decode the meta index buf", common.InternalServerError)
		}

		return &Footer{
			Version:     version,
			MetaIndexBH: *metaIndexBH,
		}, nil

	default:
		return nil, fmt.Errorf("unrecognized magic number: %s", magic)
	}
}
