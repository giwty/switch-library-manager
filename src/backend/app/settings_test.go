package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"go.uber.org/zap"
)

func TestSettingsRestartPolicyCoversEverySetting(t *testing.T) {
	var candidateSettings settings.AppSettings
	type impact struct {
		reason string
		scan   bool
	}
	expected := map[string]impact{}
	for _, path := range []string{
		"SchemaVersion", "GUI.Enabled", "GUI.PageSize", "GUI.HideMissingGames", "GUI.HideDemoGames",
		"GUI.HideVersionString", "GUI.RememberWindowState", "GUI.Theme", "Scan.RescanOnStartup",
		"MissingContent.CheckForUpdates", "MissingContent.CheckForDLC", "MissingContent.IgnoreDLCUpdates",
		"MissingContent.IgnoreDLCTitleIDs", "MissingContent.IgnoreUpdateIDs",
		"Organization.CreateFolderPerGame", "Organization.MoveScanFilesToLibrary", "Organization.DlcFolder",
		"Organization.UpdatesFolder", "Organization.RenameFiles", "Organization.DeleteEmptyFolders",
		"Organization.DeleteOldUpdateFiles", "Organization.FolderNameTemplate", "Organization.AllowUnicodeFileNames",
		"Organization.FileNameTemplate", "Organization.IncludeBaseType", "Organization.ProcessWhenMissingBaseGame",
	} {
		expected[path] = impact{}
	}
	for path, reason := range map[string]string{
		"Paths.LibraryFolder": "Library folder", "Paths.ScanFolders": "Scan folders", "Paths.ProdKeys": "Keys path",
		"Scan.Recursive": "Scan rules", "Scan.IgnoreFileTypes": "Scan rules", "Scan.IgnoreUnsupportedFileTypes": "Scan rules",
	} {
		expected[path] = impact{reason, true}
	}
	expected["DataSources.TitlesURL"] = impact{reason: "Data sources"}
	expected["DataSources.VersionsURL"] = impact{reason: "Data sources"}
	expected["Logging.Debug"] = impact{reason: "Debug logging"}
	// Walk the actual schema so a newly added setting needs an explicit policy
	// decision in this test, rather than silently missing the regression matrix.
	var walk func(reflect.Value, string)
	walk = func(value reflect.Value, prefix string) {
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			path := prefix + value.Type().Field(i).Name
			if field.Kind() == reflect.Struct {
				walk(field, path+".")
				continue
			}
			want, ok := expected[path]
			if !ok {
				t.Errorf("setting %s needs an explicit restart policy test", path)
				continue
			}
			t.Run(path, func(t *testing.T) {
				original := reflect.Zero(field.Type())
				switch field.Kind() {
				case reflect.Bool:
					field.SetBool(true)
				case reflect.Int:
					field.SetInt(1)
				case reflect.String:
					field.SetString("changed")
				case reflect.Slice:
					field.Set(reflect.ValueOf([]string{"changed"}))
				default:
					t.Fatalf("unhandled setting type %s", field.Type())
				}
				reasons, scan := settingsRestartReasons(settings.AppSettings{}, candidateSettings)
				field.Set(original)
				wantReasons := []string{}
				if want.reason != "" {
					wantReasons = append(wantReasons, want.reason)
				}
				if !reflect.DeepEqual(reasons, wantReasons) || scan != want.scan {
					t.Fatalf("impact = %v, %v; want %v, %v", reasons, scan, wantReasons, want.scan)
				}
			})
		}
	}
	walk(reflect.ValueOf(&candidateSettings).Elem(), "")
	current := normalizeSettings(settings.AppSettings{})
	if reasons, scan := settingsRestartReasons(current, settings.AppSettings{}); len(reasons) != 0 || scan {
		t.Fatalf("nil/empty lists triggered restart: %v, %v", reasons, scan)
	}
	mixed := current
	mixed.GUI.Theme = settings.ThemeDark
	mixed.Paths.LibraryFolder = "new"
	mixed.Logging.Debug = true
	if reasons, scan := settingsRestartReasons(current, mixed); !reflect.DeepEqual(reasons, []string{"Library folder", "Debug logging"}) || !scan {
		t.Fatalf("mixed impact = %v, %v", reasons, scan)
	}
}

func TestSettingsSaveIntegration(t *testing.T) {
	scenario := os.Getenv("SLM_SETTINGS_SCENARIO")
	if scenario == "" {
		for _, name := range []string{"live", "unchanged", "restart", "invalid", "snapshot failure", "save failure"} {
			t.Run(name, func(t *testing.T) { runSettingsProcess(t, name, "") })
		}
		return
	}
	base := os.Getenv("SLM_SETTINGS_BASE")
	if base == "" {
		base = t.TempDir()
	}
	if scenario == "reopen" {
		manager, err := db.NewLocalSwitchDBManager(base)
		if err != nil {
			t.Fatal(err)
		}
		a := &App{baseFolder: base, localDbManager: manager, state: State{switchDB: &db.SwitchTitlesDB{}}, sugarLogger: zap.NewNop().Sugar()}
		library, err := a.UpdateLocalLibrary(false)
		manager.Close()
		if err != nil || library.NumFiles != 1 || len(library.LibraryData) != 1 || !strings.HasPrefix(library.LibraryData[0].Name, "New") {
			t.Fatalf("new-process startup reused old snapshot: %#v, %v", library, err)
		}
		persistent, err := db.NewPersistentDB(base)
		if err != nil {
			t.Fatal(err)
		}
		defer persistent.Close()
		var sentinel map[string]string
		if err := persistent.GetEntry(db.DB_TABLE_FILE_SCAN_METADATA, "sentinel", &sentinel); err != nil || sentinel["preserved"] != "yes" {
			t.Fatalf("deep metadata lost: %v, %v", sentinel, err)
		}
		return
	}
	current, err := settings.ReadSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	initial := normalizeSettings(*current)
	initial.Paths.LibraryFolder = filepath.Join(base, "old")
	newFolder := filepath.Join(base, "new")
	for folder, name := range map[string]string{initial.Paths.LibraryFolder: "Old", newFolder: "New"} {
		if err := os.Mkdir(folder, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, name+" ["+detailsBaseID+"][v0].nsp"), []byte("synthetic"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := settings.SaveSettingsWithError(&initial, base); err != nil {
		t.Fatal(err)
	}
	persistent, err := db.NewPersistentDB(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistent.AddEntry(db.DB_TABLE_FILE_SCAN_METADATA, "sentinel", map[string]string{"preserved": "yes"}); err != nil {
		t.Fatal(err)
	}
	persistent.Close()
	manager, err := db.NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	a := &App{baseFolder: base, localDbManager: manager, sugarLogger: zap.NewNop().Sugar(), state: State{
		switchDB: &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{detailsPrefix: {Attributes: db.TitleAttributes{Id: detailsBaseID, Name: "日本語"}}}},
	}}
	if _, err := a.UpdateLocalLibrary(false); err != nil {
		t.Fatal(err)
	}
	a.state.organizationIssues = []Pair{{Key: "synthetic", Value: "organization diagnostic"}}
	local, remote := a.state.localDB, a.state.switchDB
	before, err := os.ReadFile(filepath.Join(base, settings.SETTINGS_FILENAME))
	if err != nil {
		t.Fatal(err)
	}
	candidate := normalizeSettings(initial)
	switch scenario {
	case "live":
		candidate.Organization.RenameFiles = true
		candidate.Organization.AllowUnicodeFileNames = true
		candidate.GUI.PageSize = 0
		candidate.GUI.Theme = settings.ThemeDark
		candidate.GUI.RememberWindowState = true
		candidate.GUI.Enabled = false
		candidate.Scan.RescanOnStartup = true
		candidate.MissingContent.IgnoreDLCUpdates = true
		candidate.MissingContent.IgnoreUpdateIDs = []string{detailsBaseID}
	case "unchanged":
	case "restart", "save failure":
		candidate.Paths.LibraryFolder = newFolder
		candidate.Organization.AllowUnicodeFileNames = true
		if scenario == "save failure" {
			a.baseFolder = filepath.Join(base, "nonexistent")
		}
	case "invalid":
		candidate.Paths.LibraryFolder = newFolder
		candidate.SchemaVersion = 999
	case "snapshot failure":
		candidate.Paths.LibraryFolder = newFolder
		manager.Close()
	default:
		t.Fatalf("unknown scenario %q", scenario)
	}
	result, saveErr := a.SaveSettings(candidate)
	if a.state.localDB != local || a.state.switchDB != remote {
		t.Fatal("saving rebuilt the loaded databases")
	}
	if scenario == "invalid" || scenario == "snapshot failure" || scenario == "save failure" {
		if saveErr == nil {
			t.Fatal("expected save failure")
		}
		loaded, err := settings.ReadSettings(base)
		if err != nil || !reflect.DeepEqual(*loaded, initial) || a.rememberWindowState {
			t.Fatalf("failure changed runtime settings: %#v, %v", loaded, err)
		}
		after, err := os.ReadFile(filepath.Join(base, settings.SETTINGS_FILENAME))
		if err != nil || string(before) != string(after) {
			t.Fatalf("failure changed persisted settings: %v", err)
		}
		if scenario != "snapshot failure" {
			// Invalid candidates must retain the snapshot; a write failure after
			// invalidation leaves a harmless cold cache using the old settings.
			if err := os.WriteFile(filepath.Join(initial.Paths.LibraryFolder, "Extra [0100000000020000][v0].nsp"), []byte("synthetic"), 0644); err != nil {
				t.Fatal(err)
			}
			a.baseFolder = base
			library, err := a.UpdateLocalLibrary(false)
			wantFiles := 1
			if scenario == "save failure" {
				wantFiles = 2
			}
			if err != nil || library.NumFiles != wantFiles {
				t.Fatalf("snapshot after %s: %#v, %v", scenario, library, err)
			}
		}
		return
	}
	if saveErr != nil {
		t.Fatal(saveErr)
	}
	if result.RestartRequired != (scenario == "restart") {
		t.Fatalf("restart = %v for %s", result.RestartRequired, scenario)
	}
	if scenario == "restart" {
		if result.Library != nil || !reflect.DeepEqual(result.RestartReasons, []string{"Library folder"}) {
			t.Fatalf("mixed restart result = %#v", result)
		}
		manager.Close()
		runSettingsProcess(t, "reopen", base)
	} else if scenario == "live" {
		if result.Library == nil || result.Library.LibraryData[0].Name != "日本語" || len(result.Library.Issues) != 1 || result.Settings.GUI.PageSize != 100 || !a.rememberWindowState {
			t.Fatalf("live result = %#v, library = %#v", result, result.Library)
		}
		preview, err := a.GetOrganizationPreview()
		if err != nil || len(preview.Entries) != 3 || !strings.Contains(preview.Entries[0].Path, "日本語") {
			t.Fatalf("preview did not use saved rules: %#v, %v", preview, err)
		}
		details, err := a.GetGameDetails(detailsBaseID)
		if err != nil || details.Name != "日本語" {
			t.Fatalf("details did not use saved naming: %#v, %v", details, err)
		}
		result.Settings.MissingContent.IgnoreUpdateIDs[0] = "bad"
		loaded, err := settings.ReadSettings(base)
		if err != nil || loaded.MissingContent.IgnoreUpdateIDs[0] != detailsBaseID {
			t.Fatalf("returned DTO aliases active settings: %#v, %v", loaded, err)
		}
	} else if result.Library != nil || len(result.RestartReasons) != 0 {
		t.Fatalf("unchanged save did extra work: %#v", result)
	}
}

func runSettingsProcess(t *testing.T, scenario, base string) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestSettingsSaveIntegration$")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "SLM_SETTINGS_SCENARIO=") && !strings.HasPrefix(entry, "SLM_SETTINGS_BASE=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "SLM_SETTINGS_SCENARIO="+scenario, "SLM_SETTINGS_BASE="+base)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("isolated %s: %v\n%s", scenario, err, output)
	}
}
