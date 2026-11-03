package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolateSettings(t *testing.T) {
	t.Helper()
	oldSettings, oldVersionURL, oldKeys := settingsInstance, versionURL, keysInstance
	settingsInstance = nil
	keysInstance = nil
	versionURL = SLM_VERSION_URL
	t.Cleanup(func() {
		settingsInstance, versionURL, keysInstance = oldSettings, oldVersionURL, oldKeys
	})
}

func TestDefaultSettingsAndJSON(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	s, err := ReadSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if s.SchemaVersion != SETTINGS_SCHEMA_VERSION || !s.GUI.Enabled || s.GUI.RememberWindowState || s.GUI.HideVersionString || !s.Scan.Recursive || s.Scan.RescanOnStartup || s.Scan.IgnoreUnsupportedFileTypes || !s.MissingContent.CheckForUpdates || !s.MissingContent.CheckForDLC || s.GUI.PageSize != 100 || s.GUI.Theme != ThemeInherit {
		t.Fatalf("unexpected defaults: %#v", s)
	}
	if s.DataSources.TitlesURL != DEFAULT_TITLES_JSON_URL || s.DataSources.VersionsURL != DEFAULT_VERSIONS_JSON_URL || s.Organization.AllowUnicodeFileNames || !s.Organization.IncludeBaseType || s.Organization.MoveScanFilesToLibrary {
		t.Fatalf("unexpected default URLs/options: %#v", s)
	}
	if s.Organization.FileNameTemplate != "{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]" {
		t.Fatalf("default file-name template changed unexpectedly: %q", s.Organization.FileNameTemplate)
	}
	if _, err := os.Stat(filepath.Join(base, SETTINGS_FILENAME)); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadSettingsAsJSON(base)
	if err != nil {
		t.Fatal(err)
	}
	var decoded AppSettings
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != SETTINGS_SCHEMA_VERSION || decoded.Organization.FileNameTemplate == "" {
		t.Fatalf("invalid default JSON: %#v", decoded)
	}
}

func TestMoveScanFilesToLibrarySettingRoundTripsAndDefaultsOff(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	options, err := ReadSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if options.Organization.MoveScanFilesToLibrary {
		t.Fatal("move_scan_files_to_library should default to false")
	}
	options.Organization.MoveScanFilesToLibrary = true
	if err := SaveSettingsWithError(options, base); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Organization.MoveScanFilesToLibrary {
		t.Fatal("move_scan_files_to_library did not survive settings persistence")
	}
}

func TestUnicodeFileNamesSettingsCompatibility(t *testing.T) {
	for _, test := range []struct {
		name    string
		options string
		want    bool
	}{
		{"missing field", `{}`, false},
		{"legacy enabled", `{"switch_safe_file_names":true}`, false},
		{"legacy disabled", `{"switch_safe_file_names":false}`, true},
		{"unicode enabled", `{"allow_unicode_file_names":true}`, true},
		{"unicode disabled", `{"allow_unicode_file_names":false}`, false},
		{"unicode true overrides legacy true", `{"allow_unicode_file_names":true,"switch_safe_file_names":true}`, true},
		{"unicode false overrides legacy false", `{"switch_safe_file_names":false,"allow_unicode_file_names":false}`, false},
		{"both allow unicode", `{"switch_safe_file_names":false,"allow_unicode_file_names":true}`, true},
		{"both restrict unicode", `{"allow_unicode_file_names":false,"switch_safe_file_names":true}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateSettings(t)
			base := t.TempDir()
			filename := filepath.Join(base, SETTINGS_FILENAME)
			source := []byte(`{"schema_version":2,"gui":{"page_size":37},"organization":` + test.options + `}`)
			if err := os.WriteFile(filename, source, 0644); err != nil {
				t.Fatal(err)
			}
			loaded, err := ReadSettings(base)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Organization.AllowUnicodeFileNames != test.want || loaded.SchemaVersion != 2 || loaded.GUI.PageSize != 37 || !loaded.Organization.IncludeBaseType || loaded.Organization.FileNameTemplate == "" {
				t.Fatalf("unexpected settings after decoding: %#v", loaded)
			}
			contents, err := os.ReadFile(filename)
			if err != nil || string(contents) != string(source) {
				t.Fatalf("loading changed settings: %q, error: %v", contents, err)
			}
			loaded.Organization.AllowUnicodeFileNames = !test.want
			if err := SaveSettingsWithError(loaded, base); err != nil {
				t.Fatal(err)
			}
			contents, err = os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(contents), "switch_safe_file_names") || !strings.Contains(string(contents), "allow_unicode_file_names") {
				t.Fatalf("saved settings must contain only the canonical option: %s", contents)
			}
			settingsInstance = nil
			reloaded, err := ReadSettings(base)
			if err != nil {
				t.Fatal(err)
			}
			if reloaded.Organization.AllowUnicodeFileNames != !test.want || reloaded.GUI.PageSize != 37 {
				t.Fatalf("saved Unicode choice or unrelated settings did not survive reload: %#v", reloaded)
			}
		})
	}
}

func TestUnicodeFileNamesRejectsNonBooleanSettings(t *testing.T) {
	for _, field := range []string{"allow_unicode_file_names", "switch_safe_file_names"} {
		t.Run(field, func(t *testing.T) {
			var options OrganizationSettings
			if err := json.Unmarshal([]byte(`{"`+field+`":"true"}`), &options); err == nil {
				t.Fatal("non-boolean filename setting was accepted")
			}
		})
	}
}

func TestIncludeBaseTypeDefaultsAndPersistence(t *testing.T) {
	for _, test := range []struct {
		name string
		json string
		want bool
	}{
		{"missing field", `{"schema_version":2,"organization":{"rename_files":true,"file_name_template":"{TITLE_NAME}[{TYPE}]"}}`, true},
		{"enabled", `{"schema_version":2,"organization":{"include_base_type":true}}`, true},
		{"disabled", `{"schema_version":2,"organization":{"include_base_type":false}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateSettings(t)
			base := t.TempDir()
			if err := os.WriteFile(filepath.Join(base, SETTINGS_FILENAME), []byte(test.json), 0644); err != nil {
				t.Fatal(err)
			}
			loaded, err := ReadSettings(base)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Organization.IncludeBaseType != test.want {
				t.Fatalf("include_base_type = %v, want %v", loaded.Organization.IncludeBaseType, test.want)
			}
			loaded.Organization.IncludeBaseType = !test.want
			if err := SaveSettingsWithError(loaded, base); err != nil {
				t.Fatal(err)
			}
			settingsInstance = nil
			reloaded, err := ReadSettings(base)
			if err != nil {
				t.Fatal(err)
			}
			if reloaded.Organization.IncludeBaseType != !test.want {
				t.Fatalf("include_base_type after reloading = %v, want %v", reloaded.Organization.IncludeBaseType, !test.want)
			}
		})
	}
}

func TestRememberWindowStateDefaultsOffForExistingSettings(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, SETTINGS_FILENAME), []byte(`{"schema_version":2,"gui":{"enabled":true}}`), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := ReadSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GUI.RememberWindowState || loaded.GUI.HideVersionString || loaded.Scan.IgnoreUnsupportedFileTypes || loaded.Scan.RescanOnStartup {
		t.Fatal("existing settings without opt-in settings should default to false")
	}
}

func TestExistingSettingsURLsArePreserved(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	legacy := defaultSettings()
	legacy.DataSources.TitlesURL = "https://tinfoil.io/repo/db/titles.json"
	legacy.DataSources.VersionsURL = "https://raw.githubusercontent.com/blawar/titledb/master/versions.json"
	if err := SaveSettingsWithError(legacy, base); err != nil {
		t.Fatal(err)
	}
	settingsInstance = nil
	loaded, err := ReadSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DataSources.TitlesURL != legacy.DataSources.TitlesURL || loaded.DataSources.VersionsURL != legacy.DataSources.VersionsURL {
		t.Fatalf("existing source URLs changed: %#v", loaded.DataSources)
	}
}

func TestThemeSettingsNormalizeAndRoundTrip(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	custom := &AppSettings{GUI: GUISettings{Theme: ThemeDark}}
	if err := SaveSettingsWithError(custom, base); err != nil {
		t.Fatal(err)
	}
	if custom.GUI.Theme != ThemeDark {
		t.Fatalf("dark theme was changed: %q", custom.GUI.Theme)
	}

	settingsInstance = nil
	loaded, err := ReadSettings(base)
	if err != nil || loaded.GUI.Theme != ThemeDark {
		t.Fatalf("dark theme round-trip: %#v, %v", loaded, err)
	}

	settingsInstance = nil
	custom.GUI.Theme = ThemeLight
	if err := SaveSettingsWithError(custom, base); err != nil {
		t.Fatal(err)
	}
	settingsInstance = nil
	loaded, err = ReadSettings(base)
	if err != nil || loaded.GUI.Theme != ThemeLight {
		t.Fatalf("light theme round-trip: %#v, %v", loaded, err)
	}

	settingsInstance = nil
	if err := os.WriteFile(filepath.Join(base, SETTINGS_FILENAME), []byte(`{"schema_version":2}`), 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err = ReadSettings(base)
	if err != nil || loaded.GUI.Theme != ThemeInherit {
		t.Fatalf("missing theme compatibility: %#v, %v", loaded, err)
	}

	settingsInstance = nil
	if err := os.WriteFile(filepath.Join(base, SETTINGS_FILENAME), []byte(`{"schema_version":2,"gui":{"theme":"invalid"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err = ReadSettings(base)
	if err != nil || loaded.GUI.Theme != ThemeInherit {
		t.Fatalf("invalid theme normalization: %#v, %v", loaded, err)
	}
}

func TestSettingsSaveRoundTrip(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	custom := &AppSettings{
		SchemaVersion: SETTINGS_SCHEMA_VERSION,
		DataSources:   DataSourceSettings{TitlesURL: "https://titles.example", VersionsURL: "https://versions.example"},
		Paths:         PathSettings{LibraryFolder: "library", ScanFolders: []string{"one", "two"}},
		Scan:          ScanSettings{IgnoreFileTypes: []string{"txt"}, IgnoreUnsupportedFileTypes: true, RescanOnStartup: true},
		GUI:           GUISettings{PageSize: 25, RememberWindowState: true, HideVersionString: true},
		Organization:  OrganizationSettings{FileNameTemplate: "{TITLE_ID}"},
	}
	if err := SaveSettingsWithError(custom, base); err != nil {
		t.Fatal(err)
	}
	current, err := ReadSettings(base)
	if err != nil || current != custom {
		t.Fatalf("save did not refresh settings instance: %#v, %v", current, err)
	}
	settingsInstance = nil
	loaded, err := ReadSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Paths.LibraryFolder != "library" || loaded.GUI.PageSize != 25 || !loaded.GUI.RememberWindowState || !loaded.GUI.HideVersionString || len(loaded.Paths.ScanFolders) != 2 {
		t.Fatalf("round-trip mismatch: %#v", loaded)
	}
	if loaded.DataSources.TitlesURL != custom.DataSources.TitlesURL || loaded.DataSources.VersionsURL != custom.DataSources.VersionsURL {
		t.Fatal("custom URLs were not retained")
	}
	if loaded.Organization.FileNameTemplate != "{TITLE_ID}" {
		t.Fatal("custom organization options were not retained")
	}
	if !loaded.Scan.IgnoreUnsupportedFileTypes {
		t.Fatal("ignore_unsupported_file_types was not retained")
	}
	if !loaded.Scan.RescanOnStartup {
		t.Fatal("rescan_on_startup was not retained")
	}
}

func TestSaveSettingsValidatesAndReplacesFile(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()

	invalidURL := defaultSettings()
	invalidURL.DataSources.TitlesURL = "ftp://titles.example/titles.json"
	if err := SaveSettingsWithError(invalidURL, base); err == nil || !strings.Contains(err.Error(), "absolute HTTP or HTTPS URL") {
		t.Fatalf("invalid URL error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, SETTINGS_FILENAME)); !os.IsNotExist(err) {
		t.Fatalf("invalid settings created a file, stat error = %v", err)
	}

	invalidID := defaultSettings()
	invalidID.MissingContent.IgnoreUpdateIDs = []string{"not-a-title-id"}
	if err := SaveSettingsWithError(invalidID, base); err == nil || !strings.Contains(err.Error(), "exactly 16 hexadecimal") {
		t.Fatalf("invalid title ID error = %v", err)
	}

	invalidTemplate := defaultSettings()
	invalidTemplate.Organization.RenameFiles = true
	invalidTemplate.Organization.FileNameTemplate = "{VERSION}"
	if err := SaveSettingsWithError(invalidTemplate, base); err == nil || !strings.Contains(err.Error(), "file_name_template") {
		t.Fatalf("invalid template error = %v", err)
	}

	valid := defaultSettings()
	valid.Paths.LibraryFolder = "first"
	if err := SaveSettingsWithError(valid, base); err != nil {
		t.Fatal(err)
	}
	valid.Paths.LibraryFolder = "second"
	if err := SaveSettingsWithError(valid, base); err != nil {
		t.Fatal(err)
	}
	loaded, err := os.ReadFile(filepath.Join(base, SETTINGS_FILENAME))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(loaded), `"library_folder": "second"`) {
		t.Fatalf("settings file was not replaced: %s", loaded)
	}
	temporary, err := filepath.Glob(filepath.Join(base, ".settings.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporary) != 0 {
		t.Fatalf("temporary settings files remain: %v", temporary)
	}
}

func TestSettingsMigratesOlderSettingsWithoutSchemaMarker(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	oldSettings := []byte(`{"folder":"old","gui":true}`)
	filename := filepath.Join(base, SETTINGS_FILENAME)
	if err := os.WriteFile(filename, oldSettings, 0644); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Migration == nil || filepath.Base(prepared.Migration.BackupPath) != "settings.old.json" {
		t.Fatalf("unexpected migration info: %#v", prepared.Migration)
	}
	contents, readErr := os.ReadFile(prepared.Migration.BackupPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(contents) != string(oldSettings) {
		t.Fatalf("older settings were changed: %s", contents)
	}
	generated, readErr := os.ReadFile(filename)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var generatedSettings AppSettings
	if err := json.Unmarshal(generated, &generatedSettings); err != nil {
		t.Fatal(err)
	}
	if generatedSettings.SchemaVersion != SETTINGS_SCHEMA_VERSION || generatedSettings.DataSources.TitlesURL == "" {
		t.Fatalf("invalid generated defaults: %#v", generatedSettings)
	}
}

func TestSettingsMigratesOlderSchemaAndUsesBackupSuffix(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	oldSettings := []byte(`{"schema_version":1,"folder":"old"}`)
	filename := filepath.Join(base, SETTINGS_FILENAME)
	if err := os.WriteFile(filename, oldSettings, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "settings.old.json"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(prepared.Migration.BackupPath) != "settings.old.1.json" {
		t.Fatalf("backup path = %q, want settings.old.1.json", prepared.Migration.BackupPath)
	}
	contents, err := os.ReadFile(prepared.Migration.BackupPath)
	if err != nil || string(contents) != string(oldSettings) {
		t.Fatalf("older settings backup = %q, err = %v", contents, err)
	}
	existing, err := os.ReadFile(filepath.Join(base, "settings.old.json"))
	if err != nil || string(existing) != "existing" {
		t.Fatalf("existing backup changed: %q, err = %v", existing, err)
	}
}

func TestSettingsRejectsUnsupportedSchemaAndMalformedJSON(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{name: "unsupported", body: `{"schema_version":3}`, want: "unsupported settings schema_version"},
		{name: "malformed", body: `{`, want: "decode settings"},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateSettings(t)
			base := t.TempDir()
			filename := filepath.Join(base, SETTINGS_FILENAME)
			if err := os.WriteFile(filename, []byte(test.body), 0644); err != nil {
				t.Fatal(err)
			}
			_, err := ReadSettings(base)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			unchanged, readErr := os.ReadFile(filename)
			if readErr != nil || string(unchanged) != test.body {
				t.Fatalf("settings changed: %q, err = %v", unchanged, readErr)
			}
		})
	}
}

func TestSettingsRejectsNonObjectJSONWithoutChangingFile(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	filename := filepath.Join(base, SETTINGS_FILENAME)
	contents := []byte("null")
	if err := os.WriteFile(filename, contents, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSettings(base); err == nil || !strings.Contains(err.Error(), "JSON object") {
		t.Fatalf("expected object error, got %v", err)
	}
	unchanged, err := os.ReadFile(filename)
	if err != nil || string(unchanged) != string(contents) {
		t.Fatalf("settings changed: %q, err = %v", unchanged, err)
	}
}

func TestRestoreOldSettings(t *testing.T) {
	base := t.TempDir()
	filename := filepath.Join(base, SETTINGS_FILENAME)
	backup := filepath.Join(base, "settings.old.json")
	if err := os.WriteFile(backup, []byte("old settings"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("incomplete"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := restoreOldSettings(filename, backup); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filename)
	if err != nil || string(contents) != "old settings" {
		t.Fatalf("restored settings = %q, err = %v", contents, err)
	}
}

func TestCacheRoundTrip(t *testing.T) {
	base := t.TempDir()
	cache, err := ReadCache(base)
	if err != nil || cache.TitlesETag != "" || cache.VersionsETag != "" {
		t.Fatalf("default cache: %#v, %v", cache, err)
	}
	cache.TitlesETag = "titles-etag"
	cache.VersionsETag = "versions-etag"
	cache.TitlesURL = "https://titles.example/titles.json"
	cache.VersionsURL = "https://versions.example/versions.json"
	cache.TitlesSHA256 = "titles-sha256"
	cache.VersionsSHA256 = "versions-sha256"
	if err := SaveCacheWithError(cache, base); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadCache(base)
	if err != nil || loaded.TitlesETag != "titles-etag" || loaded.VersionsETag != "versions-etag" || loaded.TitlesURL != cache.TitlesURL || loaded.VersionsURL != cache.VersionsURL || loaded.TitlesSHA256 != cache.TitlesSHA256 || loaded.VersionsSHA256 != cache.VersionsSHA256 {
		t.Fatalf("cache round-trip: %#v, %v", loaded, err)
	}
}

func TestReadCacheTreatsMalformedAndLegacyCacheAsMissingValidators(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, CACHE_FILENAME)
	if err := os.WriteFile(path, []byte(`{"titles_etag":`), 0644); err != nil {
		t.Fatal(err)
	}
	cache, err := ReadCache(base)
	if err != nil || cache.TitlesETag != "" || cache.VersionsETag != "" {
		t.Fatalf("malformed cache = %#v, err = %v", cache, err)
	}

	legacyCache := `{"titles_etag":"old-title","versions_etag":"old-version"}`
	if err := os.WriteFile(path, []byte(legacyCache), 0644); err != nil {
		t.Fatal(err)
	}
	cache, err = ReadCache(base)
	if err != nil || cache.TitlesETag != "old-title" || cache.VersionsETag != "old-version" || cache.TitlesURL != "" || cache.VersionsURL != "" || cache.TitlesSHA256 != "" || cache.VersionsSHA256 != "" {
		t.Fatalf("legacy cache = %#v, err = %v", cache, err)
	}
}

func TestCheckForUpdates(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		status     int
		wantUpdate bool
		wantError  bool
	}{
		{name: "newer", body: `{"version":"3.0.0"}`, wantUpdate: true},
		{name: "same", body: `{"version":"2.1.0"}`},
		{name: "older", body: `{"version":"1.0.0"}`},
		{name: "malformed", body: "[", wantError: true},
		{name: "missing version", body: `{}`, wantError: true},
		{name: "server error", body: "no", status: http.StatusBadGateway, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateSettings(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.status != 0 {
					w.WriteHeader(tt.status)
				}
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			versionURL = server.URL
			updated, err := CheckForUpdates()
			if tt.wantError != (err != nil) || updated != tt.wantUpdate {
				t.Fatalf("updated=%v err=%v", updated, err)
			}
		})
	}
	isolateSettings(t)
	versionURL = "http://127.0.0.1:1/unreachable"
	if _, err := CheckForUpdates(); err == nil {
		t.Fatal("expected network error")
	}
}

func TestKeyDiscoveryOrderAndMissingKeys(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	configuredDir := filepath.Join(base, "configured")
	if err := os.Mkdir(configuredDir, 0755); err != nil {
		t.Fatal(err)
	}
	configuredFile := filepath.Join(configuredDir, "prod.keys")
	if err := os.WriteFile(configuredFile, []byte("header_key = configured\nfoo = bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "prod.keys"), []byte("header_key = current\n"), 0644); err != nil {
		t.Fatal(err)
	}
	SaveSettings(&AppSettings{Paths: PathSettings{ProdKeys: configuredDir}}, base)
	keys, err := InitSwitchKeys(base)
	if err != nil || keys.GetKey("header_key") != "configured" || keys.GetKey("foo") != "bar" {
		t.Fatalf("configured key discovery: keys=%v err=%v", keys, err)
	}

	isolateSettings(t)
	SaveSettings(&AppSettings{Paths: PathSettings{ProdKeys: filepath.Join(base, "missing.keys")}}, base)
	keys, err = InitSwitchKeys(base)
	if err != nil || keys.GetKey("header_key") != "current" {
		t.Fatalf("current-folder fallback: keys=%v err=%v", keys, err)
	}

	isolateSettings(t)
	emptyBase := t.TempDir()
	SaveSettings(&AppSettings{}, emptyBase)
	keys, err = InitSwitchKeys(emptyBase)
	if err == nil || keys != nil || !strings.Contains(err.Error(), "prod.keys") {
		t.Fatalf("missing key result: keys=%v err=%v", keys, err)
	}
	if current, err := SwitchKeys(); err != nil || current != nil {
		t.Fatalf("unexpected singleton after failed discovery: %v, %v", current, err)
	}
}

func TestKeyDiscoveryHomeFallback(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	if err := os.Mkdir(filepath.Join(home, ".switch"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".switch", "prod.keys"), []byte("header_key = home\n"), 0644); err != nil {
		t.Fatal(err)
	}
	SaveSettings(&AppSettings{}, base)
	keys, err := InitSwitchKeys(base)
	if err != nil || keys.GetKey("header_key") != "home" {
		t.Fatalf("home key discovery: keys=%v err=%v", keys, err)
	}
}

func TestKeyDiscoveryBaseFolderBeforeHome(t *testing.T) {
	isolateSettings(t)
	dataFolder := t.TempDir()
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	if err := os.Mkdir(filepath.Join(home, ".switch"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".switch", "prod.keys"), []byte("header_key = home\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataFolder, "prod.keys"), []byte("header_key = data-folder\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveSettingsWithError(&AppSettings{}, dataFolder); err != nil {
		t.Fatal(err)
	}

	keys, err := InitSwitchKeys(dataFolder)
	if err != nil || keys.GetKey("header_key") != "data-folder" {
		t.Fatalf("data-folder key discovery: keys=%v err=%v", keys, err)
	}
}

func TestKeyFilePathCaseAndReadSettingsSingleton(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	keyFile := filepath.Join(base, "keys.KEYS")
	if err := os.WriteFile(keyFile, []byte("header_key = value\n"), 0644); err != nil {
		t.Fatal(err)
	}
	SaveSettings(&AppSettings{Paths: PathSettings{ProdKeys: keyFile}}, base)
	keys, err := InitSwitchKeys(base)
	if err != nil || keys.GetKey("header_key") != "value" || keys.GetKey("missing") != "" {
		t.Fatalf("key file discovery: keys=%v err=%v", keys, err)
	}
	other := t.TempDir()
	got, err := ReadSettings(other)
	if err != nil || got != settingsInstance {
		t.Fatal("settings singleton was not retained")
	}
}
