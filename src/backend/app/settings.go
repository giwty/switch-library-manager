package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/trembon/switch-library-manager/backend/settings"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) LoadSettings() (*settings.AppSettings, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	result, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return nil, err
	}
	if a.ctx != nil {
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
	}
	return result, nil
}

type SettingsSaveResult struct {
	Settings        settings.AppSettings `json:"settings"`
	RestartRequired bool                 `json:"restart_required"`
	RestartReasons  []string             `json:"restart_reasons"`
	Library         *LocalLibraryData    `json:"library,omitempty"`
}

func (a *App) SaveSettings(value settings.AppSettings) (SettingsSaveResult, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	value = normalizeSettings(value)
	if err := settings.ValidateSettings(&value); err != nil {
		return SettingsSaveResult{}, err
	}
	current, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return SettingsSaveResult{}, fmt.Errorf("load current settings: %w", err)
	}
	reasons, invalidateSnapshot := settingsRestartReasons(*current, value)
	if invalidateSnapshot {
		if a.localDbManager == nil {
			return SettingsSaveResult{}, fmt.Errorf("invalidate library snapshot: library database is unavailable")
		}
		if err := a.localDbManager.ClearLocalLibrarySnapshot(); err != nil {
			return SettingsSaveResult{}, fmt.Errorf("invalidate library snapshot: %w", err)
		}
	}
	if err := settings.SaveSettingsWithError(&value, a.baseFolder); err != nil {
		return SettingsSaveResult{}, err
	}
	a.rememberWindowState = value.GUI.RememberWindowState
	result := SettingsSaveResult{
		Settings: normalizeSettings(value), RestartRequired: len(reasons) > 0, RestartReasons: reasons,
	}
	if !result.RestartRequired && current.Organization.AllowUnicodeFileNames != value.Organization.AllowUnicodeFileNames && a.state.localDB != nil {
		library := a.libraryDataLocked(a.state.localDB, value.Organization.AllowUnicodeFileNames)
		result.Library = &library
	}
	return result, nil
}

// Keep the GUI restart boundary in one place. Startup preferences and settings
// consumed by the next organization/missing-content request need no restart.
func settingsRestartReasons(current, candidate settings.AppSettings) ([]string, bool) {
	reasons := []string{}
	if current.Paths.LibraryFolder != candidate.Paths.LibraryFolder {
		reasons = append(reasons, "Library folder")
	}
	if !slices.Equal(current.Paths.ScanFolders, candidate.Paths.ScanFolders) {
		reasons = append(reasons, "Scan folders")
	}
	if current.Paths.ProdKeys != candidate.Paths.ProdKeys {
		reasons = append(reasons, "Keys path")
	}
	if current.Scan.Recursive != candidate.Scan.Recursive ||
		!slices.Equal(current.Scan.IgnoreFileTypes, candidate.Scan.IgnoreFileTypes) ||
		current.Scan.IgnoreUnsupportedFileTypes != candidate.Scan.IgnoreUnsupportedFileTypes {
		reasons = append(reasons, "Scan rules")
	}
	invalidateSnapshot := len(reasons) > 0
	if current.DataSources != candidate.DataSources {
		reasons = append(reasons, "Data sources")
	}
	if current.Logging != candidate.Logging {
		reasons = append(reasons, "Debug logging")
	}
	return reasons, invalidateSnapshot
}

func normalizeSettings(value settings.AppSettings) settings.AppSettings {
	value.Paths.ScanFolders = append([]string{}, value.Paths.ScanFolders...)
	value.MissingContent.IgnoreDLCTitleIDs = append([]string{}, value.MissingContent.IgnoreDLCTitleIDs...)
	value.MissingContent.IgnoreUpdateIDs = append([]string{}, value.MissingContent.IgnoreUpdateIDs...)
	value.Scan.IgnoreFileTypes = append([]string{}, value.Scan.IgnoreFileTypes...)
	return value
}

func (a *App) IsKeysFileAvailable() bool {
	keys, err := settings.SwitchKeys()
	return err == nil && keys != nil && keys.GetKey("header_key") != ""
}

func (a *App) CheckUpdate() (bool, error) {
	newUpdate, err := settings.CheckForUpdates()
	if err != nil && strings.Contains(err.Error(), "dial tcp") {
		return false, nil
	}
	return newUpdate, err
}
