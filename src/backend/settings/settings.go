package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mcuadros/go-version"
	"github.com/trembon/switch-library-manager/backend/dateformat"
	"go.uber.org/zap"
)

var (
	settingsInstance *AppSettings
	versionURL       = SLM_VERSION_URL
)

const (
	SETTINGS_FILENAME         = "settings.json"
	CACHE_FILENAME            = "cache.json"
	TITLE_JSON_FILENAME       = "titles.json"
	VERSIONS_JSON_FILENAME    = "versions.json"
	SLM_VERSION               = "2.1.0"
	SETTINGS_SCHEMA_VERSION   = 2
	DEFAULT_TITLES_JSON_URL   = "https://github.com/trembon/switch-library-manager/releases/download/data/titles.json"
	DEFAULT_VERSIONS_JSON_URL = "https://github.com/trembon/switch-library-manager/releases/download/data/versions.json"
	SLM_VERSION_URL           = "https://raw.githubusercontent.com/trembon/switch-library-manager/master/version.json"
)

const (
	TEMPLATE_TITLE_ID         = "TITLE_ID"
	TEMPLATE_TITLE_NAME       = "TITLE_NAME"
	TEMPLATE_DLC_NAME         = "DLC_NAME"
	TEMPLATE_VERSION          = "VERSION"
	TEMPLATE_REGION           = "REGION"
	TEMPLATE_VERSION_TXT      = "VERSION_TXT"
	TEMPLATE_TYPE             = "TYPE"
	TEMPLATE_SIZE_GB          = "SIZE_GB"
	TEMPLATE_SIZE_MB          = "SIZE_MB"
	TEMPLATE_PACKAGE_CONTENTS = "PACKAGE_CONTENTS"
)

const (
	ThemeInherit = "inherit"
	ThemeLight   = "light"
	ThemeDark    = "dark"
)

type GUISettings struct {
	Enabled             bool   `json:"enabled"`
	PageSize            int    `json:"page_size"`
	HideMissingGames    bool   `json:"hide_missing_games"`
	HideDemoGames       bool   `json:"hide_demo_games"`
	HideVersionString   bool   `json:"hide_version_string"`
	RememberWindowState bool   `json:"remember_window_state"`
	Theme               string `json:"theme"`
	DateFormat          string `json:"date_format"`
}

type PathSettings struct {
	LibraryFolder string   `json:"library_folder"`
	ScanFolders   []string `json:"scan_folders"`
	ProdKeys      string   `json:"prod_keys"`
}

type ScanSettings struct {
	Recursive                  bool     `json:"recursive"`
	RescanOnStartup            bool     `json:"rescan_on_startup"`
	IgnoreFileTypes            []string `json:"ignore_file_types"`
	IgnoreUnsupportedFileTypes bool     `json:"ignore_unsupported_file_types"`
}

type MissingContentSettings struct {
	CheckForUpdates   bool     `json:"check_for_updates"`
	CheckForDLC       bool     `json:"check_for_dlc"`
	IgnoreDLCUpdates  bool     `json:"ignore_dlc_updates"`
	IgnoreDLCTitleIDs []string `json:"ignore_dlc_title_ids"`
	IgnoreUpdateIDs   []string `json:"ignore_update_title_ids"`
}

type DataSourceSettings struct {
	TitlesURL   string `json:"titles_url"`
	VersionsURL string `json:"versions_url"`
}

type LoggingSettings struct {
	Debug bool `json:"debug"`
}

type OrganizationSettings struct {
	CreateFolderPerGame        bool   `json:"create_folder_per_game"`
	MoveScanFilesToLibrary     bool   `json:"move_scan_files_to_library"`
	DlcFolder                  string `json:"dlc_folder"`
	UpdatesFolder              string `json:"updates_folder"`
	RenameFiles                bool   `json:"rename_files"`
	DeleteEmptyFolders         bool   `json:"delete_empty_folders"`
	DeleteOldUpdateFiles       bool   `json:"delete_old_update_files"`
	FolderNameTemplate         string `json:"folder_name_template"`
	AllowUnicodeFileNames      bool   `json:"allow_unicode_file_names"`
	FileNameTemplate           string `json:"file_name_template"`
	IncludeBaseType            bool   `json:"include_base_type"`
	ProcessWhenMissingBaseGame bool   `json:"process_when_missing_base_game"`
}

// UnmarshalJSON accepts the deprecated, inverse switch_safe_file_names option.
// Only allow_unicode_file_names is emitted when settings are saved.
func (options *OrganizationSettings) UnmarshalJSON(data []byte) error {
	type plain OrganizationSettings
	decoded := struct {
		*plain
		AllowUnicodeFileNames *bool `json:"allow_unicode_file_names"`
		SwitchSafeFileNames   *bool `json:"switch_safe_file_names"`
	}{plain: (*plain)(options)}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("decode organization settings: %w", err)
	}
	if decoded.AllowUnicodeFileNames != nil {
		options.AllowUnicodeFileNames = *decoded.AllowUnicodeFileNames
	} else if decoded.SwitchSafeFileNames != nil {
		options.AllowUnicodeFileNames = !*decoded.SwitchSafeFileNames
	}
	return nil
}

// OrganizeOptions is retained as an alias for the process package API.
type OrganizeOptions = OrganizationSettings

type AppSettings struct {
	SchemaVersion  int                    `json:"schema_version"`
	GUI            GUISettings            `json:"gui"`
	Paths          PathSettings           `json:"paths"`
	Scan           ScanSettings           `json:"scan"`
	Organization   OrganizationSettings   `json:"organization"`
	MissingContent MissingContentSettings `json:"missing_content"`
	DataSources    DataSourceSettings     `json:"data_sources"`
	Logging        LoggingSettings        `json:"logging"`
}

type Cache struct {
	TitlesETag     string `json:"titles_etag"`
	VersionsETag   string `json:"versions_etag"`
	TitlesURL      string `json:"titles_url"`
	VersionsURL    string `json:"versions_url"`
	TitlesSHA256   string `json:"titles_sha256"`
	VersionsSHA256 string `json:"versions_sha256"`
}

// MigrationInfo describes an older settings file that was preserved while
// creating a current settings file with defaults.
type MigrationInfo struct {
	BackupPath string
}

// SettingsPreparation contains the settings loaded for this process and any
// migration notice that should be shown to the user.
type SettingsPreparation struct {
	Settings  *AppSettings
	Migration *MigrationInfo
}

func ReadSettingsAsJSON(baseFolder string) (string, error) {
	settings, err := ReadSettings(baseFolder)
	if err != nil {
		return "", err
	}
	bytes, err := json.MarshalIndent(settings, "", " ")
	if err != nil {
		return "", fmt.Errorf("marshal settings: %w", err)
	}
	return string(bytes), nil
}

func ReadSettings(baseFolder string) (*AppSettings, error) {
	prepared, err := PrepareSettings(baseFolder)
	if err != nil {
		return nil, err
	}
	return prepared.Settings, nil
}

// PrepareSettings loads current settings and reports whether an older file was
// preserved while defaults were generated. It is intended for application
// startup, where the caller can present the migration notice to the user.
func PrepareSettings(baseFolder string) (*SettingsPreparation, error) {
	if settingsInstance != nil {
		return &SettingsPreparation{Settings: settingsInstance}, nil
	}

	filename := filepath.Join(baseFolder, SETTINGS_FILENAME)
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		defaults := defaultSettings()
		if err := SaveSettingsWithError(defaults, baseFolder); err != nil {
			return nil, err
		}
		settingsInstance = defaults
		return &SettingsPreparation{Settings: settingsInstance}, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat settings: %w", err)
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open settings: %w", err)
	}

	contents, err := io.ReadAll(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("read settings: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close settings: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(contents, &raw); err != nil {
		return nil, fmt.Errorf("decode settings: %w; the file was not changed, create a current settings.json", err)
	}
	if raw == nil {
		return nil, errors.New("settings.json must contain a JSON object; the file was not changed")
	}

	var envelope struct {
		SchemaVersion *int `json:"schema_version"`
	}
	if err := json.Unmarshal(contents, &envelope); err != nil {
		return nil, fmt.Errorf("decode settings: %w; the file was not changed, create a current settings.json", err)
	}
	if envelope.SchemaVersion == nil || *envelope.SchemaVersion < SETTINGS_SCHEMA_VERSION {
		backupPath, err := preserveOldSettings(filename)
		if err != nil {
			return nil, err
		}

		defaults := defaultSettings()
		if err := SaveSettingsWithError(defaults, baseFolder); err != nil {
			if restoreErr := restoreOldSettings(filename, backupPath); restoreErr != nil {
				return nil, fmt.Errorf("create default settings: %w; restore older settings: %v", err, restoreErr)
			}
			return nil, fmt.Errorf("create default settings: %w; the older settings were restored", err)
		}
		return &SettingsPreparation{
			Settings:  defaults,
			Migration: &MigrationInfo{BackupPath: backupPath},
		}, nil
	}
	if *envelope.SchemaVersion != SETTINGS_SCHEMA_VERSION {
		return nil, fmt.Errorf("unsupported settings schema_version %d; current schema_version is %d; the file was not changed", *envelope.SchemaVersion, SETTINGS_SCHEMA_VERSION)
	}

	loaded := defaultSettings()
	if err := json.Unmarshal(contents, loaded); err != nil {
		return nil, fmt.Errorf("decode settings: %w; the file was not changed", err)
	}
	verifySettings(loaded)
	if _, err := dateformat.New(loaded.GUI.DateFormat); err != nil {
		return nil, fmt.Errorf("gui.date_format: %w; the file was not changed", err)
	}
	settingsInstance = loaded
	return &SettingsPreparation{Settings: settingsInstance}, nil
}

func preserveOldSettings(filename string) (string, error) {
	directory := filepath.Dir(filename)
	backupPath := filepath.Join(directory, "settings.old.json")
	for suffix := 1; ; suffix++ {
		if suffix > 1 {
			backupPath = filepath.Join(directory, fmt.Sprintf("settings.old.%d.json", suffix-1))
		}
		_, err := os.Stat(backupPath)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("check older settings backup %q: %w", backupPath, err)
		}
	}
	if err := os.Rename(filename, backupPath); err != nil {
		return "", fmt.Errorf("preserve older settings as %q: %w", backupPath, err)
	}
	return backupPath, nil
}

func restoreOldSettings(filename, backupPath string) error {
	if _, err := os.Stat(filename); err == nil {
		if err := os.Remove(filename); err != nil {
			return fmt.Errorf("remove incomplete settings file: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check incomplete settings file: %w", err)
	}
	if err := os.Rename(backupPath, filename); err != nil {
		return fmt.Errorf("rename %q back to %q: %w", backupPath, filename, err)
	}
	return nil
}

func defaultSettings() *AppSettings {
	return &AppSettings{
		SchemaVersion: SETTINGS_SCHEMA_VERSION,
		GUI: GUISettings{
			Enabled:    true,
			PageSize:   100,
			Theme:      ThemeInherit,
			DateFormat: dateformat.Default,
		},
		Paths: PathSettings{ScanFolders: []string{}},
		Scan:  ScanSettings{Recursive: true, IgnoreFileTypes: []string{}},
		Organization: OrganizationSettings{
			FolderNameTemplate:    fmt.Sprintf("{%v}", TEMPLATE_TITLE_NAME),
			FileNameTemplate:      fmt.Sprintf("{%v} ({%v})[{%v}][v{%v}]", TEMPLATE_TITLE_NAME, TEMPLATE_DLC_NAME, TEMPLATE_TITLE_ID, TEMPLATE_VERSION),
			AllowUnicodeFileNames: false,
			IncludeBaseType:       true,
		},
		MissingContent: MissingContentSettings{
			CheckForUpdates:   true,
			CheckForDLC:       true,
			IgnoreDLCTitleIDs: []string{"01007F600B135007"},
			IgnoreUpdateIDs:   []string{},
		},
		DataSources: DataSourceSettings{
			TitlesURL:   DEFAULT_TITLES_JSON_URL,
			VersionsURL: DEFAULT_VERSIONS_JSON_URL,
		},
	}
}

func verifySettings(settings *AppSettings) {
	settings.GUI.Theme = NormalizeTheme(settings.GUI.Theme)
	settings.GUI.DateFormat = dateformat.Normalize(settings.GUI.DateFormat)
	if settings.DataSources.TitlesURL == "" {
		settings.DataSources.TitlesURL = DEFAULT_TITLES_JSON_URL
	}
	if settings.DataSources.VersionsURL == "" {
		settings.DataSources.VersionsURL = DEFAULT_VERSIONS_JSON_URL
	}
	if settings.GUI.PageSize <= 0 {
		settings.GUI.PageSize = 100
	}
	if settings.Paths.ScanFolders == nil {
		settings.Paths.ScanFolders = []string{}
	}
	if settings.Scan.IgnoreFileTypes == nil {
		settings.Scan.IgnoreFileTypes = []string{}
	}
	if settings.MissingContent.IgnoreDLCTitleIDs == nil {
		settings.MissingContent.IgnoreDLCTitleIDs = []string{}
	}
	if settings.MissingContent.IgnoreUpdateIDs == nil {
		settings.MissingContent.IgnoreUpdateIDs = []string{}
	}
}

// NormalizeTheme returns a supported GUI theme value. Empty and unknown
// values intentionally follow the system preference.
func NormalizeTheme(theme string) string {
	switch theme {
	case ThemeLight, ThemeDark:
		return theme
	default:
		return ThemeInherit
	}
}

func defaultCache() *Cache {
	return &Cache{}
}

func ReadCache(baseFolder string) (*Cache, error) {
	file, err := os.Open(filepath.Join(baseFolder, CACHE_FILENAME))
	if errors.Is(err, os.ErrNotExist) {
		return defaultCache(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("open cache: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read cache: %w", err)
	}
	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		zap.S().Warnf("ignoring malformed cache file %q: %v", filepath.Join(baseFolder, CACHE_FILENAME), err)
		return defaultCache(), nil
	}
	return &cache, nil
}

func SaveCacheWithError(cache *Cache, baseFolder string) error {
	if cache == nil {
		return errors.New("cache is nil")
	}
	data, err := json.MarshalIndent(cache, "", " ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	if err := os.WriteFile(filepath.Join(baseFolder, CACHE_FILENAME), data, 0644); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	return nil
}

func SaveSettings(settings *AppSettings, baseFolder string) *AppSettings {
	if err := SaveSettingsWithError(settings, baseFolder); err != nil {
		zap.S().Errorf("Failed to save settings: %v", err)
	}
	settingsInstance = settings
	return settings
}

func SaveSettingsWithError(settings *AppSettings, baseFolder string) error {
	if err := ValidateSettings(settings); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", " ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := writeSettingsFile(filepath.Join(baseFolder, SETTINGS_FILENAME), data); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	settingsInstance = settings
	return nil
}

// ValidateSettings normalizes defaults and validates a candidate without saving
// or publishing it. Callers can validate before invalidating dependent caches.
func ValidateSettings(settings *AppSettings) error {
	if settings == nil {
		return errors.New("settings are nil")
	}
	if settings.SchemaVersion == 0 {
		settings.SchemaVersion = SETTINGS_SCHEMA_VERSION
	}
	if settings.SchemaVersion != SETTINGS_SCHEMA_VERSION {
		return fmt.Errorf("unsupported settings schema_version %d", settings.SchemaVersion)
	}
	verifySettings(settings)
	return validateSettings(settings)
}

func validateSettings(settings *AppSettings) error {
	if _, err := dateformat.New(settings.GUI.DateFormat); err != nil {
		return fmt.Errorf("gui.date_format: %w", err)
	}
	if err := validateDataSourceURL("titles_url", settings.DataSources.TitlesURL); err != nil {
		return err
	}
	if err := validateDataSourceURL("versions_url", settings.DataSources.VersionsURL); err != nil {
		return err
	}
	for _, id := range append(append([]string{}, settings.MissingContent.IgnoreDLCTitleIDs...), settings.MissingContent.IgnoreUpdateIDs...) {
		if id == "" {
			continue
		}
		if len(id) != 16 {
			return fmt.Errorf("title ID %q must contain exactly 16 hexadecimal characters", id)
		}
		for _, char := range id {
			if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
				return fmt.Errorf("title ID %q must contain only hexadecimal characters", id)
			}
		}
	}
	if settings.Organization.RenameFiles && !containsTitleIdentity(settings.Organization.FileNameTemplate) {
		return errors.New("file_name_template must contain {TITLE_NAME} or {TITLE_ID} when file renaming is enabled")
	}
	if settings.Organization.CreateFolderPerGame && !containsTitleIdentity(settings.Organization.FolderNameTemplate) {
		return errors.New("folder_name_template must contain {TITLE_NAME} or {TITLE_ID} when game folders are enabled")
	}
	return nil
}

func validateDataSourceURL(name, value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("data_sources.%s must be an absolute HTTP or HTTPS URL", name)
	}
	return nil
}

func containsTitleIdentity(template string) bool {
	return strings.Contains(template, "{"+TEMPLATE_TITLE_NAME+"}") || strings.Contains(template, "{"+TEMPLATE_TITLE_ID+"}")
}

func writeSettingsFile(filename string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".settings.json.tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary settings file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	if err := temporary.Chmod(0644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary settings permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary settings file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary settings file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary settings file: %w", err)
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		// Windows does not replace an existing file during Rename. Removing the
		// old file is the platform fallback; the normal path remains an atomic
		// same-directory replacement on platforms that support it.
		if runtime.GOOS != "windows" {
			return fmt.Errorf("replace settings file: %w", err)
		}
		if removeErr := os.Remove(filename); removeErr != nil {
			return fmt.Errorf("replace settings file: %w; remove existing file: %v", err, removeErr)
		}
		if renameErr := os.Rename(temporaryName, filename); renameErr != nil {
			return fmt.Errorf("replace settings file after removing existing file: %w", renameErr)
		}
	}
	return nil
}

func CheckForUpdates() (bool, error) {
	localVer := SLM_VERSION
	res, err := http.Get(versionURL)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return false, fmt.Errorf("version check returned %s", res.Status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}
	remoteValues := map[string]string{}
	if err := json.Unmarshal(body, &remoteValues); err != nil {
		return false, err
	}
	remoteVer := remoteValues["version"]
	if remoteVer == "" {
		return false, errors.New("version check response does not contain a version")
	}
	return version.CompareSimple(remoteVer, localVer) > 0, nil
}
