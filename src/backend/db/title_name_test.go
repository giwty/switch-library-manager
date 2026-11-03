package db

import (
	"testing"

	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func TestResolveGameTitleName(t *testing.T) {
	japanese := &SwitchTitle{Attributes: TitleAttributes{Id: "010044901C5C2000", Name: "カプコン ファイティング コレクション2"}}
	tests := []struct {
		name   string
		remote *SwitchTitle
		local  *SwitchGameFiles
		want   string
	}{
		{
			name:   "prefer a non-CJK database title",
			remote: &SwitchTitle{Attributes: TitleAttributes{Name: "Catalog Game"}},
			local:  titleNameBase("Base English", "Base British", "base.nsp"),
			want:   "Catalog Game",
		},
		{
			name:   "base American English wins over British English and updates",
			remote: japanese,
			local: titleNameBase("  Base US  ", "Base UK", "test.nsp", map[int]SwitchFileInfo{
				99: titleNameFile("update.nsp", "Update US", "Update UK"),
			}),
			want: "Base US",
		},
		{
			name:   "base British English is used when American English is empty",
			remote: japanese,
			local:  titleNameBase(" \t ", "  Base UK  ", "test.nsp"),
			want:   "Base UK",
		},
		{
			name:   "updates are checked from highest version downward",
			remote: japanese,
			local: titleNameUpdates(map[int]SwitchFileInfo{
				7: titleNameFile("older.nsp", "Older US", "Older UK"),
				8: titleNameFile("middle.nsp", "Middle US", "Middle UK"),
				9: titleNameFile("newer.nsp", "", " Newer UK "),
			}),
			want: "Newer UK",
		},
		{
			name:   "base English wins over a higher update",
			remote: japanese,
			local: titleNameBase("Base English", "", "base.nsp", map[int]SwitchFileInfo{
				99: titleNameFile("update.nsp", "Update English", ""),
			}),
			want: "Base English",
		},
		{
			name:   "filename wins over a Japanese database title",
			remote: japanese,
			local:  titleNameBase("", "", "test.nsp"),
			want:   "test",
		},
		{
			name:   "update filename wins over a Japanese database title",
			remote: japanese,
			local:  titleNameUpdates(map[int]SwitchFileInfo{1: titleNameFile("01.nsp", "", "")}),
			want:   "01",
		},
		{
			name:   "Japanese database title is used when no local name exists",
			remote: japanese,
			want:   japanese.Attributes.Name,
		},
		{
			name:   "Japanese database title is used when local filenames are empty",
			remote: japanese,
			local:  titleNameBase("", "", " "),
			want:   japanese.Attributes.Name,
		},
		{
			name:  "filename fallback uses highest update version first",
			local: titleNameUpdates(map[int]SwitchFileInfo{2: titleNameFile("01.nsp", "", ""), 9: titleNameFile("test.nsp", "", "")}),
			want:  "test",
		},
		{
			name: "filename fallback uses DLC title ID order",
			local: &SwitchGameFiles{Dlc: map[string]SwitchFileInfo{
				"b": titleNameFile("second.nsp", "", ""),
				"a": titleNameFile("first.nsp", "", ""),
			}},
			want: "first",
		},
		{
			name:   "trimmed non-CJK database title wins",
			remote: &SwitchTitle{Attributes: TitleAttributes{Name: " Catalog Game \t"}},
			want:   "Catalog Game",
		},
		{
			name:   "empty local names are skipped",
			remote: &SwitchTitle{Attributes: TitleAttributes{Name: " \t "}},
			local:  titleNameBase(" \n ", " \t ", " 01.nsp "),
			want:   "01",
		},
		{
			name: "nil metadata returns unknown title",
			want: "Unknown Title",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ResolveGameTitleName(test.remote, test.local, false); got != test.want {
				t.Fatalf("ResolveGameTitleName() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveGameTitleNameAllowsUnicodeCatalogTitles(t *testing.T) {
	for _, name := range []string{"日本語", "한국어", "中文", "åäö 🎮"} {
		t.Run(name, func(t *testing.T) {
			remote := &SwitchTitle{Attributes: TitleAttributes{Name: "  " + name + "  "}}
			local := titleNameBase("Embedded English", "British English", "Filename.nsp")
			if got := ResolveGameTitleName(remote, local, true); got != name {
				t.Fatalf("Unicode catalog title = %q, want %q", got, name)
			}
			if remote.Attributes.Name != "  "+name+"  " {
				t.Fatalf("resolver mutated catalog title: %q", remote.Attributes.Name)
			}
			local.File.Metadata = nil
			if got := ResolveGameTitleName(remote, local, true); got != name {
				t.Fatalf("Unicode catalog title without metadata = %q", got)
			}
		})
	}
}

func TestResolveGameTitleNameUnicodeFallbacks(t *testing.T) {
	for _, remote := range []*SwitchTitle{nil, {Attributes: TitleAttributes{Name: " \t "}}} {
		local := titleNameBase("Embedded English", "", "日本語 [0100000000010000].nsp")
		if got := ResolveGameTitleName(remote, local, true); got != "Embedded English" {
			t.Fatalf("embedded fallback = %q", got)
		}
		local.File.Metadata = nil
		if got := ResolveGameTitleName(remote, local, true); got != "日本語" {
			t.Fatalf("Unicode filename fallback = %q", got)
		}
		if got := ResolveGameTitleName(remote, nil, true); got != "Unknown Title" {
			t.Fatalf("missing title fallback = %q", got)
		}
	}
}

func titleNameBase(american, british, fileName string, updates ...map[int]SwitchFileInfo) *SwitchGameFiles {
	game := &SwitchGameFiles{BaseExist: true, File: titleNameFile(fileName, american, british)}
	if len(updates) > 0 {
		game.Updates = updates[0]
	}
	return game
}

func titleNameUpdates(updates map[int]SwitchFileInfo) *SwitchGameFiles {
	return &SwitchGameFiles{Updates: updates}
}

func titleNameFile(fileName, american, british string) SwitchFileInfo {
	titles := map[string]switchfs.NacpTitle{
		"AmericanEnglish": {Title: american},
		"BritishEnglish":  {Title: british},
	}
	return SwitchFileInfo{
		ExtendedInfo: ExtendedFileInfo{FileName: fileName},
		Metadata: &switchfs.ContentMetaAttributes{
			Ncap: &switchfs.Nacp{TitleName: titles},
		},
	}
}
