package switchfs

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

type fsHeader struct {
	encType       byte //(0 = Auto, 1 = None, 2 = AesCtrOld, 3 = AesCtr, 4 = AesCtrEx)
	fsType        byte //(0 = RomFs, 1 = PartitionFs)
	hashType      byte // (0 = Auto, 2 = HierarchicalSha256, 3 = HierarchicalIntegrity (Ivfc))
	fsHeaderBytes []byte
	generation    uint32
}

type fsEntry struct {
	StartOffset uint32
	EndOffset   uint32
	Size        uint32
}

type hashInfo struct {
	pfs0HeaderOffset uint64
	pfs0size         uint64
}

func getFsEntry(ncaHeader *ncaHeader, index int) fsEntry {
	fsEntryOffset := 0x240 + 0x10*index
	fsEntryBytes := ncaHeader.headerBytes[fsEntryOffset : fsEntryOffset+0x10]

	entryStartOffset := binary.LittleEndian.Uint32(fsEntryBytes[0x0:0x4]) * 0x200
	entryEndOffset := binary.LittleEndian.Uint32(fsEntryBytes[0x4:0x8]) * 0x200

	return fsEntry{StartOffset: entryStartOffset, EndOffset: entryEndOffset, Size: entryEndOffset - entryStartOffset}
}

func getFsHeader(ncaHeader *ncaHeader, index int) (*fsHeader, error) {
	fsHeaderHashOffset := /*hash pfs0HeaderOffset*/ 0x280 + /*hash pfs0size*/ 0x20*index
	fsHeaderHash := ncaHeader.headerBytes[fsHeaderHashOffset : fsHeaderHashOffset+0x20]

	fsHeaderOffset := 0x400 + 0x200*index
	fsHeaderBytes := ncaHeader.headerBytes[fsHeaderOffset : fsHeaderOffset+0x200]

	actualHash := sha256.Sum256(fsHeaderBytes)

	if bytes.Compare(actualHash[:], fsHeaderHash) != 0 {
		return nil, errors.New("fs headerBytes hash mismatch")
	}

	result := fsHeader{fsHeaderBytes: fsHeaderBytes}

	result.fsType = fsHeaderBytes[0x2:0x3][0]
	result.hashType = fsHeaderBytes[0x3:0x4][0]
	result.encType = fsHeaderBytes[0x4:0x5][0]

	generationBytes := fsHeaderBytes[0x140 : 0x140+0x4] //generation
	result.generation = binary.LittleEndian.Uint32(generationBytes)

	return &result, nil
}

func (fh *fsHeader) getHashInfo() (*hashInfo, error) {
	hashInfoBytes := fh.fsHeaderBytes[0x8:0x100]
	result := hashInfo{}
	if fh.hashType == 2 {

		result.pfs0HeaderOffset = binary.LittleEndian.Uint64(hashInfoBytes[0x38 : 0x38+0x8])
		result.pfs0size = binary.LittleEndian.Uint64(hashInfoBytes[0x40 : 0x40+0x8])
		return &result, nil
	} else if fh.hashType == 3 {
		result.pfs0HeaderOffset = binary.LittleEndian.Uint64(hashInfoBytes[0x88 : 0x88+0x8])
		result.pfs0size = binary.LittleEndian.Uint64(hashInfoBytes[0x90 : 0x90+0x8])
		return &result, nil
	}
	return nil, errors.New("non supported hash type")
}
