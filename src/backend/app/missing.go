package app

import (
	"errors"
	"strings"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/process"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func (a *App) GetMissingGames() ([]SwitchTitle, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	if a.state.switchDB == nil || a.state.localDB == nil {
		return nil, errors.New("local and title databases must be loaded")
	}
	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return nil, err
	}
	return getMissingGames(a.state.localDB, a.state.switchDB, settingsObj), nil
}

func (a *App) GetMissingDLC() ([]process.IncompleteTitle, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	if a.state.switchDB == nil || a.state.localDB == nil {
		return nil, errors.New("local and title databases must be loaded")
	}
	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return nil, err
	}
	ignoreIDs := makeIgnoreIDs(settingsObj.MissingContent.IgnoreDLCTitleIDs)
	missing := process.ScanForMissingDLC(a.state.localDB.TitlesMap, a.state.switchDB.TitlesMap, ignoreIDs, settingsObj.TitleNameOverrides)
	values := make([]process.IncompleteTitle, 0, len(missing))
	for _, value := range missing {
		values = append(values, value)
	}
	return values, nil
}

func (a *App) GetMissingUpdates() ([]process.IncompleteTitle, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	if a.state.switchDB == nil || a.state.localDB == nil {
		return nil, errors.New("local and title databases must be loaded")
	}
	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return nil, err
	}
	ignoreIDs := makeIgnoreIDs(settingsObj.MissingContent.IgnoreUpdateIDs)
	missing := process.ScanForMissingUpdates(
		a.state.localDB.TitlesMap,
		a.state.switchDB.TitlesMap,
		ignoreIDs,
		settingsObj.MissingContent.IgnoreDLCUpdates,
		settingsObj.TitleNameOverrides,
	)
	values := make([]process.IncompleteTitle, 0, len(missing))
	for _, value := range missing {
		values = append(values, value)
	}
	return values, nil
}

func makeIgnoreIDs(ids []string) map[string]struct{} {
	ignoreIDs := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		ignoreIDs[strings.ToLower(id)] = struct{}{}
	}
	return ignoreIDs
}

func getMissingGames(localDB *db.LocalSwitchFilesDB, switchDB *db.SwitchTitlesDB, settingsObj *settings.AppSettings) []SwitchTitle {
	var result []SwitchTitle
	for key, title := range switchDB.TitlesMap {
		if _, ok := localDB.TitlesMap[key]; ok {
			continue
		}
		if title.Attributes.Name == "" || title.Attributes.Id == "" {
			continue
		}
		if settingsObj.GUI.HideDemoGames && title.Attributes.IsDemo {
			continue
		}
		result = append(result, SwitchTitle{
			TitleId:     title.Attributes.Id,
			Name:        db.GameName(key+"000", title.Attributes.Name, settingsObj.TitleNameOverrides),
			Icon:        title.Attributes.BannerUrl,
			Region:      title.Attributes.Region,
			ReleaseDate: title.Attributes.ParsedReleaseDate,
		})
	}
	return result
}
