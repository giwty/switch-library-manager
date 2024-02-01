package switchfs

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/giwty/switch-library-manager/switchfs/_crypto"
	"io"
)

const (
	NcaSectionType_Code = iota
	NcaSectionType_Data
	NcaSectionType_Logo
)
const (
	NcaContentType_Program = iota
	NcaContentType_Meta
	NcaContentType_Control
	NcaContentType_Manual
	NcaContentType_Data
	NcaContentType_PublicData
)

func openMetaNcaDataSection(reader io.ReaderAt, ncaOffset int64) (*fsHeader, []byte, error) {
	//read the NCA headerBytes
	encNcaHeader := make([]byte, 0xC00)
	n, err := reader.ReadAt(encNcaHeader, ncaOffset)

	if err != nil {
		return nil, nil, errors.New("failed to read NCA header " + err.Error())
	}
	if n != 0xC00 {
		return nil, nil, errors.New("failed to read NCA header")
	}

	keys, err := settings.SwitchKeys()
	if err != nil {
		return nil, nil, err
	}
	headerKey := keys.GetKey("header_key")
	if headerKey == "" {
		return nil, nil, errors.New("missing key - header_key")
	}
	ncaHeader, err := DecryptNcaHeader(headerKey, encNcaHeader)
	if err != nil {
		return nil, nil, err
	}

	if ncaHeader.HasRightsId() {
		//fail - need title keys
		return nil, nil, errors.New("non standard encryption is not supported")
	}

	/*if ncaHeader.contentType != NcaContentType_Meta {
		return nil, errors.New("not a meta NCA")
	}*/

	dataSectionIndex := 0

	fsHeader, err := getFsHeader(ncaHeader, dataSectionIndex)
	if err != nil {
		return nil, nil, err
	}

	entry := getFsEntry(ncaHeader, dataSectionIndex)

	if entry.Size == 0 {
		return nil, nil, errors.New("empty section")
	}

	encodedEntryContent := make([]byte, entry.Size)
	entryOffset := ncaOffset + int64(entry.StartOffset)
	_, err = reader.ReadAt(encodedEntryContent, entryOffset)
	if err != nil {
		return nil, nil, err
	}
	if fsHeader.encType != 3 {
		return nil, nil, errors.New("non supported encryption type [encryption type:" + string(fsHeader.encType))
	}

	/*if fsHeader.hashType != 2 { //Sha256 (FS_TYPE_PFS0)
		return nil, errors.New("non FS_TYPE_PFS0")
	}*/
	decoded, err := decryptAesCtr(ncaHeader, fsHeader, entry.StartOffset, entry.Size, encodedEntryContent)
	if err != nil {
		return nil, nil, err
	}
	hashInfo, err := fsHeader.getHashInfo()
	if err != nil {
		return nil, nil, err
	}

	return fsHeader, decoded[hashInfo.pfs0HeaderOffset:], nil
}

func decryptAesCtr(ncaHeader *ncaHeader, fsHeader *fsHeader, offset uint32, size uint32, encoded []byte) ([]byte, error) {
	keyRevision := string(ncaHeader.getKeyRevision())
	cryptoType := ncaHeader.cryptoType

	if cryptoType != 0 {
		return []byte{}, errors.New("unsupported crypto type")
	}

	keys, _ := settings.SwitchKeys()

	keyName := fmt.Sprintf("key_area_key_application_%x", keyRevision)
	KeyString := keys.GetKey(keyName)
	if KeyString == "" {
		return nil, errors.New(fmt.Sprintf("missing Key_area_key[%v]", keyName))
	}
	key, _ := hex.DecodeString(KeyString)

	decKey := _crypto.DecryptAes128Ecb(ncaHeader.encryptedKeys[0x20:0x30], key)

	counter := make([]byte, 0x10)
	binary.BigEndian.PutUint64(counter, uint64(fsHeader.generation))
	binary.BigEndian.PutUint64(counter[8:], uint64(offset/0x10))

	c, _ := aes.NewCipher(decKey)

	decContent := make([]byte, size)

	s := cipher.NewCTR(c, counter)
	s.XORKeyStream(decContent, encoded[0:size])

	return decContent, nil
}
