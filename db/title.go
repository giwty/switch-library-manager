package db

import (
	"fmt"
	"strconv"
)

// Get a title ID base and type
func getTitleBaseAndType(id string) (string, uint64, error) {
	var getErr error
	var intId uint64
	var baseId string
	var titleType uint64

	// Convert the Id to an integer
	intId, getErr = strconv.ParseUint(id, 16, 64)
	if getErr != nil {
		return baseId, titleType, getErr
	}

	// Get the base ID
	// Mask the ID with the title bitmask
	baseIntId := intId & TITLE_ID_BITMASK

	// Convert it to an hexa string
	baseId = fmt.Sprintf("%x", baseIntId)

	// Get type
	// Mask the ID with the title bitmask
	titleType = intId &^ TITLE_ID_BITMASK

	return baseId, titleType, getErr
}
