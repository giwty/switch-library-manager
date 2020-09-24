package settings

import (
	"errors"
	"github.com/magiconair/properties"
	"path/filepath"
)

var (
	keysInstance *switchKeys
)

type switchKeys struct {
	keys map[string]string
}

func (k *switchKeys) GetKey(keyName string) string {
	return k.keys[keyName]
}

func SwitchKeys() (*switchKeys, error) {
	return keysInstance, nil
}

func InitSwitchKeys(baseFolder string) (*switchKeys, error) {

	// init from a file
	p, err := properties.LoadFile(filepath.Join(baseFolder, "prod.keys"), properties.UTF8)
	if err != nil {
		p, err = properties.LoadFile("${HOME}/.switch/prod.keys", properties.UTF8)
	}
	if err != nil {
		prodKeysPath := ReadSettings(baseFolder).Prodkeys
		if prodKeysPath != "" {
			p, err = properties.LoadFile(filepath.Join(prodKeysPath, "prod.keys"), properties.UTF8)
		}
	}
	if err != nil {
		return nil, errors.New("couldn't find prod.keys")
	}
	keysInstance = &switchKeys{keys: map[string]string{}}
	for _, key := range p.Keys() {
		value, _ := p.Get(key)
		keysInstance.keys[key] = value
	}

	return keysInstance, nil
}
