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
	path := filepath.Join(baseFolder, "prod.keys")
	p, err := properties.LoadFile(path, properties.UTF8)
	if err != nil {
		path = "${HOME}/.switch/prod.keys"
		p, err = properties.LoadFile(path, properties.UTF8)
	}
	settings := ReadSettings(baseFolder)
	if err != nil {
		path := settings.Prodkeys
		if path != "" {
			p, err = properties.LoadFile(filepath.Join(path, "prod.keys"), properties.UTF8)
		}
	}
	if err != nil {
		return nil, errors.New("Error trying to read prod.keys [reason:" + err.Error() + "]")
	}
	settings.Prodkeys = path
	SaveSettings(settings, baseFolder)
	keysInstance = &switchKeys{keys: map[string]string{}}
	for _, key := range p.Keys() {
		value, _ := p.Get(key)
		keysInstance.keys[key] = value
	}

	return keysInstance, nil
}
