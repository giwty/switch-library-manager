package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestMissingGettersRequireLoadedDatabases(t *testing.T) {
	for _, a := range []*App{{}, {state: State{switchDB: &db.SwitchTitlesDB{}}}, {state: State{localDB: &db.LocalSwitchFilesDB{}}}} {
		_, gamesErr := a.GetMissingGames()
		_, updatesErr := a.GetMissingUpdates()
		_, dlcErr := a.GetMissingDLC()
		for _, err := range []error{gamesErr, updatesErr, dlcErr} {
			if err == nil || !strings.Contains(err.Error(), "databases must be loaded") {
				t.Fatalf("unloaded getter error = %v", err)
			}
		}
	}
}

func TestMissingGettersSettingsIntegration(t *testing.T) {
	scenario := os.Getenv("SLM_MISSING_TEST_CASE")
	if scenario == "" {
		for _, name := range []string{"filter and persist", "invalid settings"} {
			t.Run(name, func(t *testing.T) { runMissingGetterProcess(t, name, "") })
		}
		return
	}
	base := os.Getenv("SLM_MISSING_TEST_FOLDER")
	if base == "" {
		base = t.TempDir()
	}
	const baseID = "0100abcd00000000"
	const dlcID = "0100abcd00001001"
	prefix := baseID[:13]
	a := &App{baseFolder: base, state: State{
		localDB: &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{prefix: {BaseExist: true, Updates: map[int]db.SwitchFileInfo{}, Dlc: map[string]db.SwitchFileInfo{}}}},
		switchDB: &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
			prefix:          {Attributes: db.TitleAttributes{Id: baseID, Name: "Owned"}, Updates: map[int]string{65536: "2026-01-01"}, Dlc: map[string]db.TitleAttributes{dlcID: {Id: dlcID, Name: "Expansion"}}},
			"0100abcd00002": {Attributes: db.TitleAttributes{Id: "0100abcd00002000", Name: "Missing"}},
			"0100abcd00003": {Attributes: db.TitleAttributes{Id: "0100abcd00003000", Name: "Demo", IsDemo: true}},
			"0100abcd00004": {Attributes: db.TitleAttributes{Id: "0100abcd00004000"}},
		}},
	}}
	if scenario == "invalid settings" {
		if err := os.WriteFile(filepath.Join(base, settings.SETTINGS_FILENAME), []byte("{"), 0600); err != nil {
			t.Fatal(err)
		}
		_, gamesErr := a.GetMissingGames()
		_, updatesErr := a.GetMissingUpdates()
		_, dlcErr := a.GetMissingDLC()
		for _, err := range []error{gamesErr, updatesErr, dlcErr} {
			if err == nil || !strings.Contains(err.Error(), "decode settings") {
				t.Fatalf("invalid settings error = %v", err)
			}
		}
		return
	}
	if scenario != "reload" {
		if err := settings.SaveSettingsWithError(&settings.AppSettings{SchemaVersion: settings.SETTINGS_SCHEMA_VERSION, GUI: settings.GUISettings{HideDemoGames: true}}, base); err != nil {
			t.Fatal(err)
		}
		games, err := a.GetMissingGames()
		if err != nil || len(games) != 1 || games[0].Name != "Missing" {
			t.Fatalf("filtered games = %#v, %v", games, err)
		}
		updates, err := a.GetMissingUpdates()
		if err != nil || len(updates) != 1 || updates[0].LatestUpdate != 65536 {
			t.Fatalf("updates = %#v, %v", updates, err)
		}
		dlc, err := a.GetMissingDLC()
		if err != nil || len(dlc) != 1 || len(dlc[0].MissingDLC) != 1 {
			t.Fatalf("DLC = %#v, %v", dlc, err)
		}
		if _, err := a.SetContentIgnored("updates", strings.ToUpper(baseID), true); err != nil {
			t.Fatal(err)
		}
		if _, err := a.SetContentIgnored("dlc", strings.ToUpper(dlcID), true); err != nil {
			t.Fatal(err)
		}
		runMissingGetterProcess(t, "reload", base)
	}
	updates, err := a.GetMissingUpdates()
	if err != nil || len(updates) != 0 {
		t.Fatalf("ignored updates = %#v, %v", updates, err)
	}
	dlc, err := a.GetMissingDLC()
	if err != nil || len(dlc) != 0 {
		t.Fatalf("ignored DLC = %#v, %v", dlc, err)
	}
	loaded, err := a.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.GUI.HideDemoGames {
		t.Fatal("saved demo filtering preference was not preserved")
	}
	if scenario == "reload" {
		return
	}
	changed := *loaded
	changed.GUI.HideDemoGames = false
	if _, err := a.SaveSettings(changed); err != nil {
		t.Fatal(err)
	}
	games, err := a.GetMissingGames()
	if err != nil || len(games) != 2 {
		t.Fatalf("games with demos = %#v, %v", games, err)
	}
}

func runMissingGetterProcess(t *testing.T, scenario, folder string) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestMissingGettersSettingsIntegration$")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "SLM_MISSING_TEST_CASE=") && !strings.HasPrefix(entry, "SLM_MISSING_TEST_FOLDER=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "SLM_MISSING_TEST_CASE="+scenario, "SLM_MISSING_TEST_FOLDER="+folder)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("isolated %s: %v\n%s", scenario, err, output)
	}
}
