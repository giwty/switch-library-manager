package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

func TestNormalizeSettingsInitializesFrontendLists(t *testing.T) {
	value := normalizeSettings(settings.AppSettings{})

	if value.Paths.ScanFolders == nil || value.MissingContent.IgnoreDLCTitleIDs == nil || value.MissingContent.IgnoreUpdateIDs == nil || value.Scan.IgnoreFileTypes == nil {
		t.Fatal("expected settings lists to be initialized")
	}
}

func TestWindowStateLoadingIsOptIn(t *testing.T) {
	base := t.TempDir()
	want := &settings.WindowState{Width: 1360, Height: 840, X: 40, Y: -20, ScreenWidth: 1920, ScreenHeight: 1080}
	if err := settings.SaveWindowState(want, base); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(base, settings.WINDOW_STATE_FILENAME)
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	if got := loadWindowState(base, false, zap.NewNop().Sugar()); got != nil {
		t.Fatalf("disabled load returned %#v; want nil", got)
	}
	disabledWidth, disabledHeight := windowSizeForStart(false, want)
	if disabledWidth != defaultWindowWidth || disabledHeight != defaultWindowHeight {
		t.Fatalf("disabled startup size = %dx%d; want %dx%d", disabledWidth, disabledHeight, defaultWindowWidth, defaultWindowHeight)
	}
	after, err := os.ReadFile(filename)
	if err != nil || string(after) != string(before) {
		t.Fatalf("disabled load changed saved state: err=%v", err)
	}

	got := loadWindowState(base, true, zap.NewNop().Sugar())
	if got == nil || *got != *want {
		t.Fatalf("enabled load = %#v; want %#v", got, want)
	}
	enabledWidth, enabledHeight := windowSizeForStart(true, got)
	if enabledWidth != want.Width || enabledHeight != want.Height {
		t.Fatalf("enabled startup size = %dx%d; want %dx%d", enabledWidth, enabledHeight, want.Width, want.Height)
	}
	defaultWidth, defaultHeight := windowSizeForStart(true, nil)
	if defaultWidth != defaultWindowWidth || defaultHeight != defaultWindowHeight {
		t.Fatalf("missing-state startup size = %dx%d; want %dx%d", defaultWidth, defaultHeight, defaultWindowWidth, defaultWindowHeight)
	}
}

func TestWindowStateLoadFallsBackForMalformedState(t *testing.T) {
	base := t.TempDir()
	filename := filepath.Join(base, settings.WINDOW_STATE_FILENAME)
	malformed := []byte(`{"width":`)
	if err := os.WriteFile(filename, malformed, 0644); err != nil {
		t.Fatal(err)
	}

	if got := loadWindowState(base, true, zap.NewNop().Sugar()); got != nil {
		t.Fatalf("malformed state load returned %#v; want nil", got)
	}
	width, height := windowSizeForStart(true, nil)
	if width != defaultWindowWidth || height != defaultWindowHeight {
		t.Fatalf("malformed state startup size = %dx%d; want %dx%d", width, height, defaultWindowWidth, defaultWindowHeight)
	}
	after, err := os.ReadFile(filename)
	if err != nil || string(after) != string(malformed) {
		t.Fatalf("reading malformed state changed its file: err=%v", err)
	}
}

func TestResolveWindowStartState(t *testing.T) {
	current := wailsruntime.Screen{
		IsCurrent: true,
		IsPrimary: true,
	}
	current.Size.Width = 1366
	current.Size.Height = 768
	other := wailsruntime.Screen{}
	other.Size.Width = 1920
	other.Size.Height = 1080

	for _, test := range []struct {
		name      string
		state     *settings.WindowState
		screens   []wailsruntime.Screen
		screenErr error
		want      windowStartState
	}{
		{
			name: "matching display restores saved bounds",
			state: &settings.WindowState{
				Width: 1440, Height: 900, X: 120, Y: 45, ScreenWidth: 1920, ScreenHeight: 1080,
			},
			screens: []wailsruntime.Screen{current, other},
			want: windowStartState{
				Width: 1440, Height: 900, X: 120, Y: 45, RestorePosition: true,
			},
		},
		{
			name: "missing display recenters saved size when it fits",
			state: &settings.WindowState{
				Width: 1200, Height: 600, X: 1600, Y: 100, ScreenWidth: 1920, ScreenHeight: 1080,
			},
			screens: []wailsruntime.Screen{current},
			want:    windowStartState{Width: 1200, Height: 600, Center: true},
		},
		{
			name: "resolution change clamps and recenters",
			state: &settings.WindowState{
				Width: 1600, Height: 900, X: 120, Y: 45, ScreenWidth: 1920, ScreenHeight: 1080,
			},
			screens: []wailsruntime.Screen{current},
			want:    windowStartState{Width: 1366, Height: 768, Center: true},
		},
		{
			name: "window too large for matching display clamps and recenters",
			state: &settings.WindowState{
				Width: 1500, Height: 800, X: 120, Y: 45, ScreenWidth: 1366, ScreenHeight: 768,
			},
			screens: []wailsruntime.Screen{current, other},
			want:    windowStartState{Width: 1366, Height: 768, Center: true},
		},
		{
			name: "legacy state recenters saved size",
			state: &settings.WindowState{
				Width: 1200, Height: 600, X: -800, Y: 50,
			},
			screens: []wailsruntime.Screen{current},
			want:    windowStartState{Width: 1200, Height: 600, Center: true},
		},
		{
			name: "screen error uses defaults",
			state: &settings.WindowState{
				Width: 1440, Height: 900, X: 120, Y: 45, ScreenWidth: 1920, ScreenHeight: 1080,
			},
			screens:   []wailsruntime.Screen{current, other},
			screenErr: errors.New("screen query failed"),
			want:      windowStartState{Width: defaultWindowWidth, Height: defaultWindowHeight, Center: true},
		},
		{
			name: "no usable screen uses defaults",
			state: &settings.WindowState{
				Width: 1440, Height: 900, X: 120, Y: 45, ScreenWidth: 1920, ScreenHeight: 1080,
			},
			screens: []wailsruntime.Screen{{IsCurrent: true}},
			want:    windowStartState{Width: defaultWindowWidth, Height: defaultWindowHeight, Center: true},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := resolveWindowStartState(test.state, test.screens, test.screenErr)
			if got != test.want {
				t.Fatalf("resolveWindowStartState() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestWindowStateSaveIsOptInAndNormalOnly(t *testing.T) {
	base := t.TempDir()
	original := &settings.WindowState{Width: 1200, Height: 600, X: 10, Y: 20, ScreenWidth: 1920, ScreenHeight: 1080}
	if err := settings.SaveWindowState(original, base); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(base, settings.WINDOW_STATE_FILENAME)
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	changed := &settings.WindowState{Width: 1500, Height: 950, X: -800, Y: 60, ScreenWidth: 2560, ScreenHeight: 1440}

	for _, test := range []struct {
		name    string
		enabled bool
		normal  bool
	}{
		{name: "disabled", enabled: false, normal: true},
		{name: "not normal", enabled: true, normal: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := saveWindowStateIfNeeded(base, test.enabled, test.normal, changed); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(filename)
			if err != nil || string(after) != string(before) {
				t.Fatalf("skipped save changed state: err=%v", err)
			}
		})
	}
	if err := saveWindowStateIfNeeded(base, true, true, changed); err != nil {
		t.Fatal(err)
	}
	got, err := settings.ReadWindowState(base)
	if err != nil || got == nil || *got != *changed {
		t.Fatalf("enabled normal save = %#v, %v; want %#v", got, err, changed)
	}
}

func TestSaveSettingsUpdatesRememberWindowStateFlag(t *testing.T) {
	base := t.TempDir()
	manager, err := db.NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	application := &App{baseFolder: base, localDbManager: manager}
	if _, err := application.SaveSettings(settings.AppSettings{
		GUI: settings.GUISettings{RememberWindowState: true},
	}); err != nil {
		t.Fatal(err)
	}
	if !application.rememberWindowState {
		t.Fatal("remember-window-state flag was not enabled after settings save")
	}
	if _, err := application.SaveSettings(settings.AppSettings{}); err != nil {
		t.Fatal(err)
	}
	if application.rememberWindowState {
		t.Fatal("remember-window-state flag was not disabled after settings save")
	}
}

func TestRestartFailureCanBeRetriedAndRepeatedRequestsAreRejected(t *testing.T) {
	launches := 0
	quits := 0
	application := &App{}
	application.ctx = context.Background()
	application.restart = func() error {
		launches++
		if launches == 1 {
			return errors.New("start failed")
		}
		return nil
	}
	application.quit = func(context.Context) {
		if !applicationMutexAvailable(application) {
			t.Error("application state mutex is held while quitting")
		}
		quits++
	}

	if err := application.Restart(); err == nil || !strings.Contains(err.Error(), "start failed") {
		t.Fatalf("first Restart() error = %v, want launch failure", err)
	}
	if err := application.Restart(); err != nil {
		t.Fatalf("retry Restart() error = %v", err)
	}
	if err := application.Restart(); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("repeated Restart() error = %v, want already-in-progress error", err)
	}
	if launches != 2 || quits != 1 {
		t.Fatalf("restart launches = %d and quits = %d; want 2 and 1", launches, quits)
	}
}

func applicationMutexAvailable(application *App) bool {
	if !application.state.mu.TryLock() {
		return false
	}
	application.state.mu.Unlock()
	return true
}

func TestRestartRequiresConfiguredLauncherAndStartedApplication(t *testing.T) {
	if err := (&App{}).Restart(); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("Restart() without launcher error = %v, want unavailable", err)
	}
	if err := (&App{restart: func() error { return nil }}).Restart(); err == nil || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("Restart() before startup error = %v, want not-ready", err)
	}
}

func TestSettingsMigrationMessageIncludesBackupAndDefaultInstructions(t *testing.T) {
	message := settingsMigrationMessage(&settings.MigrationInfo{BackupPath: "C:\\app\\settings.old.json"})
	if !strings.Contains(message, "C:\\app\\settings.old.json") || !strings.Contains(message, "current default values") || !strings.Contains(message, "docs/settings.md") {
		t.Fatalf("migration message = %q", message)
	}
	if strings.Contains(message, "v1") || strings.Contains(message, "v2") {
		t.Fatalf("migration message contains schema-specific wording: %q", message)
	}
}

func TestFrontendModelsPreserveJSONContract(t *testing.T) {
	data, err := json.Marshal(LocalLibraryData{
		LibraryData: []LibraryTemplateData{{TitleId: "0100000000001000"}},
		Issues:      []Pair{{Key: "file.nsp", Value: "issue"}},
		NumFiles:    1,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `{"library_data":[{"id":0,"name":"","version":"","dlc":"","titleId":"0100000000001000","path":"","icon":"","update":0,"region":"","type":""}],"issues":[{"key":"file.nsp","value":"issue"}],"num_files":1}`
	if string(data) != want {
		t.Fatalf("JSON = %s, want %s", data, want)
	}
}

func TestBuildSwitchDBClosesDownloadedFiles(t *testing.T) {
	for _, test := range []struct {
		name         string
		failVersions bool
		wantBuildErr bool
	}{
		{name: "success"},
		{name: "versions download failure", failVersions: true, wantBuildErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			baseFolder := t.TempDir()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/versions" && test.failVersions {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/titles":
					_, _ = fmt.Fprint(w, `{"0100000000010000":{"id":"0100000000010000","name":"Game"}}`)
				case "/versions":
					_, _ = fmt.Fprint(w, `{"0100000000010000":{"1":"2024-01-01"}}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			if err := settings.SaveSettingsWithError(&settings.AppSettings{
				SchemaVersion: settings.SETTINGS_SCHEMA_VERSION,
				DataSources: settings.DataSourceSettings{
					TitlesURL:   server.URL + "/titles",
					VersionsURL: server.URL + "/versions",
				},
			}, baseFolder); err != nil {
				t.Fatal(err)
			}

			application := &App{
				baseFolder:  baseFolder,
				sugarLogger: zap.NewNop().Sugar(),
			}
			_, err := application.buildSwitchDB()
			if (err != nil) != test.wantBuildErr {
				t.Fatalf("buildSwitchDB() error = %v, want error: %v", err, test.wantBuildErr)
			}

			filesToCheck := []string{settings.TITLE_JSON_FILENAME}
			if !test.failVersions {
				filesToCheck = append(filesToCheck, settings.VERSIONS_JSON_FILENAME)
			} else if _, statErr := os.Stat(filepath.Join(baseFolder, settings.VERSIONS_JSON_FILENAME)); !os.IsNotExist(statErr) {
				t.Fatalf("versions file should not be created after failed download, stat error = %v", statErr)
			}
			for _, name := range filesToCheck {
				path := filepath.Join(baseFolder, name)
				renamed := path + ".renamed"
				if err := os.Rename(path, renamed); err != nil {
					t.Fatalf("rename %s after buildSwitchDB: %v", name, err)
				}
				if err := os.Remove(renamed); err != nil {
					t.Fatalf("remove renamed %s: %v", name, err)
				}
			}
		})
	}
}

func TestRescanLibraryRefreshesSnapshotAndPreservesMetadataCache(t *testing.T) {
	baseFolder := t.TempDir()
	libraryFolder := filepath.Join(baseFolder, "library")
	additionalFolder := filepath.Join(baseFolder, "additional")
	nestedFolder := filepath.Join(additionalFolder, "nested")
	if err := os.MkdirAll(libraryFolder, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nestedFolder, 0755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(libraryFolder, "First Game [0100000000001000][v0].nsp")
	additional := filepath.Join(nestedFolder, "Additional Game [0100000000003000][v0].nsp")
	if err := os.WriteFile(first, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(additional, []byte("additional"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveSettingsWithError(&settings.AppSettings{
		Paths: settings.PathSettings{LibraryFolder: libraryFolder, ScanFolders: []string{additionalFolder}},
		Scan:  settings.ScanSettings{Recursive: true, IgnoreFileTypes: []string{}},
	}, baseFolder); err != nil {
		t.Fatal(err)
	}

	localManager, err := db.NewLocalSwitchDBManager(baseFolder)
	if err != nil {
		t.Fatal(err)
	}
	defer localManager.Close()
	application := &App{
		baseFolder:     baseFolder,
		localDbManager: localManager,
		sugarLogger:    zap.NewNop().Sugar(),
		state:          State{switchDB: &db.SwitchTitlesDB{}},
	}

	firstScan, err := application.RescanLibrary(false)
	if err != nil {
		t.Fatal(err)
	}
	if firstScan.NumFiles != 2 {
		t.Fatalf("first scan files = %d, want 2 across recursive scan folders", firstScan.NumFiles)
	}

	// A disabled startup rescan continues to use the saved library snapshot.
	second := filepath.Join(libraryFolder, "Second Game [0100000000002000][v0].nsp")
	if err := os.WriteFile(second, []byte("second"), 0644); err != nil {
		t.Fatal(err)
	}
	unsupported := filepath.Join(libraryFolder, "notes.txt")
	if err := os.WriteFile(unsupported, []byte("notes"), 0644); err != nil {
		t.Fatal(err)
	}
	cachedStartup, err := application.UpdateLocalLibrary(false)
	if err != nil || cachedStartup.NumFiles != 2 {
		t.Fatalf("cached startup: files=%d err=%v, want 2 files", cachedStartup.NumFiles, err)
	}

	// An enabled startup rescan sees new files while leaving deep metadata intact.
	refreshedStartup, err := application.UpdateLocalLibrary(true)
	if err != nil || refreshedStartup.NumFiles != 4 || len(refreshedStartup.Issues) != 1 {
		t.Fatalf("refreshed startup: files=%d issues=%d err=%v, want 4 files and 1 issue", refreshedStartup.NumFiles, len(refreshedStartup.Issues), err)
	}

	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(unsupported); err != nil {
		t.Fatal(err)
	}
	normalScan, err := application.RescanLibrary(false)
	if err != nil {
		t.Fatal(err)
	}
	if normalScan.NumFiles != 2 {
		t.Fatalf("normal rescan files = %d, want 2 after deletion", normalScan.NumFiles)
	}
	if len(normalScan.Issues) != 0 {
		t.Fatalf("normal rescan retained stale skipped-file diagnostics: %#v", normalScan.Issues)
	}

	lastGoodSnapshot := application.state.localDB
	if err := settings.SaveSettingsWithError(&settings.AppSettings{
		Paths: settings.PathSettings{LibraryFolder: libraryFolder, ScanFolders: []string{filepath.Join(baseFolder, "missing-folder")}},
		Scan:  settings.ScanSettings{Recursive: true, IgnoreFileTypes: []string{}},
	}, baseFolder); err != nil {
		t.Fatal(err)
	}
	if _, err := application.RescanLibrary(false); err == nil || !strings.Contains(err.Error(), "missing-folder") {
		t.Fatalf("missing scan folder error = %v, want folder context", err)
	}
	if application.state.localDB != lastGoodSnapshot {
		t.Fatal("failed scan replaced the last good in-memory snapshot")
	}
	cachedAfterFailure, err := application.UpdateLocalLibrary(false)
	if err != nil || cachedAfterFailure.NumFiles != 2 {
		t.Fatalf("cached snapshot after failed scan: files=%d err=%v, want 2 files", cachedAfterFailure.NumFiles, err)
	}

	if err := settings.SaveSettingsWithError(&settings.AppSettings{
		Paths: settings.PathSettings{LibraryFolder: libraryFolder, ScanFolders: []string{additionalFolder}},
		Scan:  settings.ScanSettings{Recursive: true, IgnoreFileTypes: []string{}},
	}, baseFolder); err != nil {
		t.Fatal(err)
	}

	hardScan, err := application.RescanLibrary(true)
	if err != nil {
		t.Fatal(err)
	}
	if hardScan.NumFiles != 2 {
		t.Fatalf("hard rescan files = %d, want 2", hardScan.NumFiles)
	}

	for _, path := range []string{second, additional} {
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove %s: %v", path, err)
		}
	}

	for _, scan := range []struct {
		name string
		run  func() (LocalLibraryData, error)
	}{
		{name: "startup scan", run: func() (LocalLibraryData, error) { return application.UpdateLocalLibrary(true) }},
		{name: "normal rescan", run: func() (LocalLibraryData, error) { return application.RescanLibrary(false) }},
		{name: "hard rescan", run: func() (LocalLibraryData, error) { return application.RescanLibrary(true) }},
	} {
		t.Run("empty library "+scan.name, func(t *testing.T) {
			got, err := scan.run()
			if err != nil {
				t.Fatal(err)
			}
			if got.NumFiles != 0 || len(got.LibraryData) != 0 || len(got.Issues) != 0 {
				t.Fatalf("empty scan = %#v, want no files, rows, or issues", got)
			}
			if got.LibraryData == nil || got.Issues == nil {
				t.Fatalf("empty scan lists must be non-nil: library=%#v issues=%#v", got.LibraryData, got.Issues)
			}
		})
	}
}
