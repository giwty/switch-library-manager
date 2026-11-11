package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeTitleNameOverrides(t *testing.T) {
	const id = "0100abcdef123000"
	for _, test := range []struct {
		name    string
		input   map[string]string
		invalid bool
	}{
		{"empty", nil, false},
		{"normalized", map[string]string{"0100ABCDEF123000": "  私の Game  名  "}, false},
		{"same aliases", map[string]string{"0100ABCDEF123000": "Game", id: "Game"}, false},
		{"conflicting aliases", map[string]string{"0100ABCDEF123000": "Other", id: "Game"}, true},
		{"short ID", map[string]string{"123000": "Game"}, true},
		{"non hex", map[string]string{"0100gbcdef123000": "Game"}, true},
		{"DLC", map[string]string{"0100abcdef124001": "Game"}, true},
		{"update", map[string]string{"0100abcdef123800": "Game"}, true},
		{"blank", map[string]string{id: "  "}, true},
		{"control", map[string]string{id: "Game\nName"}, true},
		{"invalid UTF8", map[string]string{id: string([]byte{0xff})}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeTitleNameOverrides(test.input)
			if (err != nil) != test.invalid {
				t.Fatalf("result = %v, %v", got, err)
			}
			if !test.invalid && got == nil {
				t.Fatal("empty names must serialize as an object")
			}
			if test.name == "normalized" && got[id] != "私の Game  名" {
				t.Fatalf("name = %q", got[id])
			}
		})
	}
}

func TestTitleNamesSettingsLoadSaveAndFailure(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	initial, err := ReadSettings(base)
	if err != nil || initial.TitleNameOverrides == nil {
		t.Fatalf("defaults = %v, %v", initial, err)
	}
	candidate := *initial
	candidate.TitleNameOverrides = map[string]string{"0100ABCDEF123000": "  Custom 日本語  "}
	if err := SaveSettingsWithError(&candidate, base); err != nil {
		t.Fatal(err)
	}
	settingsInstance = nil
	loaded, err := ReadSettings(base)
	if err != nil || !reflect.DeepEqual(loaded.TitleNameOverrides, map[string]string{"0100abcdef123000": "Custom 日本語"}) {
		t.Fatalf("reload = %v, %v", loaded, err)
	}
	for _, contents := range []string{
		`{"schema_version":2,"title_name_overrides":{"bad":"Name"}}`,
		`{"schema_version":2,"title_name_overrides":{"0100abcdef123000":" "}}`,
	} {
		settingsInstance = nil
		if err := os.WriteFile(filepath.Join(base, SETTINGS_FILENAME), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadSettings(base); err == nil {
			t.Fatal("invalid saved names accepted")
		}
	}
	settingsInstance = nil
	if err := os.WriteFile(filepath.Join(base, SETTINGS_FILENAME), []byte(`{"schema_version":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err = ReadSettings(base)
	if err != nil || loaded.TitleNameOverrides == nil || len(loaded.TitleNameOverrides) != 0 {
		t.Fatalf("old current settings = %v, %v", loaded, err)
	}
}
