package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/trembon/switch-library-manager/backend/settings"
)

// Separate processes isolate the settings singleton, including the reload test.
func TestSetContentIgnored(t *testing.T) {
	scenario := os.Getenv("SLM_IGNORE_TEST_CASE")
	if scenario == "" {
		for _, name := range []string{"mutation", "invalid", "save failure", "load failure", "concurrent"} {
			t.Run(name, func(t *testing.T) { runIgnoreTestProcess(t, name, "") })
		}
		return
	}
	if scenario == "reload" {
		loaded, err := settings.ReadSettings(os.Getenv("SLM_IGNORE_TEST_FOLDER"))
		if err != nil || !reflect.DeepEqual(loaded.MissingContent.IgnoreUpdateIDs, []string{"0100abcd00000000"}) {
			t.Fatalf("fresh settings reload = %#v, %v", loaded, err)
		}
		return
	}
	base := t.TempDir()
	a := &App{baseFolder: base}
	if scenario == "load failure" {
		if err := os.WriteFile(filepath.Join(base, settings.SETTINGS_FILENAME), []byte("{"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := a.SetContentIgnored("updates", "0100abcd00000000", true); err == nil || !strings.Contains(err.Error(), "load ignore settings") {
			t.Fatalf("load error = %v", err)
		}
		return
	}
	initial := &settings.AppSettings{
		Paths: settings.PathSettings{LibraryFolder: "synthetic-library", ScanFolders: []string{"synthetic-scan"}},
		GUI:   settings.GUISettings{Theme: settings.ThemeDark, PageSize: 23},
		MissingContent: settings.MissingContentSettings{
			CheckForUpdates: true, CheckForDLC: true,
			IgnoreUpdateIDs:   []string{"0100ABCD00000000", "0100abcd00000000"},
			IgnoreDLCTitleIDs: []string{"0100abcd00001002"},
		},
	}
	if err := settings.SaveSettingsWithError(initial, base); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(base, settings.SETTINGS_FILENAME))
	if err != nil {
		t.Fatal(err)
	}
	switch scenario {
	case "mutation":
		for i := 0; i < 2; i++ {
			saved, err := a.SetContentIgnored("updates", "0100ABCD00000000", true)
			if err != nil || !reflect.DeepEqual(saved.IgnoreUpdateIDs, []string{"0100abcd00000000"}) {
				t.Fatalf("idempotent add = %#v, %v", saved, err)
			}
		}
		runIgnoreTestProcess(t, "reload", base)
		for _, kind := range []string{"updates", "dlc"} {
			id := "0100ABCD00000000"
			if kind == "dlc" {
				id = "0100ABCD00001001"
			}
			if _, err := a.SetContentIgnored(kind, id, true); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if _, err := a.SetContentIgnored(kind, id, false); err != nil {
					t.Fatal(err)
				}
			}
		}
		// Removal must also clear duplicate entries already present in settings.
		if err := settings.SaveSettingsWithError(initial, base); err != nil {
			t.Fatal(err)
		}
		saved, err := a.SetContentIgnored("updates", "0100abcd00000000", false)
		if err != nil || len(saved.IgnoreUpdateIDs) != 0 {
			t.Fatalf("remove duplicates = %#v, %v", saved, err)
		}
		saved, err = a.SetContentIgnored("updates", "0100abcd00001001", true)
		if err != nil {
			t.Fatal(err)
		}
		// Returned lists must not let callers mutate the published settings.
		saved.IgnoreUpdateIDs[0] = "bad"
		saved.IgnoreDLCTitleIDs[0] = "bad"
		current, err := settings.ReadSettings(base)
		if err != nil {
			t.Fatal(err)
		}
		want := *initial
		want.MissingContent.IgnoreUpdateIDs = []string{"0100abcd00001001"}
		if !reflect.DeepEqual(*current, want) {
			t.Fatalf("unrelated settings changed: got %#v; want %#v", *current, want)
		}
		if !reflect.DeepEqual(initial.MissingContent.IgnoreUpdateIDs, []string{"0100ABCD00000000", "0100abcd00000000"}) {
			t.Fatal("the previous published settings were mutated")
		}
		data, err := os.ReadFile(filepath.Join(base, settings.SETTINGS_FILENAME))
		if err != nil {
			t.Fatal(err)
		}
		var disk settings.AppSettings
		if err := json.Unmarshal(data, &disk); err != nil || !reflect.DeepEqual(disk, *current) {
			t.Fatalf("persisted settings differ: %#v, %v", disk, err)
		}
	case "invalid":
		for _, input := range []struct{ kind, id string }{
			{"other", "0100abcd00000000"}, {"updates", ""}, {"updates", "0100abcd0000000"},
			{"updates", "0100abcd0000000z"}, {"updates", "0100abcd00000800"},
			{"dlc", "0100abcd00000000"}, {"dlc", "0100abcd00000800"},
			{"dlc", "0100abcd00000001"}, {"updates", " 0100abcd00000000"},
		} {
			if _, err := a.SetContentIgnored(input.kind, input.id, true); err == nil {
				t.Errorf("accepted invalid input %#v", input)
			}
		}
		after, err := os.ReadFile(filepath.Join(base, settings.SETTINGS_FILENAME))
		if err != nil || string(after) != string(before) {
			t.Fatalf("invalid input changed settings: %v", err)
		}
	case "save failure":
		// A nonexistent destination fails before touching the original file.
		a.baseFolder = filepath.Join(base, "nonexistent", "folder")
		if _, err := a.SetContentIgnored("updates", "0100abcd00000000", false); err == nil || !strings.Contains(err.Error(), "save ignore settings") {
			t.Fatalf("save error = %v", err)
		}
		current, err := settings.ReadSettings(base)
		if err != nil || current != initial || len(current.MissingContent.IgnoreUpdateIDs) != 2 {
			t.Fatalf("failed save changed memory: %#v, %v", current, err)
		}
		after, err := os.ReadFile(filepath.Join(base, settings.SETTINGS_FILENAME))
		if err != nil || string(after) != string(before) {
			t.Fatalf("failed save changed original file: %v", err)
		}
	case "concurrent":
		var workers sync.WaitGroup
		for _, id := range []string{"0100abcd00001001", "0100abcd00001002", "0100abcd00001003"} {
			workers.Add(1)
			go func() {
				defer workers.Done()
				if _, err := a.SetContentIgnored("updates", id, true); err != nil {
					t.Error(err)
				}
			}()
		}
		workers.Wait()
		current, err := settings.ReadSettings(base)
		if err != nil || len(current.MissingContent.IgnoreUpdateIDs) != 5 {
			t.Fatalf("concurrent edits lost settings: %#v, %v", current, err)
		}
	default:
		t.Fatalf("unknown test scenario %q", scenario)
	}
}

func runIgnoreTestProcess(t *testing.T, scenario, folder string) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestSetContentIgnored$")
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "SLM_IGNORE_TEST_CASE=") && !strings.HasPrefix(entry, "SLM_IGNORE_TEST_FOLDER=") {
			env = append(env, entry)
		}
	}
	command.Env = append(env, "SLM_IGNORE_TEST_CASE="+scenario, "SLM_IGNORE_TEST_FOLDER="+folder)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("isolated %s test: %v\n%s", scenario, err, output)
	}
}
