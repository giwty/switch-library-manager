package settings

import (
	"errors"
	"github.com/magiconair/properties"
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
	if keysInstance != nil {
		return keysInstance, nil
	}
	// init from a file
	p, err := properties.LoadFile("./prod.keys", properties.UTF8)
	if err != nil {
		p, err = properties.LoadFile("${HOME}/.switch/prod.keys", properties.UTF8)
	}
	if err != nil {
		return nil, errors.New("couldn't find keys.prod")
	}
	keysInstance = &switchKeys{keys: map[string]string{}}
	for _, key := range p.Keys() {
		value, _ := p.Get(key)
		keysInstance.keys[key] = value
	}

	return keysInstance, nil
}
