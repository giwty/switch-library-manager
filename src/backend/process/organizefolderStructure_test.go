package process

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
	"go.uber.org/zap"
)

type progressEvent struct {
	curr    int
	total   int
	message string
}

type progressRecorder struct {
	events []progressEvent
}

func (p *progressRecorder) UpdateProgress(curr int, total int, message string) {
	p.events = append(p.events, progressEvent{curr: curr, total: total, message: message})
}

func TestApplyTemplate(t *testing.T) {
	data := map[string]string{
		settings.TEMPLATE_TITLE_NAME:       "The Game / Demo",
		settings.TEMPLATE_TITLE_ID:         "0100abcd00001000",
		settings.TEMPLATE_VERSION:          "12",
		settings.TEMPLATE_VERSION_TXT:      "1.2.0",
		settings.TEMPLATE_REGION:           "US",
		settings.TEMPLATE_TYPE:             "UPD",
		settings.TEMPLATE_DLC_NAME:         "The Game - Extra Pack",
		settings.TEMPLATE_SIZE_GB:          "0.6GB",
		settings.TEMPLATE_SIZE_MB:          "600MB",
		settings.TEMPLATE_PACKAGE_CONTENTS: "1G+1U+1D",
	}

	tests := []struct {
		name     string
		safe     bool
		template string
		try      int
		want     string
	}{
		{"all placeholders and repeated values", false, "{TITLE_NAME}-{TITLE_NAME}-{TITLE_ID}-{VERSION}-{VERSION_TXT}-{REGION}-{TYPE}-{DLC_NAME}-{SIZE_GB}-{SIZE_MB}-{SIZE_MB} ({PACKAGE_CONTENTS})", 0, "The Game  Demo-The Game  Demo-0100ABCD00001000-12-1.2.0-US-UPD-The Game - Extra Pack-0.6GB-600MB-600MB (1G+1U+1D)"},
		{"package contents placeholder omitted", false, "{TITLE_NAME}[{TITLE_ID}]", 0, "The Game  Demo[0100ABCD00001000]"},
		{"safe name", true, "Pokémon™: {TITLE_NAME} [v{VERSION}]", 0, "Pokémon The Game Demo [v12]"},
		{"collision suffix", false, "name.", 2, "name(2)"},
		{"cleanup and illegal characters", false, "[]()<>a:b%?", 0, "ab"},
		{"trailing periods", false, "name...", 0, "name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := applyTemplate(data, tt.safe, tt.template, tt.try); got != tt.want {
				t.Fatalf("applyTemplate() = %q, want %q", got, tt.want)
			}
		})
	}

	emptyData := map[string]string{
		settings.TEMPLATE_TITLE_NAME:       "The Game",
		settings.TEMPLATE_PACKAGE_CONTENTS: "",
	}
	if got := applyTemplate(emptyData, false, "{TITLE_NAME} {PACKAGE_CONTENTS}", 0); got != "The Game" {
		t.Fatalf("applyTemplate() with empty package contents = %q, want %q", got, "The Game")
	}
}

func TestApplyTemplateTrimsDLCNameEdges(t *testing.T) {
	tests := []struct {
		name    string
		dlcName string
		want    string
	}{
		{name: "ASCII separator", dlcName: "Game - Expansion", want: "Game (Expansion)"},
		{name: "Unicode separator", dlcName: "Game – Expansion", want: "Game (Expansion)"},
		{name: "game name at end", dlcName: "Expansion - Game", want: "Game (Expansion)"},
		{name: "repeated separators", dlcName: "Game --- Expansion", want: "Game (Expansion)"},
		{name: "Unicode whitespace", dlcName: "\u00a0Game —\u00a0Expansion\u00a0", want: "Game (Expansion)"},
		{name: "internal hyphens preserved", dlcName: "Game - Expansion - Pack", want: "Game (Expansion - Pack)"},
		{name: "empty after trimming", dlcName: "Game —", want: "Game"},
	}

	for _, test := range tests {
		for _, safe := range []bool{false, true} {
			t.Run(test.name+"/safe="+strconv.FormatBool(safe), func(t *testing.T) {
				data := map[string]string{
					settings.TEMPLATE_TITLE_NAME: "Game",
					settings.TEMPLATE_DLC_NAME:   test.dlcName,
				}
				if got := applyTemplate(data, safe, "{TITLE_NAME} ({DLC_NAME})", 0); got != test.want {
					t.Fatalf("applyTemplate() = %q, want %q", got, test.want)
				}
			})
		}
	}
}

func TestApplyTemplateTrimsColonSeparatedDLCNames(t *testing.T) {
	tests := []struct {
		name      string
		titleName string
		dlcName   string
		titleID   string
		wantDLC   string
	}{
		{
			name:      "Aquapark io",
			titleName: "Aquapark io",
			dlcName:   "Aquapark io: Animals DLC",
			titleID:   "010027B01B997003",
			wantDLC:   "Aquapark io (Animals DLC)[010027B01B997003][v0]",
		},
		{
			name:      "Aquarium Land",
			titleName: "Aquarium Land",
			dlcName:   "Aquarium Land: Puppy",
			titleID:   "0100BAE01C1FB001",
			wantDLC:   "Aquarium Land (Puppy)[0100BAE01C1FB001][v0]",
		},
		{
			name:      "Across the Obelisk",
			titleName: "Across the Obelisk",
			dlcName:   "Across the Obelisk: The Wolf Wars",
			titleID:   "01004FA01D2C7002",
			wantDLC:   "Across the Obelisk (The Wolf Wars)[01004FA01D2C7002][v0]",
		},
		{
			name:      "repeated leading and trailing colons",
			titleName: "Game",
			dlcName:   "::: Game::: Expansion :::",
			titleID:   "0100000000001001",
			wantDLC:   "Game (Expansion)[0100000000001001][v0]",
		},
		{
			name:      "mixed edge separators and internal hyphen",
			titleName: "Game",
			dlcName:   " : -Game : - — Expansion - Pack — - : ",
			titleID:   "0100000000001001",
			wantDLC:   "Game (Expansion - Pack)[0100000000001001][v0]",
		},
		{
			name:      "empty after trimming removes parentheses",
			titleName: "Game",
			dlcName:   "Game : — : ",
			titleID:   "0100000000001001",
			wantDLC:   "Game [0100000000001001][v0]",
		},
	}

	for _, test := range tests {
		for _, safe := range []bool{false, true} {
			t.Run(test.name+"/safe="+strconv.FormatBool(safe), func(t *testing.T) {
				data := map[string]string{
					settings.TEMPLATE_TITLE_NAME: test.titleName,
					settings.TEMPLATE_TITLE_ID:   test.titleID,
					settings.TEMPLATE_VERSION:    "0",
					settings.TEMPLATE_DLC_NAME:   test.dlcName,
				}
				got := applyTemplate(data, safe, "{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]", 0)
				if got != test.wantDLC {
					t.Fatalf("applyTemplate() = %q, want %q", got, test.wantDLC)
				}
			})
		}
	}
}

func TestSetFileSizeTemplateData(t *testing.T) {
	tests := []struct {
		name string
		size int64
		gb   string
		mb   string
	}{
		{"600 MB", 600_000_000, "0.6GB", "600MB"},
		{"2.7 GB", 2_700_000_000, "2.7GB", "2700MB"},
		{"halfway GB rounds up", 1_250_000_000, "1.3GB", "1250MB"},
		{"halfway MB rounds up", 1_500_000, "0.0GB", "2MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]string{}
			setFileSizeTemplateData(data, tt.size)
			if got := data[settings.TEMPLATE_SIZE_GB]; got != tt.gb {
				t.Errorf("SIZE_GB = %q, want %q", got, tt.gb)
			}
			if got := data[settings.TEMPLATE_SIZE_MB]; got != tt.mb {
				t.Errorf("SIZE_MB = %q, want %q", got, tt.mb)
			}
		})
	}
}

func TestTemplateHelpers(t *testing.T) {
	options := settings.OrganizeOptions{
		RenameFiles:        true,
		FolderNameTemplate: "{TITLE_NAME}",
		FileNameTemplate:   "{TITLE_ID}-{VERSION}",
	}
	data := map[string]string{settings.TEMPLATE_TITLE_NAME: "Game", settings.TEMPLATE_TITLE_ID: "abcd", settings.TEMPLATE_VERSION: "3"}

	if got := getFolderName(options, data); got != "Game" {
		t.Fatalf("getFolderName() = %q", got)
	}
	if got := getFileName(options, "original.nsp", data, 0); got != "ABCD-3.nsp" {
		t.Fatalf("getFileName() = %q", got)
	}
	options.RenameFiles = false
	if got := getFileName(options, "original.nsp", data, 4); got != "original.nsp" {
		t.Fatalf("getFileName() with rename disabled = %q", got)
	}
}

func TestPackageContents(t *testing.T) {
	base := db.ExtendedFileInfo{BaseFolder: "library", FileName: "package.xci"}
	other := db.ExtendedFileInfo{BaseFolder: "library", FileName: "standalone.nsp"}
	file := func(name string) db.ExtendedFileInfo {
		return db.ExtendedFileInfo{BaseFolder: base.BaseFolder, FileName: name}
	}

	tests := []struct {
		name string
		game *db.SwitchGameFiles
		want string
	}{
		{
			name: "base only ignores standalone content",
			game: &db.SwitchGameFiles{
				BaseExist: true,
				File:      db.SwitchFileInfo{ExtendedInfo: base},
				Updates:   map[int]db.SwitchFileInfo{1: {ExtendedInfo: other}},
				Dlc:       map[string]db.SwitchFileInfo{"dlc": {ExtendedInfo: other}},
			},
		},
		{
			name: "one bundled update",
			game: &db.SwitchGameFiles{
				BaseExist: true,
				File:      db.SwitchFileInfo{ExtendedInfo: base},
				Updates:   map[int]db.SwitchFileInfo{1: {ExtendedInfo: base}},
			},
			want: "1G+1U",
		},
		{
			name: "multiple bundled updates and DLCs",
			game: &db.SwitchGameFiles{
				BaseExist: true,
				File:      db.SwitchFileInfo{ExtendedInfo: base},
				Updates: map[int]db.SwitchFileInfo{
					1: {ExtendedInfo: base},
					2: {ExtendedInfo: base},
					3: {ExtendedInfo: other},
				},
				Dlc: map[string]db.SwitchFileInfo{
					"dlc-one":     {ExtendedInfo: base},
					"dlc-two":     {ExtendedInfo: base},
					"dlc-outside": {ExtendedInfo: other},
				},
			},
			want: "1G+2U+2D",
		},
		{
			name: "filename fallback preserves recognized marker",
			game: &db.SwitchGameFiles{
				BaseExist: true,
				File:      db.SwitchFileInfo{ExtendedInfo: file("Game [0100000000010000][v1] (1g+1u+2d).nsp")},
			},
			want: "1G+1U+2D",
		},
		{
			name: "filename fallback ignores unrecognized order",
			game: &db.SwitchGameFiles{
				BaseExist: true,
				File:      db.SwitchFileInfo{ExtendedInfo: file("Game (1G+1D+1U).nsp")},
			},
		},
		{
			name: "missing base has no package marker",
			game: &db.SwitchGameFiles{File: db.SwitchFileInfo{ExtendedInfo: file("Game (1G+1U).nsp")}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := packageContents(tt.game); got != tt.want {
				t.Fatalf("packageContents() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOrganizeByFoldersExpandsPackageContentsOnlyForBundledPackage(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "incoming")
	mustMkdir(t, source)

	baseName := "package.xci"
	standaloneUpdateName := "standalone-update.nsp"
	standaloneDLCName := "standalone-dlc.nsp"
	for _, name := range []string{baseName, standaloneUpdateName, standaloneDLCName} {
		writeFixture(t, filepath.Join(source, name), "synthetic content")
	}

	baseFile := db.ExtendedFileInfo{BaseFolder: source, FileName: baseName}
	standaloneUpdate := db.ExtendedFileInfo{BaseFolder: source, FileName: standaloneUpdateName}
	standaloneDLC := db.ExtendedFileInfo{BaseFolder: source, FileName: standaloneDLCName}
	setProcessSettings(t, baseFolder, settings.OrganizeOptions{
		RenameFiles:           true,
		FileNameTemplate:      "{TITLE_NAME}_{TYPE} ({PACKAGE_CONTENTS})",
		IncludeBaseType:       true,
		AllowUnicodeFileNames: true,
	})

	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"game": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: baseFile, Metadata: contentMetadata("0100000000010000", 0, "1.0.0")},
			Updates: map[int]db.SwitchFileInfo{
				1: {ExtendedInfo: baseFile},
				2: {ExtendedInfo: baseFile},
				3: {ExtendedInfo: standaloneUpdate},
			},
			Dlc: map[string]db.SwitchFileInfo{
				"dlc-one":   {ExtendedInfo: baseFile},
				"dlc-two":   {ExtendedInfo: baseFile},
				"dlc-three": {ExtendedInfo: standaloneDLC},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"game": {Attributes: db.TitleAttributes{Id: "0100000000010000", Name: "Example Game"}},
	}}

	OrganizeByFolders(baseFolder, local, remote, nil)

	assertMoved(t,
		filepath.Join(source, baseName),
		filepath.Join(source, "Example Game_BASE (1G+2U+2D).xci"),
		"bundled package with placeholder",
	)
	assertMoved(t,
		filepath.Join(source, standaloneUpdateName),
		filepath.Join(source, "Example Game_UPD.nsp"),
		"standalone update without package marker",
	)
	assertMoved(t,
		filepath.Join(source, standaloneDLCName),
		filepath.Join(source, "Example Game_DLC.nsp"),
		"standalone DLC without package marker",
	)
}

func TestIsOptionsValid(t *testing.T) {
	tests := []struct {
		name string
		opt  settings.OrganizeOptions
		want bool
	}{
		{"defaults", settings.OrganizeOptions{}, true},
		{"rename requires a template", settings.OrganizeOptions{RenameFiles: true}, false},
		{"rename requires title identity", settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{VERSION}"}, false},
		{"valid file template", settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_ID}"}, true},
		{"folder requires a template", settings.OrganizeOptions{CreateFolderPerGame: true}, false},
		{"folder requires title identity", settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{REGION}"}, false},
		{"valid folder template", settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"}, true},
		{"both valid", settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_NAME}", CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_ID}"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOptionsValid(tt.opt); got != tt.want {
				t.Fatalf("IsOptionsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTitleNameAndDLCNameHelpers(t *testing.T) {
	metadata := contentMetadata("0100abcd00001000", 3, "3.0.0")
	metadata.Ncap.TitleName["AmericanEnglish"] = switchfs.NacpTitle{Title: "Name From NACP"}
	local := &db.SwitchGameFiles{BaseExist: true, File: db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "Name From File [0100abcd00001000][v3].nsp"}, Metadata: metadata}}

	remote := &db.SwitchTitle{Attributes: db.TitleAttributes{Id: metadata.TitleId, Name: "Remote Name"}, Dlc: map[string]db.TitleAttributes{
		"0100abcd00001a01": {Name: "Remote Name - Expansion\nPack"},
	}}
	if got := db.ResolveGameTitleName(remote, local, false); got != "Remote Name" {
		t.Fatalf("remote title name = %q", got)
	}
	remote.Attributes.Name = "日本語"
	if got := db.ResolveGameTitleName(remote, local, false); got != "Name From NACP" {
		t.Fatalf("NACP title name = %q", got)
	}
	local.File.Metadata.Ncap.TitleName = nil
	if got := db.ResolveGameTitleName(nil, local, false); got != "Name From File" {
		t.Fatalf("filename title name = %q", got)
	}
	if got := db.ResolveGameTitleName(nil, nil, false); got != "Unknown Title" {
		t.Fatalf("nil title name = %q", got)
	}

	if got := getDlcName(remote, db.SwitchFileInfo{Metadata: metadataForID("0100abcd00001a01", 1)}); got != "Remote Name - Expansion Pack" {
		t.Fatalf("DLC name = %q", got)
	}
	if got := getDlcName(remote, db.SwitchFileInfo{Metadata: metadataForID("missing", 1)}); got != "" {
		t.Fatalf("unknown DLC name = %q", got)
	}
	if got := getDlcName(nil, db.SwitchFileInfo{}); got != "" {
		t.Fatalf("nil DLC name = %q", got)
	}
}

func TestOrganizeByFoldersMovesBaseUpdateAndDLC(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "incoming")
	mustMkdir(t, source)

	baseID := "0100E95004039000"
	updateID := "0100E95004039800"
	dlcOneID := "0100E9500403A001"
	dlcTwoID := "0100E9500403A002"
	baseName := "base.nsp"
	updateName := "update.nsp"
	dlcOneName := "dlc-one.nsp"
	dlcTwoName := "dlc-two.nsp"
	for _, name := range []string{baseName, updateName, dlcOneName, dlcTwoName} {
		writeFixture(t, filepath.Join(source, name), name)
	}

	options := settings.OrganizeOptions{
		CreateFolderPerGame:   true,
		FolderNameTemplate:    "{TITLE_NAME}",
		RenameFiles:           true,
		FileNameTemplate:      "{TITLE_ID}_{TYPE}_{VERSION}_{VERSION_TXT}_{SIZE_GB}_{SIZE_MB}",
		IncludeBaseType:       true,
		UpdatesFolder:         "updates",
		DlcFolder:             "dlc",
		AllowUnicodeFileNames: true,
		DeleteEmptyFolders:    true,
	}
	setProcessSettings(t, baseFolder, options)

	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: baseName, Size: 600_000_000}, Metadata: contentMetadata(baseID, 0, "1.0.0")},
			BaseExist: true,
			Updates:   map[int]db.SwitchFileInfo{5: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: updateName, Size: 2_700_000_000}, Metadata: contentMetadata(updateID, 5, "5.0.0")}},
			Dlc: map[string]db.SwitchFileInfo{
				dlcOneID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: dlcOneName, Size: 1_200_000_000}, Metadata: metadataForID(dlcOneID, 0)},
				dlcTwoID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: dlcTwoName, Size: 3_000_000_000}, Metadata: metadataForID(dlcTwoID, 1)},
			},
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: baseID, Name: "Test Game", Region: "US"}, Dlc: map[string]db.TitleAttributes{
			dlcOneID: {Id: dlcOneID, Name: "Test Game - Expansion"},
			dlcTwoID: {Id: dlcTwoID, Name: "Test Game - Expansion"},
		}},
	}}

	var progress progressRecorder
	OrganizeByFolders(baseFolder, local, remote, &progress)

	destination := filepath.Join(baseFolder, "Test Game")
	assertMoved(t, filepath.Join(source, baseName), filepath.Join(destination, baseID+"_BASE_0_1.0.0_0.6GB_600MB.nsp"), "base")
	assertMoved(t, filepath.Join(source, updateName), filepath.Join(destination, "updates", updateID+"_UPD_5_5.0.0_2.7GB_2700MB.nsp"), "update")
	assertMoved(t, filepath.Join(source, dlcOneName), filepath.Join(destination, "dlc", dlcOneID+"_DLC_0_1.0.0_1.2GB_1200MB.nsp"), "v0 DLC")
	assertMoved(t, filepath.Join(source, dlcTwoName), filepath.Join(destination, "dlc", dlcTwoID+"_DLC_1__3.0GB_3000MB.nsp"), "nonzero DLC without a display version")
	if len(progress.events) == 0 || progress.events[len(progress.events)-1].message != "done" {
		t.Fatalf("organization progress did not finish: %#v", progress.events)
	}

	OrganizeByFolders(baseFolder, local, remote, nil)
}

func TestSetDlcVersionTemplateData(t *testing.T) {
	tests := []struct {
		name            string
		metadata        *switchfs.ContentMetaAttributes
		wantVersion     string
		wantDisplayText string
	}{
		{name: "missing metadata", wantVersion: "0"},
		{name: "v0 without NACP", metadata: metadataForID("0100E9500403A001", 0), wantVersion: "0", wantDisplayText: "1.0.0"},
		{name: "v0 with empty display version", metadata: contentMetadata("0100E9500403A001", 0, ""), wantVersion: "0", wantDisplayText: "1.0.0"},
		{name: "v0 with display version", metadata: contentMetadata("0100E9500403A001", 0, "2.3.4"), wantVersion: "0", wantDisplayText: "2.3.4"},
		{name: "nonzero without display version", metadata: metadataForID("0100E9500403A001", 1), wantVersion: "1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			templateData := map[string]string{
				settings.TEMPLATE_VERSION:     "previous version",
				settings.TEMPLATE_VERSION_TXT: "previous display version",
			}
			setDlcVersionTemplateData(templateData, db.SwitchFileInfo{Metadata: test.metadata})
			if got := templateData[settings.TEMPLATE_VERSION]; got != test.wantVersion {
				t.Errorf("version = %q, want %q", got, test.wantVersion)
			}
			if got := templateData[settings.TEMPLATE_VERSION_TXT]; got != test.wantDisplayText {
				t.Errorf("display version = %q, want %q", got, test.wantDisplayText)
			}
		})
	}
}

func TestOrganizeByFoldersUsesHighestBundledUpdateVersion(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "incoming")
	mustMkdir(t, source)

	baseID := "010087E01FCD6000"
	updateID := "010087E01FCD6800"
	baseName := "Cuisineer [010087E01FCD6000][v0].xci"
	standaloneUpdateName := "Cuisineer [010087E01FCD6800][v196608].nsp"
	writeFixture(t, filepath.Join(source, baseName), "synthetic super XCI")
	writeFixture(t, filepath.Join(source, standaloneUpdateName), "synthetic patch NSP")
	baseFile := db.ExtendedFileInfo{BaseFolder: source, FileName: baseName, Size: 20}
	standaloneUpdateFile := db.ExtendedFileInfo{BaseFolder: source, FileName: standaloneUpdateName, Size: 18}

	setProcessSettings(t, baseFolder, settings.OrganizeOptions{
		RenameFiles:           true,
		FileNameTemplate:      "{TITLE_NAME} [{TITLE_ID}][{TYPE}][v{VERSION}][{VERSION_TXT}]",
		IncludeBaseType:       true,
		AllowUnicodeFileNames: true,
	})
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"010087e01fcd6": {
			BaseExist: true,
			File: db.SwitchFileInfo{
				ExtendedInfo: baseFile,
				Metadata:     contentMetadata(baseID, 0, "1.0.100"),
			},
			// Leave MultiContent false to ensure source identity, rather than
			// the aggregate flag, controls bundled update handling.
			Updates: map[int]db.SwitchFileInfo{
				65536: {
					ExtendedInfo: baseFile,
					Metadata:     contentMetadata(updateID, 65536, "1.0.185"),
				},
				196608: {
					ExtendedInfo: standaloneUpdateFile,
					Metadata:     contentMetadata(updateID, 196608, "2.0.27"),
				},
			},
			LatestUpdate: 196608,
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"010087e01fcd6": {Attributes: db.TitleAttributes{Id: baseID, Name: "Cuisineer"}},
	}}

	OrganizeByFolders(baseFolder, local, remote, nil)

	assertMoved(t,
		filepath.Join(source, baseName),
		filepath.Join(source, "Cuisineer [010087E01FCD6000][BASE][v65536][1.0.185].xci"),
		"bundled update XCI",
	)
	assertMoved(t,
		filepath.Join(source, standaloneUpdateName),
		filepath.Join(source, "Cuisineer [010087E01FCD6800][UPD][v196608][2.0.27].nsp"),
		"standalone update",
	)
}

func TestDeleteOldUpdatesPreservesPackageAndDeletesStandaloneUpdate(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "library")
	mustMkdir(t, source)
	packagePath := filepath.Join(source, "Cuisineer [010087E01FCD6000][v65536].xci")
	oldUpdatePath := filepath.Join(source, "Cuisineer [010087E01FCD6800][v32768].nsp")
	writeFixture(t, packagePath, "synthetic super XCI")
	writeFixture(t, oldUpdatePath, "synthetic old patch NSP")
	if err := settings.SaveSettingsWithError(&settings.AppSettings{
		Paths:        settings.PathSettings{LibraryFolder: source},
		Organization: settings.OrganizeOptions{DeleteEmptyFolders: true},
	}, baseFolder); err != nil {
		t.Fatal(err)
	}
	packageInfo := extendedInfoForPath(t, packagePath)
	oldUpdateInfo := extendedInfoForPath(t, oldUpdatePath)

	local := &db.LocalSwitchFilesDB{Skipped: map[db.ExtendedFileInfo]db.SkippedFile{
		oldUpdateInfo: {
			ReasonCode: db.REASON_OLD_UPDATE,
			ReasonText: "older standalone update",
		},
	}, CleanupCandidates: []db.CleanupCandidate{{File: oldUpdateInfo, Replacement: packageInfo, ReasonCode: db.REASON_OLD_UPDATE}}}
	if err := DeleteOldUpdates(baseFolder, local, nil); err != nil {
		t.Fatalf("DeleteOldUpdates() = %v", err)
	}

	assertExists(t, packagePath)
	assertNotExists(t, oldUpdatePath)
}

func TestOrganizeByFoldersProcessesMissingBaseAndNilMetadata(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "incoming")
	mustMkdir(t, source)
	updateID := "0100E95004039800"
	dlcID := "0100E9500403A001"
	updateName := "Missing Base [0100E95004039000][v7].nsp"
	dlcName := "missing-dlc.nsp"
	writeFixture(t, filepath.Join(source, updateName), "update")
	writeFixture(t, filepath.Join(source, dlcName), "dlc")
	options := settings.OrganizeOptions{
		CreateFolderPerGame:        true,
		FolderNameTemplate:         "{TITLE_NAME}",
		RenameFiles:                true,
		FileNameTemplate:           "{TITLE_ID}_{TYPE}_{VERSION}",
		UpdatesFolder:              "updates",
		DlcFolder:                  "dlc",
		ProcessWhenMissingBaseGame: true,
		AllowUnicodeFileNames:      true,
	}
	setProcessSettings(t, baseFolder, options)

	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "missing-base.nsp"}},
			Updates:   map[int]db.SwitchFileInfo{7: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: updateName}, Metadata: metadataForID(updateID, 7)}},
			Dlc:       map[string]db.SwitchFileInfo{dlcID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: dlcName}}},
			BaseExist: false,
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Missing Base"}},
	}}
	OrganizeByFolders(baseFolder, local, remote, nil)

	assertMoved(t, filepath.Join(source, updateName), filepath.Join(baseFolder, "Missing Base", "updates", updateID+"_UPD_7.nsp"), "missing-base update")
	assertMoved(t, filepath.Join(source, dlcName), filepath.Join(baseFolder, "Missing Base", "dlc", dlcID+"_DLC_0.nsp"), "metadata-free DLC")
}

func TestOrganizeByFoldersWithoutGameFoldersAndWithDlcCollision(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "incoming")
	updatesFolder := filepath.Join(baseFolder, "updates")
	dlcFolder := filepath.Join(baseFolder, "dlc")
	mustMkdir(t, source)
	mustMkdir(t, updatesFolder)
	mustMkdir(t, dlcFolder)
	baseName := "base.nsp"
	updateName := "update.nsp"
	dlcOneName := "one.nsp"
	dlcTwoName := "two.nsp"
	for _, name := range []string{baseName, updateName, dlcOneName, dlcTwoName} {
		writeFixture(t, filepath.Join(source, name), name)
	}
	baseID := "0100E95004039000"
	updateID := "0100E95004039800"
	dlcOneID := "0100E9500403A001"
	dlcTwoID := "0100E9500403A002"
	setProcessSettings(t, baseFolder, settings.OrganizeOptions{
		RenameFiles:           true,
		FileNameTemplate:      "{TITLE_ID}_{DLC_NAME}",
		AllowUnicodeFileNames: true,
		UpdatesFolder:         updatesFolder,
		DlcFolder:             dlcFolder,
	})
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: baseName}, Metadata: contentMetadata(baseID, 0, "")},
			Updates:   map[int]db.SwitchFileInfo{3: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: updateName}, Metadata: metadataForID(updateID, 3)}},
			Dlc: map[string]db.SwitchFileInfo{
				dlcOneID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: dlcOneName}, Metadata: metadataForID(dlcOneID, 1)},
				dlcTwoID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: dlcTwoName}, Metadata: metadataForID(dlcTwoID, 1)},
			},
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{"0100E95004039": {
		Attributes: db.TitleAttributes{Id: baseID, Name: "Game"},
		Dlc: map[string]db.TitleAttributes{
			dlcOneID: {Id: dlcOneID, Name: "Game - Expansion"},
			dlcTwoID: {Id: dlcTwoID, Name: "Game - Expansion"},
		},
	}}}
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertMoved(t, filepath.Join(source, baseName), filepath.Join(source, baseID+"_.nsp"), "base without folder")
	assertMoved(t, filepath.Join(source, updateName), filepath.Join(updatesFolder, updateID+"_.nsp"), "update outside folder")
	assertMoved(t, filepath.Join(source, dlcOneName), filepath.Join(dlcFolder, dlcOneID+"_Expansion.nsp"), "first colliding DLC")
	assertMoved(t, filepath.Join(source, dlcTwoName), filepath.Join(dlcFolder, dlcTwoID+"_Expansion.nsp"), "second colliding DLC")

	setProcessSettings(t, baseFolder, settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"})
	secondSource := filepath.Join(baseFolder, "bad-source")
	mustMkdir(t, secondSource)
	writeFixture(t, filepath.Join(secondSource, "bad.nsp"), "bad")
	writeFixture(t, filepath.Join(baseFolder, "occupied"), "not a folder")
	local.TitlesMap["0100E95004039"].File.ExtendedInfo = db.ExtendedFileInfo{BaseFolder: secondSource, FileName: "bad.nsp"}
	remote.TitlesMap["0100E95004039"].Attributes.Name = "occupied"
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertExists(t, filepath.Join(secondSource, "bad.nsp"))
}

func TestOrganizeByFoldersSplitFilesAndMissingBaseSkip(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "split-incoming")
	mustMkdir(t, source)
	writeFixture(t, filepath.Join(source, "game.00"), "part 0")
	writeFixture(t, filepath.Join(source, "game.01"), "part 1")
	writeFixture(t, filepath.Join(source, "keep.txt"), "keep")
	setProcessSettings(t, baseFolder, settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"})
	file := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "game.00"}, Metadata: contentMetadata("0100E95004039000", 0, "")}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"split": {File: file, BaseExist: true, IsSplit: true},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"split": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Split"}},
	}}
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertMoved(t, filepath.Join(source, "game.00"), filepath.Join(baseFolder, "Split", "game.00"), "split part zero")
	assertMoved(t, filepath.Join(source, "game.01"), filepath.Join(baseFolder, "Split", "game.01"), "split part one")
	assertExists(t, filepath.Join(source, "keep.txt"))
	missingSource := filepath.Join(baseFolder, "missing")
	mustMkdir(t, missingSource)
	writeFixture(t, filepath.Join(missingSource, "missing.nsp"), "missing")
	missing := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: missingSource, FileName: "missing.nsp"}}
	local = &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{"missing": {File: missing, BaseExist: false}}}
	remote = &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{"missing": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Missing"}}}}
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertExists(t, filepath.Join(missingSource, "missing.nsp"))
}

func TestOrganizeByFoldersSkipsMultiContentRecordsAndInvalidOptions(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "incoming")
	mustMkdir(t, source)
	name := "multi.nsp"
	writeFixture(t, filepath.Join(source, name), "multi")
	options := settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"}
	setProcessSettings(t, baseFolder, options)
	file := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: name}, Metadata: contentMetadata("0100E95004039000", 0, "1")}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {File: file, BaseExist: true, MultiContent: true, Updates: map[int]db.SwitchFileInfo{1: file}, Dlc: map[string]db.SwitchFileInfo{"dlc": file}},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Multi"}}}}
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertMoved(t, filepath.Join(source, name), filepath.Join(baseFolder, "Multi", name), "multi-content base")

	setProcessSettings(t, baseFolder, settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{VERSION}"})
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertExists(t, filepath.Join(baseFolder, "Multi", name))
}

func TestDeleteOldUpdates(t *testing.T) {
	baseFolder := t.TempDir()
	oldFolder := filepath.Join(baseFolder, "old")
	mustMkdir(t, oldFolder)
	oldPath := filepath.Join(oldFolder, "old.nsp")
	newPath := filepath.Join(oldFolder, "new.nsp")
	writeFixture(t, oldPath, "old")
	writeFixture(t, newPath, "new")
	setProcessSettings(t, baseFolder, settings.OrganizeOptions{DeleteEmptyFolders: true})
	oldInfo := extendedInfoForPath(t, oldPath)
	newInfo := extendedInfoForPath(t, newPath)
	var progress progressRecorder
	local := &db.LocalSwitchFilesDB{Skipped: map[db.ExtendedFileInfo]db.SkippedFile{
		oldInfo: {ReasonCode: db.REASON_OLD_UPDATE},
		{BaseFolder: oldFolder, FileName: "already-missing.nsp"}: {ReasonCode: db.REASON_OLD_UPDATE},
		{BaseFolder: oldFolder, FileName: "new.nsp"}:             {ReasonCode: db.REASON_DUPLICATE},
	}, CleanupCandidates: []db.CleanupCandidate{{File: oldInfo, Replacement: newInfo, ReasonCode: db.REASON_OLD_UPDATE}}}
	if err := DeleteOldUpdatesAt(baseFolder, oldFolder, local, &progress); err != nil {
		t.Fatalf("DeleteOldUpdatesAt() = %v", err)
	}
	assertNotExists(t, oldPath)
	assertExists(t, newPath)
	assertExists(t, oldFolder)
	if len(progress.events) != 1 {
		t.Fatalf("progress events = %d, want 1: %#v", len(progress.events), progress.events)
	}

	cleanupFolder := filepath.Join(baseFolder, "cleanup")
	mustMkdir(t, cleanupFolder)
	unverifiedPath := filepath.Join(cleanupFolder, "old.nsp")
	writeFixture(t, unverifiedPath, "old")
	if err := DeleteOldUpdatesAt(baseFolder, cleanupFolder, &db.LocalSwitchFilesDB{Skipped: map[db.ExtendedFileInfo]db.SkippedFile{
		{BaseFolder: cleanupFolder, FileName: "old.nsp"}: {ReasonCode: db.REASON_OLD_UPDATE},
	}}, nil); err != nil {
		t.Fatalf("DeleteOldUpdatesAt() without candidate = %v", err)
	}
	assertExists(t, unverifiedPath)
	assertExists(t, cleanupFolder)
}

func TestDeleteOldUpdatesRejectsCandidateChangedAfterScan(t *testing.T) {
	baseFolder := t.TempDir()
	cleanupRoot := filepath.Join(baseFolder, "library")
	mustMkdir(t, cleanupRoot)
	sourcePath := filepath.Join(cleanupRoot, "old.nsp")
	replacementPath := filepath.Join(cleanupRoot, "new.nsp")
	writeFixture(t, sourcePath, "old")
	writeFixture(t, replacementPath, "replacement")
	sourceInfo := extendedInfoForPath(t, sourcePath)
	replacementInfo := extendedInfoForPath(t, replacementPath)
	local := &db.LocalSwitchFilesDB{CleanupCandidates: []db.CleanupCandidate{{File: sourceInfo, Replacement: replacementInfo}}}
	writeFixture(t, sourcePath, "source changed after scan")
	setProcessSettings(t, baseFolder, settings.OrganizeOptions{})

	if err := DeleteOldUpdatesAt(baseFolder, cleanupRoot, local, nil); err == nil {
		t.Fatal("DeleteOldUpdatesAt() accepted a candidate changed after scan")
	}
	assertExists(t, sourcePath)
	assertExists(t, replacementPath)
}

func TestDeleteOldUpdatesAtRejectsMissingScanPaths(t *testing.T) {
	baseFolder := t.TempDir()
	cleanupRoot := filepath.Join(baseFolder, "library")
	mustMkdir(t, cleanupRoot)
	replacementPath := filepath.Join(cleanupRoot, "replacement.nsp")
	writeFixture(t, replacementPath, "replacement")
	replacementInfo := extendedInfoForPath(t, replacementPath)
	setProcessSettings(t, baseFolder, settings.OrganizeOptions{})
	if err := DeleteOldUpdatesAt(baseFolder, cleanupRoot, nil, nil); err == nil {
		t.Fatal("DeleteOldUpdatesAt(nil) returned nil")
	}
	local := &db.LocalSwitchFilesDB{CleanupCandidates: []db.CleanupCandidate{{
		File:        db.ExtendedFileInfo{BaseFolder: cleanupRoot, FileName: "missing.nsp"},
		Replacement: replacementInfo,
	}}}
	if err := DeleteOldUpdatesAt(baseFolder, cleanupRoot, local, nil); err == nil {
		t.Fatal("DeleteOldUpdatesAt() accepted a missing cleanup source")
	}
	local.CleanupCandidates[0].File = db.ExtendedFileInfo{BaseFolder: cleanupRoot, FileName: "existing.nsp"}
	writeFixture(t, filepath.Join(cleanupRoot, "existing.nsp"), "duplicate")
	local.CleanupCandidates[0].File = extendedInfoForPath(t, filepath.Join(cleanupRoot, "existing.nsp"))
	local.CleanupCandidates[0].Replacement = db.ExtendedFileInfo{BaseFolder: cleanupRoot, FileName: "missing-replacement.nsp"}
	if err := DeleteOldUpdatesAt(baseFolder, cleanupRoot, local, nil); err == nil {
		t.Fatal("DeleteOldUpdatesAt() accepted a missing replacement")
	}
	assertExists(t, filepath.Join(cleanupRoot, "existing.nsp"))
}

func TestFilesystemHelpers(t *testing.T) {
	baseFolder := t.TempDir()
	created := filepath.Join(baseFolder, "created")
	logger := zap.NewNop().Sugar()
	if err := createFolder(created, logger); err != nil {
		t.Fatalf("createFolder() = %v", err)
	}
	if err := createFolder(created, logger); err != nil {
		t.Fatalf("createFolder() existing = %v", err)
	}
	nonEmpty := filepath.Join(baseFolder, "non-empty")
	mustMkdir(t, nonEmpty)
	writeFixture(t, filepath.Join(nonEmpty, "file"), "file")
	if err := deleteEmptyFolder(nonEmpty); err != nil {
		t.Fatalf("deleteEmptyFolder(non-empty) = %v", err)
	}
	if err := deleteEmptyFolder(created); err != nil {
		t.Fatalf("deleteEmptyFolder(empty) = %v", err)
	}
	assertNotExists(t, created)
	if err := deleteEmptyFolder(filepath.Join(baseFolder, "missing")); err == nil {
		t.Fatal("deleteEmptyFolder(missing) returned nil")
	}

	from := filepath.Join(baseFolder, "from")
	to := filepath.Join(baseFolder, "to")
	writeFixture(t, from, "move")
	if err := moveFile(from, from); err != nil {
		t.Fatalf("moveFile(same path) = %v", err)
	}
	if err := moveFile(from, to); err != nil {
		t.Fatalf("moveFile() = %v", err)
	}
	assertExists(t, to)
	if err := moveFile(from, filepath.Join(baseFolder, "missing", "to")); err == nil {
		t.Fatal("moveFile(missing destination) returned nil")
	}
	invalidSource := filepath.Join(baseFolder, "invalid-source")
	writeFixture(t, invalidSource, "remain")
	if err := moveFile(invalidSource, "invalid\x00destination"); err == nil {
		t.Fatal("moveFile(invalid destination) returned nil")
	}
	assertExists(t, invalidSource)
	conflictingSource := filepath.Join(baseFolder, "conflicting-source")
	writeFixture(t, conflictingSource, "new source")
	if err := moveFile(conflictingSource, to); err == nil {
		t.Fatal("moveFile(existing destination) returned nil")
	}
	if content, err := os.ReadFile(to); err != nil || string(content) != "move" {
		t.Fatalf("existing destination was changed: content=%q err=%v", content, err)
	}
	assertExists(t, conflictingSource)
	if err := deleteEmptyFolders(filepath.Join(baseFolder, "not-there")); err == nil {
		t.Fatal("deleteEmptyFolders(missing) returned nil")
	}
}

func extendedInfoForPath(t *testing.T, filePath string) db.ExtendedFileInfo {
	t.Helper()
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat fixture %q: %v", filePath, err)
	}
	return db.ExtendedFileInfo{BaseFolder: filepath.Dir(filePath), FileName: filepath.Base(filePath), Size: info.Size(), ModTime: info.ModTime()}
}

func setProcessSettings(t *testing.T, baseFolder string, options settings.OrganizeOptions) {
	t.Helper()
	settings.SaveSettings(&settings.AppSettings{Organization: options}, baseFolder)
}

func contentMetadata(id string, version int, displayVersion string) *switchfs.ContentMetaAttributes {
	return &switchfs.ContentMetaAttributes{TitleId: id, Version: version, Ncap: &switchfs.Nacp{
		DisplayVersion: displayVersion,
		TitleName:      map[string]switchfs.NacpTitle{},
	}}
}

func metadataForID(id string, version int) *switchfs.ContentMetaAttributes {
	return &switchfs.ContentMetaAttributes{TitleId: id, Version: version}
}

func writeFixture(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("write fixture %q: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func assertMoved(t *testing.T, source, destination, label string) {
	t.Helper()
	assertNotExists(t, source)
	if _, err := os.Stat(destination); err != nil {
		entries, readErr := os.ReadDir(filepath.Dir(destination))
		t.Fatalf("%s destination %q is missing: %v (directory entries: %v, read error: %v)", label, destination, err, entries, readErr)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %q to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q not to exist, stat error = %v", path, err)
	}
}

func TestParseTitleNameFromFileName(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"Name [id].nsp", "Name"},
		{"Name.nsp", "Name"},
		{"[id].nsp", ""},
	} {
		if got := db.ParseTitleNameFromFileName(test.input); got != test.want {
			t.Errorf("ParseTitleNameFromFileName(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestTemplateOutputHasNoWhitespaceRuns(t *testing.T) {
	data := map[string]string{settings.TEMPLATE_TITLE_NAME: "  A\t B  "}
	if got := applyTemplate(data, false, "{TITLE_NAME}", 0); strings.ContainsAny(got, "\t\n") || got != "A B" {
		t.Fatalf("normalized template = %q", got)
	}
}
