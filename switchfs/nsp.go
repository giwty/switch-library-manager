package switchfs

import (
	"bytes"
	"errors"
	"os"
	"strings"
)

func ReadNspMetadata(filePath string) (*ContentMetaAttributes, error) {

	if !strings.HasSuffix(filePath, "nsp") && !strings.HasSuffix(filePath, "nsz") {
		return nil, errors.New("only NSP file type is supported")
	}

	pfs0, err := ReadPfs0File(filePath)
	if err != nil {
		return nil, errors.New("Invalid NSP file, reason - [" + err.Error() + "]")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()
	for _, pfs0File := range pfs0.Files {

		fileOffset := int64(pfs0File.StartOffset)

		if strings.Contains(pfs0File.Name, "cnmt.nca") {
			section, err := openMetaNcaDataSection(file, fileOffset)
			if err != nil {
				return nil, err
			}
			pfs0, err := readPfs0(bytes.NewReader(section))
			if err != nil {
				return nil, err
			}
			cnmt, err := readBinaryCnmt(pfs0, section)
			if err != nil {
				return nil, err
			}
			return cnmt, err

		} else if strings.Contains(pfs0File.Name, ".cnmt.xml") {
			xmlBytes := make([]byte, pfs0File.Size)
			_, err = file.ReadAt(xmlBytes, fileOffset)
			if err != nil {
				return nil, err
			}

			cnmt, err := readXmlCnmt(xmlBytes)
			if err != nil {
				return nil, err
			}
			return cnmt, err
		}
	}
	return nil, errors.New("could not generate metadata")

}
