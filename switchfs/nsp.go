package switchfs

import (
	"bytes"
	"errors"
	"go.uber.org/zap"
	"strings"
)

func ReadNspMetadata(filePath string) (map[string]*ContentMetaAttributes, error) {

	pfs0, err := ReadPfs0File(filePath)
	if err != nil {
		return nil, errors.New("Invalid NSP file, reason - [" + err.Error() + "]")
	}

	file, err := OpenFile(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	contentMap := map[string]*ContentMetaAttributes{}

	for _, pfs0File := range pfs0.Files {

		fileOffset := int64(pfs0File.StartOffset)

		if strings.Contains(pfs0File.Name, "cnmt.nca") {
			_, section, err := openMetaNcaDataSection(file, fileOffset)
			if err != nil {
				return nil, err
			}
			currpfs0, err := readPfs0(bytes.NewReader(section), 0x0)
			if err != nil {
				return nil, err
			}
			currCnmt, err := readBinaryCnmt(currpfs0, section)
			if err != nil {
				return nil, err
			}
			if currCnmt.Type != "DLC" {
				nacp, err := ExtractNacp(currCnmt, file, pfs0, 0)
				if err != nil {
					zap.S().Debug("Failed to extract nacp [%v]\n", err.Error())
				}
				currCnmt.Ncap = nacp
			}

			contentMap[currCnmt.TitleId] = currCnmt

		} /*else if strings.Contains(pfs0File.Name, ".cnmt.xml") {
			xmlBytes := make([]byte, pfs0File.Size)
			_, err = file.ReadAt(xmlBytes, fileOffset)
			if err != nil {
				return nil, err
			}

			currCnmt, err := readXmlCnmt(xmlBytes)
			if err != nil {
				return nil, err
			}
			contentMap[currCnmt.TitleId] = currCnmt
		}*/
	}
	return contentMap, nil

}
