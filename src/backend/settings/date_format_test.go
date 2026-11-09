package settings

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/dateformat"
)

// Each load/reload runs in a fresh process because settings are process-global.
func TestDateFormatSettings(t *testing.T) {
	scenario := os.Getenv("SLM_DATE_SETTINGS_SCENARIO")
	if scenario == "" {
		runDateSettingsProcess(t, "defaults", t.TempDir())
		for _, field := range []string{"", `,"date_format":""`, `,"date_format":"  "`, `,"date_format":"DD.MM.YYYY"`} {
			base := t.TempDir()
			body := `{"schema_version":2,"gui":{"page_size":37` + field + `},"paths":{"library_folder":"synthetic"}}`
			if err := os.WriteFile(filepath.Join(base, SETTINGS_FILENAME), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			load := "load ISO"
			if strings.Contains(field, "DD.MM.YYYY") {
				load = "load custom"
			}
			runDateSettingsProcess(t, load, base)
			runDateSettingsProcess(t, "reload", base)
		}
		runDateSettingsProcess(t, "invalid save", t.TempDir())
		base := t.TempDir()
		body := `{"schema_version":2,"gui":{"date_format":"invalid"}}`
		if err := os.WriteFile(filepath.Join(base, SETTINGS_FILENAME), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		runDateSettingsProcess(t, "invalid load", base)
		return
	}
	base := os.Getenv("SLM_DATE_SETTINGS_BASE")
	filename := filepath.Join(base, SETTINGS_FILENAME)
	if scenario == "defaults" {
		loaded, err := ReadSettings(base)
		if err != nil || loaded.GUI.DateFormat != dateformat.Default {
			t.Fatalf("defaults = %#v, %v", loaded, err)
		}
		return
	}
	if scenario == "invalid save" {
		current := defaultSettings()
		if err := SaveSettingsWithError(current, base); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		candidate := *current
		candidate.GUI.DateFormat = "YYYY-MM-DD HH:mm"
		if err := SaveSettingsWithError(&candidate, base); err == nil || !strings.Contains(err.Error(), "gui.date_format") {
			t.Fatalf("save error = %v", err)
		}
		after, err := os.ReadFile(filename)
		if err != nil || string(before) != string(after) || settingsInstance != current {
			t.Fatalf("failed save changed settings: %v", err)
		}
		return
	}
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	loaded, loadErr := ReadSettings(base)
	after, err := os.ReadFile(filename)
	if err != nil || string(before) != string(after) {
		t.Fatalf("load rewrote file: %v", err)
	}
	if scenario == "invalid load" {
		if loadErr == nil || !strings.Contains(loadErr.Error(), "gui.date_format") || settingsInstance != nil {
			t.Fatalf("invalid load = %#v, %v", loaded, loadErr)
		}
		return
	}
	want := dateformat.Default
	if scenario == "load custom" {
		want = "DD.MM.YYYY"
	}
	if scenario == "reload" {
		want = "MMMM D, YYYY"
	}
	if loadErr != nil || loaded.GUI.DateFormat != want || loaded.GUI.PageSize != 37 || loaded.Paths.LibraryFolder != "synthetic" || loaded.SchemaVersion != 2 {
		t.Fatalf("%s = %#v, %v", scenario, loaded, loadErr)
	}
	if scenario != "reload" {
		loaded.GUI.DateFormat = " \tMMMM D, YYYY\n"
		if err := SaveSettingsWithError(loaded, base); err != nil {
			t.Fatal(err)
		}
	}
}

func runDateSettingsProcess(t *testing.T, scenario, base string) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestDateFormatSettings$")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "SLM_DATE_SETTINGS_SCENARIO=") && !strings.HasPrefix(entry, "SLM_DATE_SETTINGS_BASE=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "SLM_DATE_SETTINGS_SCENARIO="+scenario, "SLM_DATE_SETTINGS_BASE="+base)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("isolated %s: %v\n%s", scenario, err, output)
	}
}
