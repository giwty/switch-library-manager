package switchfs

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

const (
	fileEntryTableSize = 0x18
	pfs0Magic          = "PFS0"
)

type fileEntry struct {
	StartOffset uint64
	Size        uint64
	Name        string
}

// PFS0 struct to represent PFS0 filesystem of NSP
type PFS0 struct {
	Filepath  string
	Size      uint64
	HeaderLen uint16
	Files     []fileEntry
}

// https://wiki.oatmealdome.me/PFS0_(File_Format)
func ReadPfs0File(filePath string) (*PFS0, error) {

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	p, err := readPfs0(file)
	if err != nil {
		return nil, err
	}
	p.Filepath = filePath
	return p, nil
}

func readPfs0(reader io.ReaderAt) (*PFS0, error) {

	header := make([]byte, 0xC)
	n, err := reader.ReadAt(header, 0x0)
	if err != nil {
		return nil, err
	}
	if n != 0xC {
		return nil, errors.New("failed to read file")
	}
	if string(header[:0x4]) != pfs0Magic {
		return nil, errors.New("Invalid NSP headerBytes. Expected 'PFS0', got '" + string(header[:0x4]) + "'")
	}
	p := &PFS0{}

	fileCount := binary.LittleEndian.Uint16(header[0x4:0x8])

	fileEntryTableOffset := 0x10 + (fileEntryTableSize * fileCount)

	stringsLen := binary.LittleEndian.Uint16(header[0x8:0xC])
	p.HeaderLen = fileEntryTableOffset + stringsLen
	fileNamesBuffer := make([]byte, stringsLen)
	_, err = reader.ReadAt(fileNamesBuffer, int64(fileEntryTableOffset))
	if err != nil {
		return nil, err
	}

	p.Files = make([]fileEntry, fileCount)
	// go over the fileEntries
	for i := uint16(0); i < fileCount; i++ {
		fileEntryTable := make([]byte, fileEntryTableSize)
		_, err = reader.ReadAt(fileEntryTable, int64(0x10+(fileEntryTableSize*i)))
		if err != nil {
			return nil, err
		}

		fileOffset := binary.LittleEndian.Uint64(fileEntryTable[0:8])
		fileSize := binary.LittleEndian.Uint64(fileEntryTable[8:16])
		var nameBytes []byte
		for _, b := range fileNamesBuffer[binary.LittleEndian.Uint16(fileEntryTable[16:20]):] {
			if b == 0x0 {
				break
			} else {
				nameBytes = append(nameBytes, b)
			}
		}

		p.Files[i] = fileEntry{fileOffset + uint64(p.HeaderLen), fileSize, string(nameBytes)}
	}
	p.HeaderLen += stringsLen

	return p, nil
}
