package process

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func TestScanForMissingUpdates(t *testing.T) {
	baseID := "0100abcd00001000"
	updateID := "0100abcd00001800"
	dlcID := "0100abcd00001a01"
	local := map[string]*db.SwitchGameFiles{
		"0100abcd00001": {
			BaseExist: true,
			File:      db.SwitchFileInfo{Metadata: metadataForID(baseID, 0)},
			Updates: map[int]db.SwitchFileInfo{
				2: {Metadata: metadataForID(updateID, 2)},
			},
			Dlc: map[string]db.SwitchFileInfo{
				dlcID: {Metadata: metadataWithDisplayVersion(dlcID, 1, "2.1.0")},
			},
		},
		"missing-base": {
			File:      db.SwitchFileInfo{Metadata: metadataForID("0100feed00001000", 0)},
			BaseExist: false,
		},
		"not-remote": {
			File:      db.SwitchFileInfo{Metadata: metadataForID("0100feed00002000", 0)},
			BaseExist: true,
		},
	}
	remote := map[string]*db.SwitchTitle{
		"0100abcd00001": {
			Attributes: db.TitleAttributes{Id: baseID, Name: "Game"},
			Updates:    map[int]string{1: "old", 5: "latest"},
			Dlc: map[string]db.TitleAttributes{
				dlcID: {Id: dlcID, Name: "DLC", Version: json.Number("3"), ReleaseDate: 20240102},
			},
		},
	}

	result := ScanForMissingUpdates(local, remote, map[string]struct{}{}, false)
	game, ok := result[baseID]
	if !ok {
		t.Fatal("expected missing base update and DLC update")
	}
	if game.LocalUpdate != 2 || game.LatestUpdate != 5 || game.LatestUpdateDate != "latest" {
		t.Fatalf("update result = %#v", game)
	}
	dlc, ok := result[dlcID]
	if !ok || dlc.LocalUpdate != 1 || dlc.LatestUpdate != 3 || dlc.LatestUpdateDate != "2024-01-02" || dlc.LocalDisplayVersion != "2.1.0" {
		t.Fatalf("DLC result = %#v, present = %v", dlc, ok)
	}

	ignored := ScanForMissingUpdates(local, remote, map[string]struct{}{baseID: {}}, false)
	if len(ignored) != 0 {
		t.Fatalf("ignored base result = %#v", ignored)
	}
	ignoredDLC := ScanForMissingUpdates(local, remote, map[string]struct{}{dlcID: {}}, false)
	if _, ok := ignoredDLC[dlcID]; ok {
		t.Fatal("ignored DLC was reported")
	}
	withoutDLC := ScanForMissingUpdates(local, remote, map[string]struct{}{}, true)
	if _, ok := withoutDLC[dlcID]; ok {
		t.Fatal("DLC update was reported when ignored")
	}

	noMissing := map[string]*db.SwitchGameFiles{
		"0100abcd00001": {BaseExist: true, File: db.SwitchFileInfo{Metadata: metadataForID(baseID, 0)}, Updates: map[int]db.SwitchFileInfo{5: {}}},
	}
	if got := ScanForMissingUpdates(noMissing, remote, nil, true); len(got) != 0 {
		t.Fatalf("complete result = %#v", got)
	}
}

func TestScanForMissingUpdatesNormalizesIgnoredBaseIDs(t *testing.T) {
	baseID := "0100abcd00001000"
	otherBaseID := "0100feed00001000"
	tests := []struct {
		name     string
		remoteID string
	}{
		{name: "uppercase", remoteID: strings.ToUpper(baseID)},
		{name: "mixed case", remoteID: "0100AbCd00001000"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			local := map[string]*db.SwitchGameFiles{
				"0100abcd00001": {BaseExist: true, File: db.SwitchFileInfo{Metadata: metadataForID(baseID, 0)}},
				"0100feed00001": {BaseExist: true, File: db.SwitchFileInfo{Metadata: metadataForID(otherBaseID, 0)}},
			}
			remote := map[string]*db.SwitchTitle{
				"0100abcd00001": {Attributes: db.TitleAttributes{Id: test.remoteID}, Updates: map[int]string{1: "latest"}},
				"0100feed00001": {Attributes: db.TitleAttributes{Id: otherBaseID}, Updates: map[int]string{1: "latest"}},
			}

			got := ScanForMissingUpdates(local, remote, map[string]struct{}{baseID: {}}, true)
			if len(got) != 1 {
				t.Fatalf("missing updates = %#v, want only the unrelated title", got)
			}
			if _, ok := got[otherBaseID]; !ok {
				t.Fatalf("unrelated title %q was not reported: %#v", otherBaseID, got)
			}
		})
	}
}

func TestScanForMissingUpdatesNormalizesIgnoredDLCIDs(t *testing.T) {
	baseID := "0100abcd00001000"
	ignoredDLCID := "0100abcd00001a01"
	otherDLCID := "0100abcd00001a02"
	tests := []struct {
		name       string
		metadataID string
	}{
		{name: "uppercase", metadataID: strings.ToUpper(ignoredDLCID)},
		{name: "mixed case", metadataID: "0100AbCd00001A01"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			local := map[string]*db.SwitchGameFiles{"0100abcd00001": {
				BaseExist: true,
				File:      db.SwitchFileInfo{Metadata: metadataForID(baseID, 0)},
				Dlc: map[string]db.SwitchFileInfo{
					ignoredDLCID: {Metadata: metadataForID(test.metadataID, 1)},
					otherDLCID:   {Metadata: metadataForID(otherDLCID, 1)},
				},
			}}
			remote := map[string]*db.SwitchTitle{"0100abcd00001": {
				Attributes: db.TitleAttributes{Id: baseID},
				Dlc: map[string]db.TitleAttributes{
					ignoredDLCID: {Id: ignoredDLCID, Version: json.Number("3")},
					otherDLCID:   {Id: otherDLCID, Version: json.Number("3")},
				},
			}}

			got := ScanForMissingUpdates(local, remote, map[string]struct{}{ignoredDLCID: {}}, false)
			if len(got) != 1 {
				t.Fatalf("missing updates = %#v, want only the unrelated DLC", got)
			}
			if _, ok := got[otherDLCID]; !ok {
				t.Fatalf("unrelated DLC %q was not reported: %#v", otherDLCID, got)
			}
		})
	}
}

func TestScanForMissingUpdatesDLCBranches(t *testing.T) {
	baseID := "0100abcd00001000"
	dlcGoodID := "0100abcd00001a01"
	dlcBadNumberID := "0100abcd00001a02"
	dlcNoLocalID := "0100abcd00001a03"
	local := map[string]*db.SwitchGameFiles{"0100abcd00001": {
		BaseExist: true,
		File:      db.SwitchFileInfo{Metadata: metadataForID(baseID, 0)},
		Dlc: map[string]db.SwitchFileInfo{
			dlcGoodID:      {Metadata: metadataForID(dlcGoodID, 1)},
			dlcBadNumberID: {Metadata: metadataForID(dlcBadNumberID, 1)},
			dlcNoLocalID:   {},
		},
	}}
	remote := map[string]*db.SwitchTitle{"0100abcd00001": {
		Attributes: db.TitleAttributes{Id: baseID},
		Dlc: map[string]db.TitleAttributes{
			dlcGoodID:      {Id: dlcGoodID, Version: json.Number("2"), ReleaseDate: 0},
			dlcBadNumberID: {Id: dlcBadNumberID, Version: json.Number("not-a-number")},
			dlcNoLocalID:   {Id: dlcNoLocalID, Version: json.Number("2")},
		},
	}}
	result := ScanForMissingUpdates(local, remote, map[string]struct{}{}, false)
	if got, ok := result[dlcGoodID]; !ok || got.LatestUpdateDate != "-" {
		t.Fatalf("DLC with missing release date = %#v, present = %v", got, ok)
	}
	if _, ok := result[dlcBadNumberID]; ok {
		t.Fatal("DLC with invalid remote version was reported")
	}
	if _, ok := result[dlcNoLocalID]; ok {
		t.Fatal("DLC with no local record was reported")
	}

	local["0100abcd00001"].Dlc[dlcGoodID] = db.SwitchFileInfo{}
	if _, ok := ScanForMissingUpdates(local, remote, nil, false)[dlcGoodID]; ok {
		t.Fatal("metadata-free local DLC was reported")
	}
}

func TestScanForMissingUpdatesLocalDisplayVersion(t *testing.T) {
	baseID := "0100abcd00001000"
	updateID := "0100abcd00001800"
	tests := []struct {
		name               string
		baseMetadata       *switchfs.ContentMetaAttributes
		updates            map[int]db.SwitchFileInfo
		wantUpdate         int
		wantDisplayVersion string
	}{
		{
			name:         "highest local update",
			baseMetadata: metadataWithDisplayVersion(baseID, 0, "1.0.0"),
			updates: map[int]db.SwitchFileInfo{
				2: {Metadata: metadataWithDisplayVersion(updateID, 2, "1.2.0")},
				4: {Metadata: metadataWithDisplayVersion(updateID, 4, "1.4.0")},
			},
			wantUpdate:         4,
			wantDisplayVersion: "1.4.0",
		},
		{
			name:               "base game when there is no local update",
			baseMetadata:       metadataWithDisplayVersion(baseID, 0, "1.0.0"),
			wantDisplayVersion: "1.0.0",
		},
		{
			name:         "metadata-free filename fallback update",
			baseMetadata: metadataWithDisplayVersion(baseID, 0, "1.0.0"),
			updates: map[int]db.SwitchFileInfo{
				2: {Metadata: metadataForID(updateID, 2)},
			},
			wantUpdate: 2,
		},
		{
			name: "missing metadata",
			updates: map[int]db.SwitchFileInfo{
				2: {},
			},
			wantUpdate: 2,
		},
		{
			name: "missing NACP",
			updates: map[int]db.SwitchFileInfo{
				2: {Metadata: metadataForID(updateID, 2)},
			},
			wantUpdate: 2,
		},
		{
			name: "empty display version",
			updates: map[int]db.SwitchFileInfo{
				2: {Metadata: metadataWithDisplayVersion(updateID, 2, "")},
			},
			wantUpdate: 2,
		},
	}

	remote := map[string]*db.SwitchTitle{
		"0100abcd00001": {
			Attributes: db.TitleAttributes{Id: baseID},
			Updates:    map[int]string{9: "latest"},
			Dlc:        map[string]db.TitleAttributes{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			local := map[string]*db.SwitchGameFiles{
				"0100abcd00001": {
					BaseExist: true,
					File:      db.SwitchFileInfo{Metadata: test.baseMetadata},
					Updates:   test.updates,
				},
			}
			got := ScanForMissingUpdates(local, remote, nil, true)[baseID]
			if got.LocalUpdate != test.wantUpdate || got.LocalDisplayVersion != test.wantDisplayVersion {
				t.Fatalf("local update/display version = %d/%q, want %d/%q", got.LocalUpdate, got.LocalDisplayVersion, test.wantUpdate, test.wantDisplayVersion)
			}
		})
	}
}

func metadataWithDisplayVersion(id string, version int, displayVersion string) *switchfs.ContentMetaAttributes {
	return &switchfs.ContentMetaAttributes{
		TitleId: id,
		Version: version,
		Ncap:    &switchfs.Nacp{DisplayVersion: displayVersion},
	}
}

func TestScanForMissingDLC(t *testing.T) {
	baseID := "0100abcd00001000"
	missingID := "0100abcd00001a01"
	ignoredID := "0100abcd00001a02"
	presentID := "0100abcd00001a03"
	secondMissingID := "0100abcd00001a04"
	remote := map[string]*db.SwitchTitle{
		"0100abcd00001": {Attributes: db.TitleAttributes{Id: baseID}, Dlc: map[string]db.TitleAttributes{
			missingID:       {Name: "Expansion", Id: missingID, ReleaseDate: 20240102, ParsedReleaseDate: "2024-01-02"},
			ignoredID:       {Name: "Ignored", Id: ignoredID},
			presentID:       {Name: "Already installed", Id: presentID},
			secondMissingID: {Name: "Extra Story", Id: secondMissingID, ParsedReleaseDate: "0"},
		}},
	}
	local := map[string]*db.SwitchGameFiles{
		"0100abcd00001": {BaseExist: true, Dlc: map[string]db.SwitchFileInfo{
			ignoredID: {},
			presentID: {},
		}},
		"missing-base": {BaseExist: false},
		"not-remote":   {BaseExist: true},
	}
	result := ScanForMissingDLC(local, remote, map[string]struct{}{ignoredID: {}})
	game, ok := result[baseID]
	if !ok || len(game.MissingDLC) != 2 {
		t.Fatalf("missing DLC result = %#v, present = %v", game, ok)
	}
	gotDLC := make(map[string]MissingDLC, len(game.MissingDLC))
	for _, dlc := range game.MissingDLC {
		gotDLC[dlc.Id] = dlc
	}
	if gotDLC[missingID] != (MissingDLC{Id: missingID, Name: "Expansion", ReleaseDate: "2024-01-02"}) || gotDLC[secondMissingID] != (MissingDLC{Id: secondMissingID, Name: "Extra Story"}) {
		t.Fatalf("structured missing DLC values = %#v", gotDLC)
	}
	encodedDLC, err := json.Marshal(gotDLC[missingID])
	if err != nil {
		t.Fatalf("marshal missing DLC: %v", err)
	}
	if string(encodedDLC) != `{"id":"`+missingID+`","name":"Expansion","release_date":"2024-01-02"}` {
		t.Fatalf("missing DLC JSON = %s", encodedDLC)
	}
	encodedUnknown, err := json.Marshal(gotDLC[secondMissingID])
	if err != nil {
		t.Fatalf("marshal missing DLC without a release date: %v", err)
	}
	if string(encodedUnknown) != `{"id":"`+secondMissingID+`","name":"Extra Story","release_date":""}` {
		t.Fatalf("missing DLC JSON without a release date = %s", encodedUnknown)
	}
	if _, exists := gotDLC[presentID]; exists {
		t.Fatalf("locally present DLC was reported: %#v", gotDLC)
	}
	if got := ScanForMissingDLC(local, remote, map[string]struct{}{missingID: {}, ignoredID: {}, secondMissingID: {}}); len(got) != 0 {
		t.Fatalf("fully ignored result = %#v", got)
	}
	if got := ScanForMissingDLC(map[string]*db.SwitchGameFiles{}, remote, nil); len(got) != 0 {
		t.Fatalf("empty local result = %#v", got)
	}
}

func TestScanForBrokenFiles(t *testing.T) {
	base := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "base.nsp"}}
	dlc := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "dlc.nsp"}}
	update := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "update.nsp"}}
	result := ScanForBrokenFiles(map[string]*db.SwitchGameFiles{
		"has-base":     {BaseExist: true, File: base, Dlc: map[string]db.SwitchFileInfo{"base": dlc}, Updates: map[int]db.SwitchFileInfo{1: update}},
		"missing-base": {BaseExist: false, Dlc: map[string]db.SwitchFileInfo{"dlc": dlc}, Updates: map[int]db.SwitchFileInfo{1: update}},
	})
	if len(result) != 2 {
		t.Fatalf("broken files = %#v, want 2", result)
	}
	for _, file := range result {
		if file.ExtendedInfo.FileName != "dlc.nsp" && file.ExtendedInfo.FileName != "update.nsp" {
			t.Fatalf("unexpected broken file = %#v", file)
		}
	}
	if got := ScanForBrokenFiles(map[string]*db.SwitchGameFiles{}); len(got) != 0 {
		t.Fatalf("empty broken files = %#v", got)
	}
}
