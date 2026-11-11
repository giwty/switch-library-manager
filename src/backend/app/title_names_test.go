package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func TestGameNameOverrides(t *testing.T) {
	scenario := os.Getenv("SLM_NAME_CASE")
	if scenario == "" {
		for _, name := range []string{"save", "failure", "invalid", "concurrent", "reload", "missing database", "load failure"} {
			t.Run(name, func(t *testing.T) {
				cmd := exec.Command(os.Args[0], "-test.run=^TestGameNameOverrides$")
				cmd.Env = append(os.Environ(), "SLM_NAME_CASE="+name)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%v\n%s", err, out)
				}
			})
		}
		return
	}
	base := t.TempDir()
	a := &App{baseFolder: base, state: State{
		localDB: &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{detailsPrefix: {
			BaseExist: true, File: detailsFile("Fallback ["+detailsBaseID+"].nsp", &switchfs.ContentMetaAttributes{TitleId: detailsBaseID}),
		}}},
		switchDB: &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{detailsPrefix: {
			Attributes: db.TitleAttributes{Id: detailsBaseID, Name: "Catalog"}, Updates: map[int]string{2: "2026-01-01"},
			Dlc: map[string]db.TitleAttributes{detailsDLCID: {Id: detailsDLCID, Name: "Expansion"}},
		}}},
	}}
	if scenario == "load failure" {
		if err := os.WriteFile(filepath.Join(base, "settings.json"), []byte("{"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := a.SetGameNameOverride(detailsBaseID, "Name"); err == nil {
			t.Fatal("load error lost")
		}
		return
	}
	initial, err := settings.ReadSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	initial.GUI.PageSize = 37
	if scenario == "failure" {
		if err := os.Remove(filepath.Join(base, "settings.json")); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(base, "settings.json"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "settings.json", "keep"), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := a.SetGameNameOverride(detailsBaseID, "Name"); err == nil {
			t.Fatal("save error lost")
		}
		current, _ := settings.ReadSettings(base)
		if len(current.TitleNameOverrides) != 0 {
			t.Fatal("failed save published names")
		}
		return
	}
	if scenario == "missing database" {
		a.state.localDB = nil
		if _, err := a.SetGameNameOverride(detailsBaseID, "Name"); err == nil {
			t.Fatal("missing database accepted")
		}
		return
	}
	if scenario == "invalid" {
		for _, test := range [][2]string{{detailsUpdateID, "Name"}, {detailsDLCID, "Name"}, {"bad", ""}, {detailsBaseID, " "}, {detailsBaseID, "Name\n"}, {"0100000000990000", "Name"}} {
			if _, err := a.SetGameNameOverride(test[0], test[1]); err == nil {
				t.Fatalf("accepted %q", test)
			}
		}
		return
	}
	if scenario == "concurrent" {
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Go(func() {
				if _, err := a.SetGameNameOverride(detailsBaseID, "Name"); err != nil {
					t.Error(err)
				}
				if _, err := a.SetContentIgnored("updates", detailsBaseID, true); err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
		current, _ := settings.ReadSettings(base)
		if current.TitleNameOverrides[detailsBaseID] != "Name" || len(current.MissingContent.IgnoreUpdateIDs) != 1 {
			t.Fatal("concurrent settings lost")
		}
		return
	}
	result, err := a.SetGameNameOverride(strings.ToUpper(detailsBaseID), "  私の <Game>  ")
	if err != nil {
		t.Fatal(err)
	}
	if result.Details.Name != "私の <Game>" || result.Details.AutomaticName != "Catalog" || result.Details.CustomName != result.Details.Name || result.Library.LibraryData[0].Name != result.Details.Name {
		t.Fatalf("result = %#v", result)
	}
	current, _ := settings.ReadSettings(base)
	if current.GUI.PageSize != 37 || initial.TitleNameOverrides[detailsBaseID] != "" || a.state.switchDB.TitlesMap[detailsPrefix].Attributes.Name != "Catalog" {
		t.Fatal("unrelated state mutated")
	}
	result.TitleNameOverrides[detailsBaseID] = "mutated DTO"
	if current.TitleNameOverrides[detailsBaseID] != "私の <Game>" {
		t.Fatal("response aliases settings")
	}
	missing, err := a.GetMissingUpdates()
	if err != nil || len(missing) != 1 || missing[0].Attributes.Name != "私の <Game>" {
		t.Fatalf("updates = %v, %v", missing, err)
	}
	dlc, err := a.GetMissingDLC()
	if err != nil || len(dlc) != 1 || dlc[0].Attributes.Name != "私の <Game>" || dlc[0].MissingDLC[0].Name != "Expansion" {
		t.Fatalf("DLC = %v, %v", dlc, err)
	}
	if scenario == "reload" {
		data, err := os.ReadFile(filepath.Join(base, "settings.json"))
		if err != nil {
			t.Fatal(err)
		}
		var persisted settings.AppSettings
		if err := json.Unmarshal(data, &persisted); err != nil || persisted.TitleNameOverrides[detailsBaseID] != "私の <Game>" {
			t.Fatalf("persisted = %v, %v", persisted, err)
		}
		return
	}
	// Full settings saves retain overrides and do not require a restart.
	candidate := normalizeSettings(*current)
	candidate.GUI.Theme = settings.ThemeDark
	saved, err := a.SaveSettings(candidate)
	if err != nil || saved.RestartRequired || saved.Settings.TitleNameOverrides[detailsBaseID] != "私の <Game>" {
		t.Fatalf("settings = %v, %v", saved, err)
	}
	result, err = a.SetGameNameOverride(detailsBaseID, "")
	if err != nil || result.Details.Name != "Catalog" || result.Details.CustomName != "" || len(result.TitleNameOverrides) != 0 {
		t.Fatalf("remove = %#v, %v", result, err)
	}
	// Overrides also work without any catalog name, using the same automatic fallback.
	a.state.switchDB.TitlesMap[detailsPrefix].Attributes.Name = ""
	result, err = a.SetGameNameOverride(detailsBaseID, "Local custom")
	if err != nil || result.Details.AutomaticName != "Fallback" {
		t.Fatalf("local fallback = %#v, %v", result, err)
	}
	delete(a.state.localDB.TitlesMap, detailsPrefix)
	a.state.switchDB.TitlesMap[detailsPrefix].Attributes.Name = "Catalog"
	games, err := a.GetMissingGames()
	if err != nil || len(games) != 1 || games[0].Name != "Local custom" {
		t.Fatalf("missing = %v, %v", games, err)
	}
}
