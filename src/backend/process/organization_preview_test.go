package process

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func TestBuildOrganizationPreviewUsesActualBaseUpdateAndDLC(t *testing.T) {
	root := t.TempDir()
	options := settings.OrganizeOptions{
		CreateFolderPerGame:   true,
		FolderNameTemplate:    "{TITLE_NAME}",
		RenameFiles:           true,
		FileNameTemplate:      "{TITLE_ID}_{TYPE}_{VERSION}_{SIZE_GB}_{SIZE_MB}",
		AllowUnicodeFileNames: true,
		IncludeBaseType:       true,
		UpdatesFolder:         "updates",
		DlcFolder:             "dlc",
	}
	baseID := "0100E95004039000"
	updateID := "0100E95004039800"
	dlcID := "0100E9500403A001"
	gameKey := "0100e95004039"
	source := filepath.Join(root, "incoming")
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		gameKey: {
			BaseExist: true,
			File: db.SwitchFileInfo{
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "base.nsp", Size: 600_000_000},
				Metadata:     previewTestMetadata(baseID, 0, "1.0.0"),
			},
			Updates: map[int]db.SwitchFileInfo{5: {
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "update.nsp", Size: 2_700_000_000},
				Metadata:     previewTestMetadata(updateID, 5, "5.0.0"),
			}},
			Dlc: map[string]db.SwitchFileInfo{dlcID: {
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "dlc.nsp", Size: 1_200_000_000},
				Metadata:     previewTestMetadata(dlcID, 1, "1.0.0"),
			}},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		gameKey: {
			Attributes: db.TitleAttributes{Id: baseID, Name: "Preview Game", Region: "US"},
			Dlc:        map[string]db.TitleAttributes{dlcID: {Id: dlcID, Name: "Preview Game - Expansion"}},
		},
	}}

	got, err := BuildOrganizationPreview(root, options, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("preview entries = %#v, want three entries", got)
	}
	want := map[string]string{
		"game":   filepath.Join("Preview Game", baseID+"_BASE_0_0.6GB_600MB.nsp"),
		"update": filepath.Join("Preview Game", "updates", updateID+"_UPD_5_2.7GB_2700MB.nsp"),
		"dlc":    filepath.Join("Preview Game", "dlc", dlcID+"_DLC_1_1.2GB_1200MB.nsp"),
	}
	for _, entry := range got {
		if entry.Path != want[entry.Kind] {
			t.Errorf("%s preview path = %q, want %q", entry.Kind, entry.Path, want[entry.Kind])
		}
	}
}

func TestBuildOrganizationPreviewUsesV0DLCDisplayVersionFallback(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "incoming")
	dlcID := "0100E9500403A001"
	options := settings.OrganizeOptions{
		RenameFiles:                true,
		FileNameTemplate:           "{TITLE_NAME} [v{VERSION}][v{VERSION_TXT}]",
		ProcessWhenMissingBaseGame: true,
	}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100e95004039": {
			Dlc: map[string]db.SwitchFileInfo{
				dlcID: {
					ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "dlc.nsp"},
					Metadata:     previewTestMetadata(dlcID, 0, ""),
				},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100e95004039": {Attributes: db.TitleAttributes{Name: "Preview Game"}},
	}}

	got, err := BuildOrganizationPreview(root, options, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("incoming", "Preview Game [v0][v1.0.0].nsp")
	for _, entry := range got {
		if entry.Kind == "dlc" {
			if entry.Path != want {
				t.Fatalf("DLC preview = %q, want %q", entry.Path, want)
			}
			return
		}
	}
	t.Fatalf("preview = %#v, missing DLC at %q", got, want)
}

func TestBuildOrganizationPreviewUsesBundledVersionForBaseFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "incoming")
	baseID := "010087E01FCD6000"
	updateID := "010087E01FCD6800"
	baseFile := db.ExtendedFileInfo{BaseFolder: source, FileName: "package.xci", Size: 20}
	standaloneUpdate := db.ExtendedFileInfo{BaseFolder: source, FileName: "update.nsp", Size: 18}
	options := settings.OrganizeOptions{
		RenameFiles:           true,
		FileNameTemplate:      "{TITLE_NAME} [{TITLE_ID}][{TYPE}][v{VERSION}][{VERSION_TXT}]",
		IncludeBaseType:       true,
		AllowUnicodeFileNames: true,
	}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"010087e01fcd6": {
			BaseExist: true,
			File: db.SwitchFileInfo{
				ExtendedInfo: baseFile,
				Metadata:     previewTestMetadata(baseID, 0, "1.0.100"),
			},
			Updates: map[int]db.SwitchFileInfo{
				65536: {
					ExtendedInfo: baseFile,
					Metadata:     previewTestMetadata(updateID, 65536, "1.0.185"),
				},
				196608: {
					ExtendedInfo: standaloneUpdate,
					Metadata:     previewTestMetadata(updateID, 196608, "2.0.27"),
				},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"010087e01fcd6": {Attributes: db.TitleAttributes{Id: baseID, Name: "Cuisineer"}},
	}}

	got, err := BuildOrganizationPreview(root, options, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, entry := range got {
		paths[entry.Kind] = entry.Path
	}
	wantBase := filepath.Join("incoming", "Cuisineer [010087E01FCD6000][BASE][v65536][1.0.185].xci")
	wantUpdate := filepath.Join("incoming", "Cuisineer [010087E01FCD6800][UPD][v196608][2.0.27].nsp")
	if paths["game"] != wantBase {
		t.Fatalf("base preview path = %q, want %q", paths["game"], wantBase)
	}
	if paths["update"] != wantUpdate {
		t.Fatalf("update preview path = %q, want %q", paths["update"], wantUpdate)
	}
}

func TestBuildOrganizationPreviewExpandsPackageContentsForBundledBase(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "incoming")
	baseFile := db.ExtendedFileInfo{BaseFolder: source, FileName: "package (1G+1U+1D).xci"}
	standaloneUpdate := db.ExtendedFileInfo{BaseFolder: source, FileName: "standalone-update.nsp"}
	baseID := "0100000000010000"
	updateID := "0100000000010800"
	dlcID := "0100000000011001"
	options := settings.OrganizeOptions{
		RenameFiles:           true,
		FileNameTemplate:      "{TITLE_NAME} ({PACKAGE_CONTENTS})[{TYPE}][v{VERSION}]",
		IncludeBaseType:       true,
		AllowUnicodeFileNames: true,
	}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"game": {
			BaseExist: true,
			File: db.SwitchFileInfo{
				ExtendedInfo: baseFile,
				Metadata:     previewTestMetadata(baseID, 0, "1.0.0"),
			},
			Updates: map[int]db.SwitchFileInfo{
				1: {ExtendedInfo: baseFile, Metadata: previewTestMetadata(updateID, 1, "1.0.1")},
				2: {ExtendedInfo: baseFile, Metadata: previewTestMetadata(updateID, 2, "1.0.2")},
				3: {ExtendedInfo: standaloneUpdate, Metadata: previewTestMetadata(updateID, 3, "1.0.3")},
			},
			Dlc: map[string]db.SwitchFileInfo{
				dlcID: {ExtendedInfo: baseFile, Metadata: previewTestMetadata(dlcID, 1, "1.0.0")},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"game": {Attributes: db.TitleAttributes{Id: baseID, Name: "Preview Game"}},
	}}

	got, err := BuildOrganizationPreview(root, options, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, entry := range got {
		paths[entry.Kind] = entry.Path
	}
	if want := filepath.Join("incoming", "Preview Game (1G+2U+1D)[BASE][v2].xci"); paths["game"] != want {
		t.Fatalf("base preview path = %q, want %q", paths["game"], want)
	}
	if want := filepath.Join("incoming", "Preview Game [UPD][v3].nsp"); paths["update"] != want {
		t.Fatalf("standalone update preview path = %q, want %q", paths["update"], want)
	}
}

func TestBuildOrganizationPreviewFallbackShowsDeterministicSize(t *testing.T) {
	root := t.TempDir()
	options := settings.OrganizeOptions{
		RenameFiles:           true,
		FileNameTemplate:      "{TITLE_NAME}_{SIZE_GB}_{SIZE_MB}",
		AllowUnicodeFileNames: true,
		UpdatesFolder:         "updates",
		DlcFolder:             "dlc",
	}

	got, err := BuildOrganizationPreview(root, options, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"game":   "Example Adventure_0.6GB_600MB.nsp",
		"update": filepath.Join("updates", "Example Adventure_0.6GB_600MB.nsp"),
		"dlc":    filepath.Join("dlc", "Example Adventure_0.6GB_600MB.nsp"),
	}
	for _, entry := range got {
		if entry.Path != want[entry.Kind] {
			t.Errorf("%s fallback preview path = %q, want %q", entry.Kind, entry.Path, want[entry.Kind])
		}
	}
}

func TestBuildOrganizationPreviewFallsBackAndKeepsOriginalNamesWhenDisabled(t *testing.T) {
	root := t.TempDir()
	options := settings.OrganizeOptions{UpdatesFolder: "updates", DlcFolder: "dlc"}

	got, err := BuildOrganizationPreview(root, options, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("fallback preview entries = %#v, want three entries", got)
	}
	for _, entry := range got {
		if strings.Contains(entry.Path, "Example Adventure") {
			t.Errorf("disabled rename unexpectedly used title name in %s path: %q", entry.Kind, entry.Path)
		}
	}
	if got[0].Path != "example-adventure.nsp" {
		t.Errorf("fallback game path = %q, want original filename", got[0].Path)
	}
	if got[1].Path != filepath.Join("updates", "example-adventure-update.nsp") {
		t.Errorf("fallback update path = %q", got[1].Path)
	}
	if got[2].Path != filepath.Join("dlc", "example-adventure-dlc.nsp") {
		t.Errorf("fallback DLC path = %q", got[2].Path)
	}
}

func TestBuildOrganizationPreviewAppliesSafeNames(t *testing.T) {
	root := t.TempDir()
	options := settings.OrganizeOptions{
		CreateFolderPerGame:   true,
		FolderNameTemplate:    "{TITLE_NAME}",
		RenameFiles:           true,
		FileNameTemplate:      "{TITLE_NAME}",
		AllowUnicodeFileNames: false,
	}
	key := "0100e95004039"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{key: {
		BaseExist: true,
		File: db.SwitchFileInfo{
			ExtendedInfo: db.ExtendedFileInfo{BaseFolder: root, FileName: "source.nsp"},
			Metadata:     previewTestMetadata("0100E95004039000", 0, "1.0.0"),
		},
	}}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{key: {
		Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Pokémon: Demo"},
	}}}

	got, err := BuildOrganizationPreview(root, options, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("safe-name preview entries = %#v, want fallback-filled entries", got)
	}
	if strings.ContainsAny(got[0].Path, ":?*<>|\"") {
		t.Errorf("safe-name preview contains an unsafe character: %q", got[0].Path)
	}
}

func previewTestMetadata(id string, version int, displayVersion string) *switchfs.ContentMetaAttributes {
	return &switchfs.ContentMetaAttributes{TitleId: id, Version: version, Ncap: &switchfs.Nacp{DisplayVersion: displayVersion}}
}
