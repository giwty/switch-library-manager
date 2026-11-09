package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/process"
	"github.com/trembon/switch-library-manager/backend/settings"
)

// These are real exported DTOs, serialized exactly as Wails sends them. The
// checked-in output is also consumed by the headless browser bridge.
type frontendFixture struct {
	Settings            settings.AppSettings      `json:"settings"`
	SettingsSaved       SettingsSaveResult        `json:"settingsSaved"`
	SettingsRestart     SettingsSaveResult        `json:"settingsRestart"`
	Library             LocalLibraryData          `json:"library"`
	MissingGames        []SwitchTitle             `json:"missingGames"`
	MissingUpdates      []process.IncompleteTitle `json:"missingUpdates"`
	MissingDLC          []process.IncompleteTitle `json:"missingDLC"`
	Details             GameDetails               `json:"details"`
	Preview             OrganizationPreview       `json:"preview"`
	Organization        OrganizationResult        `json:"organization"`
	BlockedOrganization OrganizationResult        `json:"blockedOrganization"`
	Progress            ProgressUpdate            `json:"progress"`
}

func TestFrontendFixtureContracts(t *testing.T) {
	attributes := db.TitleAttributes{Id: detailsBaseID, Name: "Amber Adventure", Region: "USA", Publisher: "Synthetic Studio", ParsedReleaseDate: "2026-01-01", BannerUrl: "/test-icon.svg"}
	rows := []LibraryTemplateData{}
	for i, name := range []string{"Amber Adventure", "Birch & Bloom", "Cobalt Quest"} {
		rows = append(rows, LibraryTemplateData{Id: i + 1, Name: name, TitleId: []string{detailsBaseID, "0100000000020000", "0100000000030000"}[i], Path: name + ".nsp", Icon: "/test-icon.svg", Region: "USA", Type: "NSP", Version: "1.0.0"})
	}
	library := LocalLibraryData{LibraryData: rows, Issues: []Pair{{Key: "/synthetic/library/notes.txt", Value: "file type is not supported"}}, NumFiles: 4}
	incomplete := process.IncompleteTitle{Attributes: attributes, LatestUpdate: 65536, LatestUpdateDate: "2026-02-01", MissingDLC: []process.MissingDLC{{Id: detailsDLCID, Name: "Amber Expansion", ReleaseDate: "2026-03-01"}}}
	fixture := frontendFixture{
		Settings: settings.AppSettings{
			SchemaVersion:  settings.SETTINGS_SCHEMA_VERSION,
			GUI:            settings.GUISettings{Enabled: true, PageSize: 2, Theme: settings.ThemeLight},
			Paths:          settings.PathSettings{LibraryFolder: "/synthetic/library", ScanFolders: []string{}},
			Scan:           settings.ScanSettings{IgnoreFileTypes: []string{}, RescanOnStartup: true},
			Organization:   settings.OrganizationSettings{RenameFiles: true, FolderNameTemplate: "{TITLE_NAME}", FileNameTemplate: "{TITLE_NAME} [{TITLE_ID}]"},
			MissingContent: settings.MissingContentSettings{CheckForUpdates: true, CheckForDLC: true, IgnoreDLCTitleIDs: []string{}, IgnoreUpdateIDs: []string{}},
			DataSources:    settings.DataSourceSettings{TitlesURL: "https://example.invalid/titles.json", VersionsURL: "https://example.invalid/versions.json"},
		},
		Library:        library,
		MissingGames:   []SwitchTitle{{Name: "Distant Demo", TitleId: "0100000000040000", Icon: "/test-icon.svg", Region: "EUR", ReleaseDate: "2026-04-01"}},
		MissingUpdates: []process.IncompleteTitle{incomplete}, MissingDLC: []process.IncompleteTitle{incomplete},
		Details:             GameDetails{Attributes: attributes, TitleID: detailsBaseID, Name: attributes.Name, ReleaseDateText: "2026-01-01", RemoteAvailable: true, MetadataAvailable: true, BaseCollected: true, BasePath: "/synthetic/library/Amber Adventure.nsp", Updates: []GameUpdateDetail{{Version: 65536, Date: "2026-02-01", Status: "missing"}}, LocalOnlyUpdates: []GameUpdateDetail{}, DLC: []GameDLCDetail{{TitleID: detailsDLCID, Name: "Amber Expansion", Attributes: db.TitleAttributes{Id: detailsDLCID}, Status: "missing"}}, LocalOnlyDLC: []GameDLCDetail{}},
		Preview:             OrganizationPreview{RootFolder: "/synthetic/library", Entries: []OrganizationPreviewEntry{{Kind: "game", Path: "Amber Adventure [0100000000010000].nsp"}}},
		Organization:        OrganizationResult{Status: "completed", Library: library},
		BlockedOrganization: OrganizationResult{Status: "blocked", ConflictCount: 1, Library: LocalLibraryData{LibraryData: rows, NumFiles: 4, Issues: []Pair{{Key: "/synthetic/library/conflict.nsp", Value: "organization conflict: destination exists"}}}},
		Progress:            ProgressUpdate{Curr: 1, Total: 2, Message: "Synthetic scan"},
	}
	fixture.SettingsSaved = SettingsSaveResult{Settings: fixture.Settings, RestartReasons: []string{}}
	restartSettings := fixture.Settings
	restartSettings.Paths.LibraryFolder = "/synthetic/another-library"
	fixture.SettingsRestart = SettingsSaveResult{Settings: restartSettings, RestartRequired: true, RestartReasons: []string{"Library folder"}}
	want, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	filename := filepath.Join("..", "..", "frontend_tests", "ui", "fixtures.json")
	if os.Getenv("SLM_UPDATE_UI_FIXTURES") == "1" {
		if err := os.WriteFile(filename, want, 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	// Normalize line endings for Windows checkouts, without accepting JSON drift.
	if !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want) {
		t.Fatal("headless UI fixtures differ from Go DTOs; review and regenerate with SLM_UPDATE_UI_FIXTURES=1 go test ./backend/app -run '^TestFrontendFixtureContracts$'")
	}
}
