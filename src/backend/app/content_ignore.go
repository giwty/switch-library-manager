package app

import (
	"fmt"
	"strings"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

// SetContentIgnored changes a single saved missing-content preference without
// replacing unrelated settings or requiring the GUI to restart.
func (a *App) SetContentIgnored(kind string, titleID string, ignored bool) (settings.MissingContentSettings, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()

	if kind != "updates" && kind != "dlc" {
		return settings.MissingContentSettings{}, fmt.Errorf("unknown ignore kind %q", kind)
	}
	titleID = strings.ToLower(titleID)
	if _, err := db.TitleIDPrefix(titleID); err != nil {
		return settings.MissingContentSettings{}, fmt.Errorf("invalid ignored title ID: %w", err)
	}
	if strings.HasSuffix(titleID, "800") || (kind == "dlc" && strings.HasSuffix(titleID, "000")) {
		return settings.MissingContentSettings{}, fmt.Errorf("%s ignore requires %s", kind, map[string]string{
			"updates": "a base game or DLC title ID",
			"dlc":     "a DLC title ID",
		}[kind])
	}
	current, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return settings.MissingContentSettings{}, fmt.Errorf("load ignore settings: %w", err)
	}
	// SaveSettingsWithError publishes the candidate only after writing succeeds.
	// Copy both slices so neither the old settings nor the returned DTO aliases it.
	candidate := *current
	candidate.MissingContent.IgnoreUpdateIDs = append([]string{}, current.MissingContent.IgnoreUpdateIDs...)
	candidate.MissingContent.IgnoreDLCTitleIDs = append([]string{}, current.MissingContent.IgnoreDLCTitleIDs...)
	ids := &candidate.MissingContent.IgnoreUpdateIDs
	if kind == "dlc" {
		ids = &candidate.MissingContent.IgnoreDLCTitleIDs
	}
	result := make([]string, 0, len(*ids)+1)
	for _, id := range *ids {
		if !strings.EqualFold(id, titleID) {
			result = append(result, id)
		}
	}
	if ignored {
		result = append(result, titleID)
	}
	*ids = result
	if err := settings.SaveSettingsWithError(&candidate, a.baseFolder); err != nil {
		return settings.MissingContentSettings{}, fmt.Errorf("save ignore settings: %w", err)
	}
	saved := candidate.MissingContent
	saved.IgnoreUpdateIDs = append([]string{}, saved.IgnoreUpdateIDs...)
	saved.IgnoreDLCTitleIDs = append([]string{}, saved.IgnoreDLCTitleIDs...)
	return saved, nil
}
