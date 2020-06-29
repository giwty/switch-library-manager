package settings

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

const (
	SETTINGS_FILENAME = "settings.json"
)

var (
	settingsInstance *settings
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

type settings struct {
	VersionsEtag           string          `json:"versions_etag"`
	TitlesEtag             string          `json:"titles_etag"`
	Folder                 string          `json:"folder"`
	CheckForMissingUpdates bool            `json:"check_for_missing_updates"`
	CheckForMissingDLC     bool            `json:"check_for_missing_dlc"`
	OrganizeOptions        OrganizeOptions `json:"organize_options"`
	ScanRecursively        bool            `json:"scan_recursively"`
}

func ReadSettings() *settings {
	if settingsInstance != nil {
		return settingsInstance
	}
	settingsInstance = &settings{}
	if _, err := os.Stat(SETTINGS_FILENAME); err == nil {
		file, err := os.Open("./" + SETTINGS_FILENAME)
		if err != nil {
			log.Print("Missing or corrupted config file, creating a new one")
			return saveDefaultSettings()
		} else {
			_ = json.NewDecoder(file).Decode(&settingsInstance)
			return settingsInstance
		}
	} else {
		return saveDefaultSettings()
	}
}

func saveDefaultSettings() *settings {
	settingsInstance = &settings{
		VersionsEtag:           "",
		TitlesEtag:             "",
		Folder:                 "",
		CheckForMissingUpdates: true,
		CheckForMissingDLC:     true,
		ScanRecursively:        true,
		OrganizeOptions: OrganizeOptions{
			RenameFiles:         true,
			CreateFolderPerGame: false,
			FolderNameTemplate:  fmt.Sprintf("{%v}", TEMPLATE_TITLE_NAME),
			FileNameTemplate: fmt.Sprintf("{%v} [{%v}][{%v}][{%v}]", TEMPLATE_TITLE_NAME, TEMPLATE_DLC_NAME,
				TEMPLATE_TITLE_ID, TEMPLATE_VERSION),
			DeleteEmptyFolders:   true,
			DeleteOldUpdateFiles: false,
		},
	}
	return SaveSettings(settingsInstance)
}

func SaveSettings(settings *settings) *settings {
	file, _ := json.MarshalIndent(settings, "", " ")
	_ = ioutil.WriteFile(SETTINGS_FILENAME, file, 0644)
	settingsInstance = settings
	return settings
}
