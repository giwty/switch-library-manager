package switchfs

import (
	"crypto/aes"
	"encoding/binary"
	"encoding/hex"
	"github.com/giwty/switch-library-manager/switchfs/_crypto"
	"strconv"
)

//https://switchbrew.org/wiki/NCA_Format

type ncaHeader struct {
	headerBytes    []byte
	rightsId       []byte
	titleId        []byte
	distribution   byte
	contentType    byte // (0x00 = Program, 0x01 = Meta, 0x02 = Control, 0x03 = Manual, 0x04 = Data, 0x05 = PublicData)
	keyGeneration2 byte
	keyGeneration1 byte
	encryptedKeys  []byte // 4 * 0x10
	cryptoType     byte   //(0x00 = Application, 0x01 = Ocean, 0x02 = System)

}

func (n *ncaHeader) HasRightsId() bool {
	for i := 0; i < 0x10; i++ {
		if n.rightsId[i] != 0 {
			return true
		}
	}
	return false
}

func (n *ncaHeader) getKeyRevision() int {
	keyGeneration := max(n.keyGeneration1, n.keyGeneration2)
	keyRevision := keyGeneration - 1
	if keyGeneration == 0 {
		keyRevision = 0
	}
	return int(keyRevision)
}

func max(a byte, b byte) byte {
	if a > b {
		return a
	}
	return b
}

func DecryptNcaHeader(key string, encHeader []byte) (*ncaHeader, error) {
	headerKey, _ := hex.DecodeString(key)
	c, err := _crypto.NewCipher(aes.NewCipher, headerKey)
	if err != nil {
		return nil, err
	}
	sector := 0
	sectorSize := 0x200
	endOffset := 0x400
	decryptNcaHeader, err := _decryptNcaHeader(c, encHeader, endOffset, sectorSize, sector)
	if err != nil {
		return nil, err
	}

	magic := string(decryptNcaHeader[0x200:0x204])

	if magic == "NCA3" {
		endOffset = 0xC00
		decryptNcaHeader, err = _decryptNcaHeader(c, encHeader, endOffset, sectorSize, sector)
	}

	result := ncaHeader{headerBytes: decryptNcaHeader}

	result.distribution = decryptNcaHeader[0x204:0x205][0]
	result.contentType = decryptNcaHeader[0x205:0x206][0]
	result.rightsId = decryptNcaHeader[0x230 : 0x230+0x10]

	title_id_dec := binary.LittleEndian.Uint64(decryptNcaHeader[0x210 : 0x210+0x8])
	result.titleId = []byte(strconv.FormatInt(int64(title_id_dec), 16))
	result.keyGeneration1 = decryptNcaHeader[0x206:0x207][0]
	result.keyGeneration2 = decryptNcaHeader[0x220:0x221][0]

	encryptedKeysAreaOffset := 0x300
	result.encryptedKeys = decryptNcaHeader[encryptedKeysAreaOffset : encryptedKeysAreaOffset+(0x10*4)]

	result.cryptoType = decryptNcaHeader[0x207:0x208][0]

	return &result, nil
}

func _decryptNcaHeader(c *_crypto.Cipher, header []byte, end int, sectorSize int, sectorNum int) ([]byte, error) {
	decrypted := make([]byte, len(header))
	for pos := 0; pos < end; pos += sectorSize {
		/* Workaround for Nintendo's custom sector...manually generate the tweak. */
		tweak := getNintendoTweak(sectorNum)

		pos := sectorSize * sectorNum
		c.Decrypt(decrypted[pos:pos+sectorSize], header[pos:pos+sectorSize], &tweak)
		sectorNum++
	}
	return decrypted, nil
}

func getNintendoTweak(sector int) [16]byte {
	tweak := [16]byte{}
	for i := 0xF; i >= 0; i-- { /* Nintendo LE custom tweak... */
		tweak[i] = uint8(sector & 0xFF)
		sector >>= 8
	}
	return tweak
}
