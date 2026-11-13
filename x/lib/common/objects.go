package common

import (
	"fmt"
	"strconv"
	"strings"
)

type DiskfileNum int64

type ObjectType byte

const (
	TypeManifest ObjectType = iota
	TypeTable
	TypeWAL
	TypeLock
)

var (
	ObjectTypeFromString = map[string]ObjectType{
		"manifest": TypeManifest,
		"sst":      TypeTable,
		"wal":      TypeWAL,
		"LOCK":     TypeLock,
	}
	ObjectTypeToString = map[ObjectType]string{
		TypeManifest: "manifest",
		TypeTable:    "sst",
		TypeWAL:      "wal",
		TypeLock:     "LOCK",
	}
)

func GetFileName(objType ObjectType, num DiskfileNum) string {
	return fmt.Sprintf("%s-%d", ObjectTypeToString[objType], num)
}

func ParseFileName(name string) (ObjectType, DiskfileNum, bool) {
	parseDiskFileNum := func(s string) (DiskfileNum, bool) {
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return DiskfileNum(0), false
		}
		return DiskfileNum(u), true
	}

	i := strings.IndexByte(name, '-')
	if i == 0 {
		return 0, 0, false
	}

	dfn, ok := parseDiskFileNum(name[i+1:])
	if !ok {
		return 0, 0, false
	}

	object, ok := ObjectTypeFromString[name[:i]]
	if !ok {
		return 0, 0, false
	}

	return object, dfn, true
}
