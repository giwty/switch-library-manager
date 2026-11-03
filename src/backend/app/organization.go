package app

import (
	"errors"
	"fmt"

	"github.com/trembon/switch-library-manager/backend/process"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func (a *App) GetOrganizationPreview() (OrganizationPreview, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()

	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return OrganizationPreview{}, err
	}
	if !process.IsOptionsValid(settingsObj.Organization) {
		return OrganizationPreview{}, errors.New("the organize options in settings.json are not valid, please check that the template contains file/folder name")
	}

	entries, err := process.BuildOrganizationPreview(
		settingsObj.Paths.LibraryFolder,
		settingsObj.Organization,
		a.state.localDB,
		a.state.switchDB,
	)
	if err != nil {
		return OrganizationPreview{}, err
	}
	previewEntries := make([]OrganizationPreviewEntry, 0, len(entries))
	for _, entry := range entries {
		previewEntries = append(previewEntries, OrganizationPreviewEntry{Kind: entry.Kind, Path: entry.Path})
	}
	return OrganizationPreview{RootFolder: settingsObj.Paths.LibraryFolder, Entries: previewEntries}, nil
}

func (a *App) OrganizeLibrary() (OrganizationResult, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	if a.state.switchDB == nil || a.state.localDB == nil {
		return OrganizationResult{}, errors.New("local and title databases must be loaded")
	}

	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return OrganizationResult{}, err
	}
	if !process.IsOptionsValid(settingsObj.Organization) {
		return OrganizationResult{}, errors.New("the organize options in settings.json are not valid, please check that the template contains file/folder name")
	}
	a.state.organizationIssues = nil
	progress := progressReporter{app: a}
	localDB, err := a.buildLocalDB(true)
	if err != nil {
		return OrganizationResult{}, err
	}
	var plan process.OrganizationPlan
	organizationEnabled := settingsObj.Organization.RenameFiles || settingsObj.Organization.CreateFolderPerGame || settingsObj.Organization.MoveScanFilesToLibrary
	if organizationEnabled {
		plan, err = process.BuildOrganizationPlan(settingsObj.Paths.LibraryFolder, localDB, a.state.switchDB, settingsObj.Organization)
		if err != nil {
			return OrganizationResult{}, err
		}
		if len(plan.Conflicts) != 0 {
			a.state.organizationIssues = organizationConflictsAsIssues(plan.Conflicts)
			return OrganizationResult{
				Status:        "blocked",
				ConflictCount: len(plan.Conflicts),
				Library:       a.libraryDataLocked(localDB, settingsObj.Organization.AllowUnicodeFileNames),
			}, nil
		}
	}
	if err := a.localDbManager.ClearLocalLibrarySnapshot(); err != nil {
		return OrganizationResult{}, fmt.Errorf("clear cached library before organization: %w", err)
	}
	var returnErr error
	changed := false
	if settingsObj.Organization.DeleteOldUpdateFiles {
		changed = len(localDB.CleanupCandidates) != 0
		if err := process.DeleteOldUpdatesAt(a.baseFolder, settingsObj.Paths.LibraryFolder, localDB, progress); err != nil {
			returnErr = err
		}
	}
	if returnErr == nil && organizationEnabled && len(plan.Moves) != 0 {
		if err := process.ExecuteOrganizationPlan(plan, localDB, settingsObj.Paths.LibraryFolder, settingsObj.Organization, progress, settingsObj.Paths.ScanFolders...); err != nil {
			returnErr = err
		} else {
			changed = true
		}
	}
	if changed || returnErr != nil {
		if _, refreshErr := a.buildLocalDB(true); refreshErr != nil {
			a.state.localDB = nil
			if clearErr := a.localDbManager.ClearLocalLibrarySnapshot(); clearErr != nil {
				refreshErr = errors.Join(refreshErr, clearErr)
			}
			returnErr = errors.Join(returnErr, fmt.Errorf("refresh library after organization: %w", refreshErr))
		}
	}
	if returnErr != nil {
		return OrganizationResult{}, returnErr
	}
	return OrganizationResult{
		Status:  "completed",
		Library: a.libraryDataLocked(a.state.localDB, settingsObj.Organization.AllowUnicodeFileNames),
	}, nil
}

func organizationConflictsAsIssues(conflicts []process.OrganizationConflict) []Pair {
	issues := make([]Pair, 0, len(conflicts))
	for _, conflict := range conflicts {
		detail := fmt.Sprintf("organization conflict: %s; destination: %s", conflict.Reason, conflict.Destination)
		if conflict.OtherSource != "" && conflict.OtherSource != conflict.Source {
			detail += "; also used by: " + conflict.OtherSource
		}
		issues = append(issues, Pair{Key: conflict.Source, Value: detail})
	}
	return issues
}
