package settings

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/magiconair/properties"
)

const (
	SETTINGS_PRODKEYS        = "prod.keys"
	SETTINGS_PRODKEYS_DIR    = ".switch"
	SETTINGS_PRODKEYS_HEADER = "header_key"
)

// Get the working dir prod keys path
func (a *AppSettings) getBaseKeysPath() string {
	return filepath.Join(a.baseFolder, SETTINGS_PRODKEYS)
}

// Get the globally accepted default for prod keys path
func (a *AppSettings) getDefaultKeysPath() string {
	return filepath.Join(a.Homedir, SETTINGS_PRODKEYS_DIR, SETTINGS_PRODKEYS)
}

// Grab a Switch key from the settings
func (a *AppSettings) GetKey(keyName string) string {
	if key, ok := a.SwitchKeys[keyName]; ok {
		return key
	}

	return ""
}

// Check if a Switch key exists and convert to string
func (a *AppSettings) HasKey(keyName string) string {
	return strconv.FormatBool(a.GetKey(keyName) != "")
}

// Read the Switch keys from the file
func (a *AppSettings) ReadKeys() error {
	var props *properties.Properties
	var propsErr error

	// Don't default to nil or it'll skip everything
	propsErr = errors.New("")

	// Trying from settings
	if a.Prodkeys != "" {
		props, propsErr = properties.LoadFile(a.Prodkeys, properties.UTF8)
	}

	// If missing or error, trying to load from the working dir
	if propsErr != nil {
		props, propsErr = properties.LoadFile(a.getBaseKeysPath(), properties.UTF8)

		// If error, try to load from the accepted default
		if propsErr != nil {
			props, propsErr = properties.LoadFile(a.getDefaultKeysPath(), properties.UTF8)
		}
	}

	// If still error, bail
	if propsErr != nil {
		return fmt.Errorf("error trying to read %s. Reason: %s", SETTINGS_PRODKEYS, propsErr)
	}

	// Read the keys into the map
	for _, key := range props.Keys() {
		value, _ := props.Get(key)
		a.SwitchKeys[key] = value
	}

	return nil
}
