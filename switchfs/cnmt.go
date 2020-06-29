package switchfs

import (
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"strings"
)

const (
	ContentMetaType_SystemProgram        = 1
	ContentMetaType_SystemData           = 2
	ContentMetaType_SystemUpdate         = 3
	ContentMetaType_BootImagePackage     = 4
	ContentMetaType_BootImagePackageSafe = 5
	ContentMetaType_Application          = 0x80
	ContentMetaType_Patch                = 0x81
	ContentMetaType_AddOnContent         = 0x82
	ContentMetaType_Delta                = 0x83
)

type ContentMetaAttributes struct {
	TitleId string
	Version int
	Type    string
}

type ContentMeta struct {
	XMLName                       xml.Name `xml:"ContentMeta"`
	Text                          string   `xml:",chardata"`
	Type                          string   `xml:"Type"`
	ID                            string   `xml:"Id"`
	Version                       int      `xml:"Version"`
	RequiredDownloadSystemVersion string   `xml:"RequiredDownloadSystemVersion"`
	Content                       []struct {
		Text          string `xml:",chardata"`
		Type          string `xml:"Type"`
		ID            string `xml:"Id"`
		Size          string `xml:"Size"`
		Hash          string `xml:"Hash"`
		KeyGeneration string `xml:"KeyGeneration"`
	} `xml:"Content"`
	Digest                string `xml:"Digest"`
	KeyGenerationMin      string `xml:"KeyGenerationMin"`
	RequiredSystemVersion string `xml:"RequiredSystemVersion"`
	OriginalId            string `xml:"OriginalId"`
}

func readBinaryCnmt(pfs0 *PFS0, data []byte) (*ContentMetaAttributes, error) {
	cnmtFile := pfs0.Files[0]
	cnmt := data[int64(cnmtFile.StartOffset):]
	titleId := binary.LittleEndian.Uint64(cnmt[0:0x8])
	version := binary.LittleEndian.Uint32(cnmt[0x8:0xC])
	metaType := ""
	switch cnmt[0xC:0xD][0] {
	case ContentMetaType_Application:
		metaType = "BASE"
	case ContentMetaType_AddOnContent:
		metaType = "DLC"
	case ContentMetaType_Patch:
		metaType = "UPD"
	}
	return &ContentMetaAttributes{Version: int(version), TitleId: fmt.Sprintf("0%x", titleId), Type: metaType}, nil
}

func readXmlCnmt(xmlBytes []byte) (*ContentMetaAttributes, error) {
	cmt := &ContentMeta{}
	err := xml.Unmarshal(xmlBytes, &cmt)
	if err != nil {
		return nil, err
	}
	titleId := strings.Replace(cmt.ID, "0x", "", 1)
	return &ContentMetaAttributes{Version: cmt.Version, TitleId: titleId, Type: cmt.Type}, nil
}
