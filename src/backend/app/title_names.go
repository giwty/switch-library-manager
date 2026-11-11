package app

import (
	"fmt"
	"strings"

	"github.com/trembon/switch-library-manager/backend/settings"
)

type GameNameOverrideResult struct {
	TitleNameOverrides map[string]string `json:"title_name_overrides"`
	Library            LocalLibraryData  `json:"library"`
	Details            GameDetails       `json:"details"`
}

// SetGameNameOverride saves one game name. An empty string explicitly removes
// the override. Persistence succeeds before any new state is published.
func (a *App) SetGameNameOverride(titleID, name string) (GameNameOverrideResult, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	titleID = strings.ToLower(titleID)
	// Validate the ID even for removals, without accepting blank custom names.
	probe := name
	if probe == "" {
		probe = "validation"
	}
	if _, err := settings.NormalizeTitleNameOverrides(map[string]string{titleID: probe}); err != nil {
		return GameNameOverrideResult{}, err
	}
	current, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return GameNameOverrideResult{}, fmt.Errorf("load game-name settings: %w", err)
	}
	candidate := normalizeSettings(*current)
	if name == "" {
		delete(candidate.TitleNameOverrides, titleID)
	} else {
		candidate.TitleNameOverrides[titleID] = strings.TrimSpace(name)
	}
	details, err := buildGameDetails(titleID, a.state.localDB, a.state.switchDB, candidate.MissingContent, candidate.Organization.AllowUnicodeFileNames, candidate.TitleNameOverrides)
	if err != nil {
		return GameNameOverrideResult{}, err
	}
	if err := settings.SaveSettingsWithError(&candidate, a.baseFolder); err != nil {
		return GameNameOverrideResult{}, fmt.Errorf("save game name: %w", err)
	}
	return GameNameOverrideResult{
		TitleNameOverrides: normalizeSettings(candidate).TitleNameOverrides,
		Library:            a.libraryDataLocked(a.state.localDB, candidate.Organization.AllowUnicodeFileNames, candidate.TitleNameOverrides),
		Details:            details,
	}, nil
}
