package settings

import (
	"encoding/json"
	"fmt"
	"github.com/mcuadros/go-version"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

var (
	settingsInstance *AppSettings
)

const (
	SETTINGS_FILENAME      = "settings.json"
	TITLE_JSON_FILENAME    = "titles.json"
	VERSIONS_JSON_FILENAME = "versions.json"
	SLM_VERSION_FILE       = "slm.json"
	TITLES_JSON_URL        = "https://tinfoil.media/repo/db/titles.json"
	VERSIONS_JSON_URL      = "https://tinfoil.media/repo/db/versions.json"
	SLM_VERSION_URL        = "https://raw.githubusercontent.com/giwty/switch-library-manager/master/slm.json"
)

const (
	TEMPLATE_TITLE_ID   = "TITLE_ID"
	TEMPLATE_TITLE_NAME = "TITLE_NAME"
	TEMPLATE_DLC_NAME   = "DLC_NAME"
	TEMPLATE_VERSION    = "VERSION"
	TEMPLATE_TYPE       = "TYPE"
)

type OrganizeOptions struct {
	CreateFolderPerGame  bool   `json:"create_folder_per_game"`
	RenameFiles          bool   `json:"rename_files"`
	DeleteEmptyFolders   bool   `json:"delete_empty_folders"`
	DeleteOldUpdateFiles bool   `json:"delete_old_update_files"`
	FolderNameTemplate   string `json:"folder_name_template"`
	FileNameTemplate     string `json:"file_name_template"`
}

type AppSettings struct {
	VersionsEtag           string          `json:"versions_etag"`
	TitlesEtag             string          `json:"titles_etag"`
	Folder                 string          `json:"folder"`
	GUI                    bool            `json:"gui"`
	Debug                  bool            `json:"debug"`
	CheckForMissingUpdates bool            `json:"check_for_missing_updates"`
	CheckForMissingDLC     bool            `json:"check_for_missing_dlc"`
	OrganizeOptions        OrganizeOptions `json:"organize_options"`
	ScanRecursively        bool            `json:"scan_recursively"`
	GuiPagingSize          int             `json:"gui_page_size"`
}

func ReadSettingsAsJSON(baseFolder string) string {
	if _, err := os.Stat(filepath.Join(baseFolder, SETTINGS_FILENAME)); err != nil {
		saveDefaultSettings(baseFolder)
	}
	file, _ := os.Open(filepath.Join(baseFolder, SETTINGS_FILENAME))
	bytes, _ := ioutil.ReadAll(file)
	return string(bytes)
}

func ReadSettings(baseFolder string) *AppSettings {
	if settingsInstance != nil {
		return settingsInstance
	}
	settingsInstance = &AppSettings{Debug: false, GuiPagingSize: 100}
	if _, err := os.Stat(filepath.Join(baseFolder, SETTINGS_FILENAME)); err == nil {
		file, err := os.Open(filepath.Join(baseFolder, SETTINGS_FILENAME))
		if err != nil {
			zap.S().Warnf("Missing or corrupted config file, creating a new one")
			return saveDefaultSettings(baseFolder)
		} else {
			_ = json.NewDecoder(file).Decode(&settingsInstance)
			return settingsInstance
		}
	} else {
		return saveDefaultSettings(baseFolder)
	}
}

func saveDefaultSettings(baseFolder string) *AppSettings {
	settingsInstance = &AppSettings{
		TitlesEtag:             "W/\"695350c8106bd61:0\"",
		VersionsEtag:           "W/\"f28b82cc956ad61:0\"",
		Folder:                 "",
		GUI:                    true,
		GuiPagingSize:          100,
		CheckForMissingUpdates: true,
		CheckForMissingDLC:     true,
		ScanRecursively:        true,
		Debug:                  false,
		OrganizeOptions: OrganizeOptions{
			RenameFiles:         false,
			CreateFolderPerGame: false,
			FolderNameTemplate:  fmt.Sprintf("{%v}", TEMPLATE_TITLE_NAME),
			FileNameTemplate: fmt.Sprintf("{%v} [{%v}][{%v}][{%v}]", TEMPLATE_TITLE_NAME, TEMPLATE_DLC_NAME,
				TEMPLATE_TITLE_ID, TEMPLATE_VERSION),
			DeleteEmptyFolders:   false,
			DeleteOldUpdateFiles: false,
		},
	}
	return SaveSettings(settingsInstance, baseFolder)
}

func SaveSettings(settings *AppSettings, baseFolder string) *AppSettings {
	file, _ := json.MarshalIndent(settings, "", " ")
	_ = ioutil.WriteFile(filepath.Join(baseFolder, SETTINGS_FILENAME), file, 0644)
	settingsInstance = settings
	return settings
}

func CheckForUpdates(workingFolder string) (bool, error) {
	file, err := os.Open(filepath.Join(workingFolder, SLM_VERSION_FILE))
	if err != nil {
		return false, err
	}
	localValues := map[string]string{}
	err = json.NewDecoder(file).Decode(&localValues)
	if err != nil {
		return false, err
	}

	localVer := localValues["version"]

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
