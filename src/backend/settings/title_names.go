package settings

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizeTitleNameOverrides validates and copies user-provided game names.
// Only base IDs are accepted: DLC names are independent of their parent game.
func NormalizeTitleNameOverrides(names map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(names))
	for id, name := range names {
		id = strings.ToLower(id)
		if len(id) != 16 || !strings.HasSuffix(id, "000") || strings.Trim(id, "0123456789abcdef") != "" {
			return nil, fmt.Errorf("title_name_overrides: %q must be a 16-character hexadecimal base game ID ending in 000", id)
		}
		if !utf8.ValidString(name) || strings.ContainsFunc(name, unicode.IsControl) {
			return nil, fmt.Errorf("title_name_overrides[%s]: name must be valid text without control characters", id)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("title_name_overrides[%s]: name must not be blank", id)
		}
		if previous, exists := result[id]; exists && previous != name {
			return nil, fmt.Errorf("title_name_overrides: conflicting names for %s", id)
		}
		result[id] = name
	}
	return result, nil
}
