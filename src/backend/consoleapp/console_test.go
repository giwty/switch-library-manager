package consoleapp

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/process"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestOrganizationEnabledForMoveOnlyOption(t *testing.T) {
	if organizationEnabled(settings.OrganizationSettings{MoveScanFilesToLibrary: true}) != true {
		t.Fatal("move-only organization should be enabled")
	}
	if organizationEnabled(settings.OrganizationSettings{}) {
		t.Fatal("organization should stay disabled when all organization options are off")
	}
}

func TestCsvExportFilename(t *testing.T) {
	date := time.Date(2026, time.September, 21, 23, 45, 0, 0, time.FixedZone("test", 2*60*60))
	tests := map[string]string{
		csvGamesKind:          "slm_games.2026-09-21.csv",
		csvMissingGamesKind:   "slm_missing_games.2026-09-21.csv",
		csvMissingUpdatesKind: "slm_missing_updates.2026-09-21.csv",
		csvMissingDLCKind:     "slm_missing_dlc.2026-09-21.csv",
		csvIssuesKind:         "slm_issues.2026-09-21.csv",
	}

	for kind, want := range tests {
		t.Run(kind, func(t *testing.T) {
			if got := csvExportFilename(kind, date); got != want {
				t.Fatalf("csvExportFilename(%q) = %q, want %q", kind, got, want)
			}
		})
	}
}

func TestProcessIssuesExportsOrganizationConflicts(t *testing.T) {
	output := filepath.Join(t.TempDir(), "issues.csv")
	conflicts := []process.OrganizationConflict{{
		Source:      `C:\library\source.nsp`,
		Destination: `C:\library\Game.nsp`,
		OtherSource: `C:\library\Game.nsp`,
		Reason:      "destination already exists",
	}}
	c := CreateConsole("", nil, nil)
	c.processIssues(&db.LocalSwitchFilesDB{Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}, output, conflicts)

	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1][2] != "organization-conflict" || !strings.Contains(records[1][1], `C:\library\Game.nsp`) {
		t.Fatalf("issue export = %#v, want organization conflict with destination", records)
	}
}

func TestCreateCsvFileWritesUTF8BOMAndRoundTripsUnicode(t *testing.T) {
	output := filepath.Join(t.TempDir(), "unicode.csv")
	header := []string{"Title", "Notes"}
	row := []string{"日本語 café åäö", "comma, quote \" and newline\nremain intact"}

	csvFile := CreateCsvFile(output, header)
	if csvFile == nil {
		t.Fatal("CreateCsvFile returned nil")
	}
	csvFile.Write(row)
	csvFile.Close()

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	bom := []byte{0xEF, 0xBB, 0xBF}
	if !bytes.HasPrefix(data, bom) {
		t.Fatalf("CSV does not start with a UTF-8 BOM: % x", data[:min(len(data), len(bom))])
	}
	if bytes.HasPrefix(data[len(bom):], bom) {
		t.Fatal("CSV contains multiple leading UTF-8 BOMs")
	}
	content := data[len(bom):]
	if !utf8.Valid(content) {
		t.Fatal("CSV content after BOM is not valid UTF-8")
	}

	records, err := csv.NewReader(bytes.NewReader(content)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{header, row}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("CSV records = %#v, want %#v", records, want)
	}
}

func TestCreateCsvFileWritesBOMForHeaderOnlyExport(t *testing.T) {
	output := filepath.Join(t.TempDir(), "header-only.csv")
	csvFile := CreateCsvFile(output, []string{"Title", "TitleId"})
	if csvFile == nil {
		t.Fatal("CreateCsvFile returned nil")
	}
	csvFile.Close()

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("header-only CSV does not start with a UTF-8 BOM: % x", data)
	}
	records, err := csv.NewReader(bytes.NewReader(data[3:])).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(records, [][]string{{"Title", "TitleId"}}) {
		t.Fatalf("header-only CSV records = %#v", records)
	}
}

func TestCreateCsvFileFailureAndDisabledExportAreSafe(t *testing.T) {
	output := filepath.Join(t.TempDir(), "missing", "cannot-create.csv")
	if csvFile := CreateCsvFile(output, []string{"Title"}); csvFile != nil {
		t.Fatal("CreateCsvFile should return nil when the output file cannot be created")
	}

	disabled := CreateCsvFile("", []string{"Title"})
	if disabled != nil {
		t.Fatal("CreateCsvFile should return nil when export is disabled")
	}
	disabled.Write([]string{"safe no-op"})
	disabled.Close()
}

func TestFlattenMissingDLC(t *testing.T) {
	titles := map[string]process.IncompleteTitle{
		"game-z": {
			Attributes: db.TitleAttributes{Id: "game-z-id", Name: "Zulu"},
			MissingDLC: []process.MissingDLC{
				{Id: "dlc-z-id", Name: "Expansion"},
				{Id: "dlc-a-id", Name: "Adventure"},
				{Id: "dlc-y-id", Name: "Expansion"},
			},
		},
		"game-a": {
			Attributes: db.TitleAttributes{Id: "game-a-id", Name: "Alpha"},
			MissingDLC: []process.MissingDLC{{Id: "dlc-b-id", Name: "Bonus"}},
		},
	}

	rows := flattenMissingDLC(titles)
	if len(rows) != 4 {
		t.Fatalf("flattened missing DLC rows = %d, want 4", len(rows))
	}
	want := []struct{ game, gameID, dlc, dlcID string }{
		{"Alpha", "game-a-id", "Bonus", "dlc-b-id"},
		{"Zulu", "game-z-id", "Adventure", "dlc-a-id"},
		{"Zulu", "game-z-id", "Expansion", "dlc-y-id"},
		{"Zulu", "game-z-id", "Expansion", "dlc-z-id"},
	}
	for i, row := range rows {
		if row.Game.Name != want[i].game || row.Game.Id != want[i].gameID || row.DLC.Name != want[i].dlc || row.DLC.Id != want[i].dlcID {
			t.Errorf("row %d = (%q, %q, %q, %q), want (%q, %q, %q, %q)", i, row.Game.Name, row.Game.Id, row.DLC.Name, row.DLC.Id, want[i].game, want[i].gameID, want[i].dlc, want[i].dlcID)
		}
		csvRow := missingDLCCSVRow(row)
		if len(csvRow) != 3 || csvRow[0] != want[i].game || csvRow[1] != want[i].gameID || csvRow[2] != want[i].dlc+" ["+want[i].dlcID+"]" {
			t.Errorf("CSV row %d = %#v", i, csvRow)
		}
	}
}
