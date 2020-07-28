package switchfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"strings"
)

func ReadXciMetadata(filePath string) (*ContentMetaAttributes, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	header := make([]byte, 0x200)
	_, err = file.Read(header)
	if err != nil {
		return nil, err
	}

	if string(header[0x100:0x104]) != "HEAD" {
		return nil, errors.New("Invalid XCI headerBytes. Expected 'HEAD', got '" + string(header[:0x4]) + "'")
	}

	rootPartitionOffset := binary.LittleEndian.Uint64(header[0x130:0x138])
	rootPartitionSize := binary.LittleEndian.Uint64(header[0x138:0x140])

	rootPartionBytes := make([]byte, rootPartitionSize)
	_, err = file.ReadAt(rootPartionBytes, int64(rootPartitionOffset))
	if err != nil {
		return nil, err
	}
	rootHfs0, err := readPfs0(bytes.NewReader(rootPartionBytes))
	if err != nil {
		return nil, err
	}

	secureHfs0, secureOffset, err := readSecurePartition(file, rootHfs0, rootPartitionOffset)
	if err != nil {
		return nil, err
	}

	var cnmt *ContentMetaAttributes = &ContentMetaAttributes{Type: "BASE"}

	for _, pfs0File := range secureHfs0.Files {

		fileOffset := secureOffset + int64(pfs0File.StartOffset)

		if strings.Contains(pfs0File.Name, "cnmt.nca") {
			section, err := openMetaNcaDataSection(file, fileOffset)
			if err != nil {
				return nil, err
			}
			pfs0, err := readPfs0(bytes.NewReader(section))
			if err != nil {
				return nil, err
			}
			currCnmt, err := readBinaryCnmt(pfs0, section)
			if err != nil {
				return nil, err
			}
			if currCnmt.Type == "BASE" {
				cnmt.TitleId = currCnmt.TitleId
			}

			if currCnmt.Version > cnmt.Version {
				cnmt.Version = currCnmt.Version
			}

		} else if strings.Contains(pfs0File.Name, ".cnmt.xml") {
			xmlBytes := make([]byte, pfs0File.Size)
			_, err = file.ReadAt(xmlBytes, fileOffset)
			if err != nil {
				return nil, err
			}

			currCnmt, err := readXmlCnmt(xmlBytes)
			if err != nil {
				return nil, err
			}
			if cnmt.Type == "BASE" {
				cnmt.TitleId = currCnmt.TitleId
			}

			if currCnmt.Version > cnmt.Version {
				cnmt.Version = currCnmt.Version
			}
		}
	}

	return cnmt, nil
}

func readSecurePartition(file *os.File, hfs0 *PFS0, rootPartitionOffset uint64) (*PFS0, int64, error) {
	for _, hfs0File := range hfs0.Files {
		offset := int64(rootPartitionOffset) + int64(hfs0File.StartOffset)

		if hfs0File.Name == "secure" {
			securePartitionBytes := make([]byte, hfs0File.Size)
			_, err := file.ReadAt(securePartitionBytes, offset)
			if err != nil {
				return nil, 0, err
			}
			securePartition, err := readPfs0(bytes.NewReader(securePartitionBytes))
			if err != nil {
				return nil, 0, err
			}
			return securePartition, offset, nil
		}
	}
	return nil, 0, nil
}
