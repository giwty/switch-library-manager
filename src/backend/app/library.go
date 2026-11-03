package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func (a *App) UpdateDB() error {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	if a.state.switchDB != nil {
		return nil
	}
	switchDB, err := a.buildSwitchDB()
	if err != nil {
		return err
	}
	a.state.switchDB = switchDB
	return nil
}

func (a *App) UpdateLocalLibrary(ignoreCache bool) (LocalLibraryData, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	return a.updateLocalLibraryLocked(ignoreCache)
}

func (a *App) RescanLibrary(hard bool) (LocalLibraryData, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	a.state.organizationIssues = nil
	if hard {
		if err := a.localDbManager.ClearScanData(); err != nil {
			return LocalLibraryData{}, err
		}
	}
	return a.updateLocalLibraryLocked(true)
}

func (a *App) updateLocalLibraryLocked(ignoreCache bool) (LocalLibraryData, error) {
	if a.state.switchDB == nil {
		return LocalLibraryData{}, errors.New("title database is not loaded")
	}

	localDB, err := a.buildLocalDB(ignoreCache)
	if err != nil {
		return LocalLibraryData{}, err
	}
	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return LocalLibraryData{}, err
	}
	return a.libraryDataLocked(localDB, settingsObj.Organization.AllowUnicodeFileNames), nil
}

func (a *App) libraryDataLocked(localDB *db.LocalSwitchFilesDB, allowUnicode bool) LocalLibraryData {
	data := buildLocalLibraryData(localDB, a.state.switchDB, allowUnicode)
	data.Issues = append(data.Issues, a.state.organizationIssues...)
	return data
}

func (a *App) buildSwitchDB() (switchTitleDB *db.SwitchTitlesDB, returnErr error) {
	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return nil, err
	}
	cache, err := settings.ReadCache(a.baseFolder)
	if err != nil {
		return nil, err
	}
	a.updateProgress(1, 4, "Downloading titles.json")

	filename := filepath.Join(a.baseFolder, settings.TITLE_JSON_FILENAME)
	titleFile, titleCache, err := db.LoadAndUpdateFile(settingsObj.DataSources.TitlesURL, filename, db.RemoteFileCache{
		URL: cache.TitlesURL, ETag: cache.TitlesETag, SHA256: cache.TitlesSHA256,
	}, db.ValidateTitlesJSON)
	if err != nil {
		return nil, errors.New("failed to download switch titles [reason:" + err.Error() + "]")
	}
	defer func() {
		if closeErr := titleFile.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close switch titles file: %w", closeErr))
			switchTitleDB = nil
		}
	}()
	cache.TitlesETag, cache.TitlesURL, cache.TitlesSHA256 = titleCache.ETag, titleCache.URL, titleCache.SHA256

	a.updateProgress(2, 4, "Downloading versions.json")
	filename = filepath.Join(a.baseFolder, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsCache, err := db.LoadAndUpdateFile(settingsObj.DataSources.VersionsURL, filename, db.RemoteFileCache{
		URL: cache.VersionsURL, ETag: cache.VersionsETag, SHA256: cache.VersionsSHA256,
	}, db.ValidateVersionsJSON)
	if err != nil {
		return nil, errors.New("failed to download switch updates [reason:" + err.Error() + "]")
	}
	defer func() {
		if closeErr := versionsFile.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close switch versions file: %w", closeErr))
			switchTitleDB = nil
		}
	}()
	cache.VersionsETag, cache.VersionsURL, cache.VersionsSHA256 = versionsCache.ETag, versionsCache.URL, versionsCache.SHA256
	if err := settings.SaveCacheWithError(cache, a.baseFolder); err != nil {
		return nil, fmt.Errorf("save title database cache: %w", err)
	}

	a.updateProgress(3, 4, "Processing switch titles and updates ...")
	switchTitleDB, err = db.CreateSwitchTitleDB(titleFile, versionsFile)
	a.updateProgress(4, 4, "Finishing up...")
	return switchTitleDB, err
}

func (a *App) buildLocalDB(ignoreCache bool) (*db.LocalSwitchFilesDB, error) {
	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return nil, err
	}
	scanFolders := append([]string{}, settingsObj.Paths.ScanFolders...)
	scanFolders = append(scanFolders, settingsObj.Paths.LibraryFolder)
	localDB, err := a.localDbManager.CreateLocalSwitchFilesDB(
		scanFolders,
		progressReporter{app: a},
		settingsObj.Scan.Recursive,
		ignoreCache,
	)
	if err == nil {
		a.state.localDB = localDB
	}
	return localDB, err
}
