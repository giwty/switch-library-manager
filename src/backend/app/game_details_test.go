package app

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

const (
	detailsBaseID       = "0100000000010000"
	detailsUpdateID     = "0100000000010800"
	detailsDLCID        = "0100000000011001"
	detailsLocalOnlyDLC = "0100000000011002"
	detailsPrefix       = "0100000000010"
)

func TestBuildGameDetailsResolvesContentIDsAndComparesLocalCollection(t *testing.T) {
	base := detailsFile("Game.nsp", &switchfs.ContentMetaAttributes{TitleId: detailsBaseID})
	localUpdate := detailsFile("Game update v3.nsp", &switchfs.ContentMetaAttributes{TitleId: detailsUpdateID, Version: 3})
	localDLC := detailsFile("Game DLC.nsp", &switchfs.ContentMetaAttributes{TitleId: detailsDLCID, Version: 1})
	localOnlyDLC := detailsFile("Game extra DLC.nsp", &switchfs.ContentMetaAttributes{TitleId: detailsLocalOnlyDLC, Version: 4})

	localDB := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		detailsPrefix: {
			BaseExist: true,
			File:      base,
			Updates: map[int]db.SwitchFileInfo{
				3: localUpdate,
			},
			Dlc: map[string]db.SwitchFileInfo{
				detailsDLCID:        localDLC,
				detailsLocalOnlyDLC: localOnlyDLC,
			},
		},
	}}
	switchDB := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		detailsPrefix: {
			Attributes: db.TitleAttributes{Id: detailsBaseID, Name: "Game", Region: "USA"},
			Updates:    map[int]string{2: "2024-02-01", 3: "2024-03-01", 5: "2024-05-01", 100: "2024-10-01"},
			Dlc: map[string]db.TitleAttributes{
				detailsDLCID:       {Id: detailsDLCID, Name: "Expansion", Version: json.Number("2"), ReleaseDate: 20240102, ParsedReleaseDate: "2024-01-02"},
				"0100000000011003": {Id: "0100000000011003", Name: "Bonus Pack", Version: json.Number("1")},
			},
		},
	}}

	for _, inputID := range []string{detailsBaseID, detailsUpdateID, detailsDLCID} {
		details, err := buildGameDetails(inputID, localDB, switchDB, settings.MissingContentSettings{}, false)
		if err != nil {
			t.Fatalf("build details for %s: %v", inputID, err)
		}
		if details.TitleID != detailsBaseID || details.Name != "Game" || !details.BaseCollected {
			t.Fatalf("unexpected game details for %s: %#v", inputID, details)
		}
		if details.BasePath != filepath.Join(base.ExtendedInfo.BaseFolder, base.ExtendedInfo.FileName) {
			t.Errorf("base path = %q", details.BasePath)
		}
		if len(details.Updates) != 4 || details.Updates[0].Version != 100 || details.Updates[0].Status != "missing" || details.Updates[1].Version != 5 || details.Updates[1].Status != "missing" || details.Updates[2].Version != 3 || details.Updates[2].Status != "collected" || details.Updates[3].Version != 2 || details.Updates[3].Status != "collected" {
			t.Errorf("remote update comparison/order = %#v", details.Updates)
		}
		if details.Updates[2].Path != filepath.Join(localUpdate.ExtendedInfo.BaseFolder, localUpdate.ExtendedInfo.FileName) {
			t.Errorf("collected update path = %q", details.Updates[2].Path)
		}
		if len(details.LocalOnlyUpdates) != 0 {
			t.Errorf("local-only updates = %#v", details.LocalOnlyUpdates)
		}
		if len(details.DLC) != 2 || details.DLC[0].Name != "Expansion" || details.DLC[0].Status != "collected" || details.DLC[0].UpdateStatus != "out_of_date" || details.DLC[1].Name != "Bonus Pack" || details.DLC[1].Status != "missing" {
			t.Errorf("DLC comparison/order = %#v", details.DLC)
		}
		if len(details.DLC) == 2 && (details.DLC[0].Attributes.ParsedReleaseDate != "2024-01-02" || details.DLC[1].Attributes.ReleaseDate != 0) {
			t.Errorf("DLC release dates = %#v", details.DLC)
		}
		if len(details.LocalOnlyDLC) != 1 || details.LocalOnlyDLC[0].TitleID != detailsLocalOnlyDLC || details.LocalOnlyDLC[0].Status != "collected" {
			t.Errorf("local-only DLC = %#v", details.LocalOnlyDLC)
		}
	}
}

func TestBuildGameDetailsSortsDLCByNewestReleaseDate(t *testing.T) {
	dlc := []db.TitleAttributes{
		{Id: "0100000000011001", Name: "Character Pack", ReleaseDate: 20200720},
		{Id: "0100000000011002", Name: "Melody Pack", ReleaseDate: 20200826},
		{Id: "0100000000011003", Name: "Symphony of the Mask", ReleaseDate: 20200923},
		{Id: "0100000000011004", Name: "Season Pass Bonus Items", ReleaseDate: 20200720},
		{Id: "0100000000011005", Name: "Unknown Z"},
		{Id: "0100000000011006", Name: "Unknown A"},
		{Id: "0100000000011007", Name: "character pack", ReleaseDate: 20200720},
	}
	remote := &db.SwitchTitle{Attributes: db.TitleAttributes{Id: detailsBaseID}, Dlc: map[string]db.TitleAttributes{}}
	for _, attributes := range dlc {
		remote.Dlc[attributes.Id] = attributes
	}
	details, err := buildGameDetails(detailsBaseID,
		&db.LocalSwitchFilesDB{},
		&db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{detailsPrefix: remote}},
		settings.MissingContentSettings{}, false)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{
		"0100000000011003", "0100000000011002", "0100000000011001", "0100000000011007",
		"0100000000011004", "0100000000011006", "0100000000011005",
	}
	if len(details.DLC) != len(wantIDs) {
		t.Fatalf("DLC count = %d, want %d", len(details.DLC), len(wantIDs))
	}
	for i, wantID := range wantIDs {
		if details.DLC[i].TitleID != wantID {
			t.Fatalf("DLC[%d] = %q, want %q: newest dates first, then name and ID, unknown dates last", i, details.DLC[i].TitleID, wantID)
		}
	}
}

func TestBuildGameDetailsUsesLocalOnlyNewerUpdateForOlderCatalogStatus(t *testing.T) {
	localUpdate := detailsFile("Game update v99.nsp", &switchfs.ContentMetaAttributes{TitleId: detailsUpdateID, Version: 99})
	localDB := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		detailsPrefix: {
			Updates: map[int]db.SwitchFileInfo{99: localUpdate},
		},
	}}
	switchDB := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		detailsPrefix: {
			Updates: map[int]string{5: "2024-05-01", 100: "2024-10-01"},
		},
	}}

	details, err := buildGameDetails(detailsBaseID, localDB, switchDB, settings.MissingContentSettings{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Updates) != 2 || details.Updates[0].Version != 100 || details.Updates[0].Status != "missing" || details.Updates[1].Version != 5 || details.Updates[1].Status != "collected" {
		t.Fatalf("remote updates should use the highest local version: %#v", details.Updates)
	}
	if len(details.LocalOnlyUpdates) != 1 || details.LocalOnlyUpdates[0].Version != 99 || details.LocalOnlyUpdates[0].Status != "collected" {
		t.Fatalf("local-only newer update should remain visible: %#v", details.LocalOnlyUpdates)
	}
}

func TestBuildGameDetailsMarksIgnoredEntriesWithoutHidingThem(t *testing.T) {
	local := &db.SwitchGameFiles{
		Updates: map[int]db.SwitchFileInfo{
			6: detailsFile("Game update v6.nsp", &switchfs.ContentMetaAttributes{TitleId: detailsUpdateID, Version: 6}),
		},
		Dlc: map[string]db.SwitchFileInfo{
			detailsDLCID: detailsFile("Game DLC.nsp", &switchfs.ContentMetaAttributes{TitleId: detailsDLCID, Version: 1}),
		},
	}
	switchDB := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		detailsPrefix: {
			Attributes: db.TitleAttributes{Id: detailsBaseID, Name: "Game"},
			Updates:    map[int]string{5: "2024-05-01", 7: "2024-07-01"},
			Dlc: map[string]db.TitleAttributes{
				detailsDLCID:       {Id: detailsDLCID, Name: "Expansion", Version: json.Number("2")},
				"0100000000011003": {Id: "0100000000011003", Name: "Ignored DLC", Version: json.Number("1")},
			},
		},
	}}
	ignoreSettings := settings.MissingContentSettings{
		IgnoreDLCUpdates:  false,
		IgnoreDLCTitleIDs: []string{"0100000000011003"},
		IgnoreUpdateIDs:   []string{"0100000000010000"},
	}

	details, err := buildGameDetails(detailsBaseID, &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{detailsPrefix: local}}, switchDB, ignoreSettings, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Updates) != 2 || details.Updates[0].Version != 7 || details.Updates[0].Status != "ignored" || details.Updates[1].Version != 5 || details.Updates[1].Status != "collected" {
		t.Fatalf("ignored and newer-local update states should remain visible: %#v", details.Updates)
	}
	if len(details.LocalOnlyUpdates) != 1 || details.LocalOnlyUpdates[0].Version != 6 || details.LocalOnlyUpdates[0].Status != "collected" {
		t.Fatalf("locally collected update absent from catalog = %#v", details.LocalOnlyUpdates)
	}
	if len(details.DLC) != 2 {
		t.Fatalf("DLC rows = %#v", details.DLC)
	}
	dlcByID := make(map[string]GameDLCDetail, len(details.DLC))
	for _, item := range details.DLC {
		dlcByID[item.TitleID] = item
	}
	if dlcByID["0100000000011003"].Status != "ignored" || dlcByID[detailsDLCID].Status != "collected" || dlcByID[detailsDLCID].UpdateStatus != "ignored" {
		t.Fatalf("ignored DLC state should remain visible: %#v", details.DLC)
	}
}

func TestBuildGameDetailsUsesLocalFallbackAndHandlesMissingMetadata(t *testing.T) {
	file := detailsFile("Local Game [0100000000010000][v0].nsp", nil)
	localDB := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		detailsPrefix: {BaseExist: true, File: file, Updates: map[int]db.SwitchFileInfo{}, Dlc: map[string]db.SwitchFileInfo{}},
	}}

	details, err := buildGameDetails(detailsBaseID, localDB, &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{}}, settings.MissingContentSettings{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if details.RemoteAvailable || !details.BaseCollected || details.Name != db.ParseTitleNameFromFileName(file.ExtendedInfo.FileName) {
		t.Fatalf("unexpected local fallback details: %#v", details)
	}
	if details.Attributes.Id != detailsBaseID {
		t.Errorf("fallback title ID = %q", details.Attributes.Id)
	}
}

func TestBuildGameDetailsUsesSharedTitleResolver(t *testing.T) {
	local := &db.SwitchGameFiles{
		BaseExist: true,
		File: detailsFile("test.nsp", &switchfs.ContentMetaAttributes{TitleId: detailsBaseID, Ncap: &switchfs.Nacp{
			TitleName: map[string]switchfs.NacpTitle{"AmericanEnglish": {Title: "Embedded English Name"}},
		}}),
	}
	remoteTitle := &db.SwitchTitle{Attributes: db.TitleAttributes{Id: detailsBaseID, Name: "カタログ名"}}
	localDB := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{detailsPrefix: local}}
	switchDB := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{detailsPrefix: remoteTitle}}

	details, err := buildGameDetails(detailsBaseID, localDB, switchDB, settings.MissingContentSettings{}, false)
	if err != nil {
		t.Fatalf("build details: %v", err)
	}
	if details.Name != "Embedded English Name" || details.Attributes.Name != details.Name {
		t.Fatalf("resolved English name is inconsistent: Name=%q Attributes.Name=%q", details.Name, details.Attributes.Name)
	}
	if remoteTitle.Attributes.Name != "カタログ名" {
		t.Fatalf("resolver mutated catalog title: %q", remoteTitle.Attributes.Name)
	}

	local.File.Metadata.Ncap.TitleName = nil
	details, err = buildGameDetails(detailsBaseID, localDB, switchDB, settings.MissingContentSettings{}, false)
	if err != nil {
		t.Fatalf("build details without English metadata: %v", err)
	}
	if details.Name != "test" || details.Attributes.Name != details.Name {
		t.Fatalf("resolved filename fallback is inconsistent: Name=%q Attributes.Name=%q", details.Name, details.Attributes.Name)
	}
}

func TestBuildGameDetailsKeepsCatalogContentWhenGameMetadataIsMissing(t *testing.T) {
	file := detailsFile("Local Game [0100000000010000][v0].nsp", nil)
	localDB := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		detailsPrefix: {BaseExist: true, File: file, Updates: map[int]db.SwitchFileInfo{}, Dlc: map[string]db.SwitchFileInfo{}},
	}}
	switchDB := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		detailsPrefix: {Updates: map[int]string{1: "2024-01-01"}},
	}}

	details, err := buildGameDetails(detailsBaseID, localDB, switchDB, settings.MissingContentSettings{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !details.RemoteAvailable || details.MetadataAvailable || details.Name != db.ParseTitleNameFromFileName(file.ExtendedInfo.FileName) {
		t.Fatalf("sparse catalog fallback = %#v", details)
	}
	if len(details.Updates) != 1 || details.Updates[0].Status != "missing" {
		t.Fatalf("catalog updates should remain available: %#v", details.Updates)
	}
}

func TestBuildGameDetailsRejectsInvalidAndUnknownIDs(t *testing.T) {
	localDB := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{}}
	switchDB := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{}}

	if _, err := buildGameDetails("not-a-title-id", localDB, switchDB, settings.MissingContentSettings{}, false); err == nil {
		t.Fatal("expected invalid ID error")
	}
	if _, err := buildGameDetails("0100000000990000", localDB, switchDB, settings.MissingContentSettings{}, false); err == nil {
		t.Fatal("expected unknown title error")
	}
	if _, err := buildGameDetails(detailsBaseID, nil, switchDB, settings.MissingContentSettings{}, false); err == nil {
		t.Fatal("expected unloaded database error")
	}
}

func TestSafeTitleAttributesRejectsUnsafeImageURLsWithoutMutatingSource(t *testing.T) {
	attributes := db.TitleAttributes{
		IconUrl:     "javascript:alert(1)",
		BannerUrl:   "https://example.com/banner.png",
		Screenshots: []string{"file:///tmp/image.png", "https://example.com/screen.png"},
	}
	got := safeTitleAttributes(attributes)
	if got.IconUrl != "" || got.BannerUrl != attributes.BannerUrl || got.Screenshots[0] != "" || got.Screenshots[1] != "https://example.com/screen.png" {
		t.Fatalf("unexpected safe URLs: %#v", got)
	}
	if attributes.Screenshots[0] != "file:///tmp/image.png" {
		t.Fatal("sanitizing details mutated the source title database")
	}
}

func detailsFile(name string, metadata *switchfs.ContentMetaAttributes) db.SwitchFileInfo {
	return db.SwitchFileInfo{
		ExtendedInfo: db.ExtendedFileInfo{BaseFolder: filepath.Join("library", "games"), FileName: name},
		Metadata:     metadata,
	}
}
