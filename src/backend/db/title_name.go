package db

import (
	"sort"
	"strings"
	"unicode"
)

func containsCJK(name string) bool {
	for _, character := range name {
		if unicode.In(character, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}

// ResolveGameTitleName chooses the same human-readable title for organization
// and presentation. allowUnicode accepts catalog titles in every script ahead
// of local fallbacks. Filesystem-specific transliteration and sanitization are
// applied by the caller when building paths.
func ResolveGameTitleName(remote *SwitchTitle, local *SwitchGameFiles, allowUnicode bool) string {
	if remote != nil {
		name := strings.TrimSpace(remote.Attributes.Name)
		if name != "" && (allowUnicode || !containsCJK(name)) {
			return name
		}
	}

	if local != nil {
		if local.BaseExist {
			if name := embeddedEnglishTitle(local.File); name != "" {
				return name
			}
		}

		versions := sortedGameUpdateVersions(local)
		for i := len(versions) - 1; i >= 0; i-- {
			if name := embeddedEnglishTitle(local.Updates[versions[i]]); name != "" {
				return name
			}
		}
	}

	if local != nil {
		if local.BaseExist {
			if name := ParseTitleNameFromFileName(local.File.ExtendedInfo.FileName); name != "" {
				return name
			}
		}

		versions := sortedGameUpdateVersions(local)
		for i := len(versions) - 1; i >= 0; i-- {
			if name := ParseTitleNameFromFileName(local.Updates[versions[i]].ExtendedInfo.FileName); name != "" {
				return name
			}
		}

		ids := make([]string, 0, len(local.Dlc))
		for id := range local.Dlc {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if name := ParseTitleNameFromFileName(local.Dlc[id].ExtendedInfo.FileName); name != "" {
				return name
			}
		}
	}

	// With Unicode disabled, a CJK catalog title is only a last resort; prefer
	// local names so Japanese catalog text is not romanized unnecessarily.
	if remote != nil {
		if name := strings.TrimSpace(remote.Attributes.Name); name != "" {
			return name
		}
	}

	return "Unknown Title"
}

func embeddedEnglishTitle(file SwitchFileInfo) string {
	if file.Metadata == nil || file.Metadata.Ncap == nil {
		return ""
	}
	for _, language := range []string{"AmericanEnglish", "BritishEnglish"} {
		if title, ok := file.Metadata.Ncap.TitleName[language]; ok {
			if name := strings.TrimSpace(title.Title); name != "" {
				return name
			}
		}
	}
	return ""
}

func sortedGameUpdateVersions(local *SwitchGameFiles) []int {
	versions := make([]int, 0, len(local.Updates))
	for version := range local.Updates {
		versions = append(versions, version)
	}
	sort.Ints(versions)
	return versions
}
