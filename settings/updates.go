package settings

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/mcuadros/go-version"
)

const (
	SLM_VERSION_URL = "https://raw.githubusercontent.com/giwty/switch-library-manager/master/slm.json"
)

// Check if an update is available
func CheckForUpdates() (bool, error) {

	localVer := SLM_VERSION

	res, err := http.Get(SLM_VERSION_URL)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, err
	}

	remoteValues := map[string]string{}
	err = json.Unmarshal(body, &remoteValues)
	if err != nil {
		return false, err
	}

	remoteVer := remoteValues["version"]

	if version.CompareSimple(remoteVer, localVer) > 0 {
		return true, nil
	}

	return false, nil
}
