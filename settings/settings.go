package settings

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

const (
	SETTINGS_DIR           = "switch-library-manager"
	SETTINGS_FILENAME      = "settings.json"
	TITLE_JSON_FILENAME    = "titles.json"
	VERSIONS_JSON_FILENAME = "versions.json"
	SLM_VERSION            = "1.5.0"
	TITLES_JSON_URL        = "https://tinfoil.media/repo/db/titles.json"
	//TITLES_JSON_URL    = "https://raw.githubusercontent.com/blawar/titledb/master/titles.US.en.json"
	VERSIONS_JSON_URL = "https://tinfoil.media/repo/db/versions.json"
	//VERSIONS_JSON_URL = "https://raw.githubusercontent.com/blawar/titledb/master/versions.json"
)

const (
	TEMPLATE_TITLE_ID    = "TITLE_ID"
	TEMPLATE_TITLE_NAME  = "TITLE_NAME"
	TEMPLATE_DLC_NAME    = "DLC_NAME"
	TEMPLATE_VERSION     = "VERSION"
	TEMPLATE_REGION      = "REGION"
	TEMPLATE_VERSION_TXT = "VERSION_TXT"
	TEMPLATE_TYPE        = "TYPE"
)

// Setting of the application
type AppSettings struct {
	// Extra internal settings
	// `json:"-"` to ignore when marshalling
	baseFolder string            `json:"-"`
	Homedir    string            `string:"-"`
	SwitchKeys map[string]string `json:"-"`
	// Unmarshalled from the JSON file
	VersionsEtag           string          `json:"versions_etag"`
	TitlesEtag             string          `json:"titles_etag"`
	Prodkeys               string          `json:"prod_keys"`
	Folder                 string          `json:"folder"`
	ScanFolders            []string        `json:"scan_folders"`
	GUI                    bool            `json:"gui"`
	Debug                  bool            `json:"debug"`
	CheckForMissingUpdates bool            `json:"check_for_missing_updates"`
	CheckForMissingDLC     bool            `json:"check_for_missing_dlc"`
	OrganizeOptions        OrganizeOptions `json:"organize_options"`
	ScanRecursively        bool            `json:"scan_recursively"`
	GuiPagingSize          int             `json:"gui_page_size"`
	IgnoreDLCTitleIds      []string        `json:"ignore_dlc_title_ids"`
	DBInHomedir            bool            `json:"db_in_homedir"`
}

// Organization settings of the application
type OrganizeOptions struct {
	CreateFolderPerGame  bool   `json:"create_folder_per_game"`
	RenameFiles          bool   `json:"rename_files"`
	DeleteEmptyFolders   bool   `json:"delete_empty_folders"`
	DeleteOldUpdateFiles bool   `json:"delete_old_update_files"`
	FolderNameTemplate   string `json:"folder_name_template"`
	SwitchSafeFileNames  bool   `json:"switch_safe_file_names"`
	FileNameTemplate     string `json:"file_name_template"`
}

// Constructor for settings
func NewAppSettings(workingFolder string) *AppSettings {
	a := AppSettings{}
	a.setBase(workingFolder)
	a.switchToHomedir()
	a.read()

	a.SwitchKeys = make(map[string]string)

	return &a
}

// Set the base bolder
func (a *AppSettings) setBase(base string) {
	a.baseFolder = base
}

// Switch the settings base folder inside the homedir
func (a *AppSettings) switchToHomedir() {
	var homedirErr error
	a.Homedir, homedirErr = os.UserHomeDir()

	if homedirErr == nil {
		basedir := a.GetHomedirPath()

		// Create a folder if it does not exist
		if mkDirErr := os.MkdirAll(basedir, os.ModePerm); mkDirErr == nil {
			// Change the base
			a.setBase(basedir)
		}
	}
}

// Get the homedir settings path
func (a *AppSettings) GetHomedirPath() string {
	return filepath.Join(a.Homedir, SETTINGS_DIR)
}

// Get the settings file path
func (a *AppSettings) getPath() string {
	return filepath.Join(a.baseFolder, SETTINGS_FILENAME)
}

// Read the file
func (a *AppSettings) read() {
	// Reading the file
	buf, bufErr := ioutil.ReadFile(a.getPath())

	// If error fill with defaults
	if bufErr != nil {
		zap.S().Warnf("Missing or corrupted config file, creating a new one.")
		a.defaults()
		a.Save()
	} else {
		// Otherwise unmarshal it
		if jsonErr := a.Load(buf); jsonErr != nil {
			zap.S().Warnf("Missing or corrupted config file, creating a new one.")
			a.defaults()
			a.Save()
		}
	}
}

// Fill the structure with default values
func (a *AppSettings) defaults() {
	a.VersionsEtag = "W/\"2ef50d1cb6bd61:0\""
	a.TitlesEtag = "W/\"a5b02845cf6bd61:0\""
	a.ScanFolders = []string{}
	a.GUI = true
	a.CheckForMissingUpdates = true
	a.CheckForMissingDLC = true
	a.OrganizeOptions.FolderNameTemplate = fmt.Sprintf("{%v}", TEMPLATE_TITLE_NAME)
	a.OrganizeOptions.FileNameTemplate = fmt.Sprintf("{%v} ({%v})[{%v}][v{%v}]",
		TEMPLATE_TITLE_NAME,
		TEMPLATE_DLC_NAME,
		TEMPLATE_TITLE_ID,
		TEMPLATE_VERSION,
	)
	a.OrganizeOptions.SwitchSafeFileNames = true
	a.ScanRecursively = true
	a.GuiPagingSize = 100
	a.IgnoreDLCTitleIds = []string{}
}

// Save to file (ignore errors)
func (a *AppSettings) Save() {
	// Marshal the struct into JSON bytes
	jsonBytes, jsonErr := json.MarshalIndent(a, "", "  ")
	if jsonErr == nil {
		// Write the file
		ioutil.WriteFile(a.getPath(), jsonBytes, 0644)
	}
}

// Return setting as JSON
func (a *AppSettings) ToJSON() string {
	// Marshal the struct into JSON bytes
	jsonBytes, jsonErr := json.MarshalIndent(a, "", "  ")
	if jsonErr != nil {
		return ""
	}

	return string(jsonBytes)
}

// Load a JSON payload
func (a *AppSettings) Load(payload []byte) error {
	jsonErr := json.Unmarshal(payload, a)
	if jsonErr != nil {
		return jsonErr
	}

	return nil
}
