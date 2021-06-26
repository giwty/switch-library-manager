package gui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/asticode/go-astilectron"
	"github.com/giwty/switch-library-manager/db"
	"github.com/giwty/switch-library-manager/process"
	"github.com/giwty/switch-library-manager/settings"
	"go.uber.org/zap"
)

const (
	GUI_MESSAGE_ERROR                = "error"
	GUI_MESSAGE_UPDATE_PROGRESS      = "updateProgress"
	GUI_MESSAGE_ORGANIZE             = "organize"
	GUI_MESSAGE_KEYS_AVAILABLE       = "isKeysFileAvailable"
	GUI_MESSAGE_LOAD_SETTINGS        = "loadSettings"
	GUI_MESSAGE_SAVE_SETTINGS        = "saveSettings"
	GUI_MESSAGE_MISSING_GAMES        = "missingGames"
	GUI_MESSAGE_UPDATE_LOCAL_LIBRARY = "updateLocalLibrary"
	GUI_MESSAGE_LIBRARY_LOADED       = "libraryLoaded"
	GUI_MESSAGE_UPDATE_DATABASE      = "updateDB"
	GUI_MESSAGE_MISSING_UPDATES      = "missingUpdates"
	GUI_MESSAGE_MISSING_DLC          = "missingDlc"
	GUI_MESSAGE_CHECK_UPDATE         = "checkUpdate"
	GUI_MESSAGE_RESCAN               = "rescan"
)

// GUI message
type Message struct {
	Name    string `json:"name"`
	Payload string `json:"payload"`
}

// GUI version of the app
type GUI struct {
	db struct {
		manager *db.LocalSwitchDBManager
		titles  *db.SwitchTitlesDB
		files   *db.LocalSwitchFilesDB
	}
	logger        *zap.SugaredLogger
	settings      *settings.AppSettings
	state         sync.Mutex
	Window        *astilectron.Window
	workingFolder string
}

// Constructor for GUI
func NewGUI(l *zap.SugaredLogger, s *settings.AppSettings, w string) *GUI {
	g := &GUI{
		logger:        l,
		settings:      s,
		workingFolder: w,
	}

	return g
}

// Init the GUI
func (g *GUI) Init() {
	// Grab the switch keys
	g.settings.ReadKeys()

	// Instantiate the database manager
	g.db.manager = db.NewLocalSwitchDBManager(g.workingFolder, g.logger, g.settings)
}

// Clean up
func (g *GUI) Defer() {
	g.db.manager.Close()
}

// Handle communication with the frontend
func (g *GUI) HandleMessage(m *astilectron.EventMessage) interface{} {
	var retValue string

	// Lock the mutex and unlock on return
	g.state.Lock()
	defer g.state.Unlock()

	// Decode the message
	msg := Message{}
	err := m.Unmarshal(&msg)

	if err != nil {
		g.logger.Errorf("Failed to parse client message: %s", err)
		return ""
	}

	g.logger.Debugf("Received message from client [%v]", msg)

	// Process the message
	switch msg.Name {
	// Organize the library
	case GUI_MESSAGE_ORGANIZE:
		g.organizeLibrary()

	// Check if prod keys are available
	case GUI_MESSAGE_KEYS_AVAILABLE:
		retValue = g.settings.HasKey(settings.SETTINGS_PRODKEYS_HEADER)

	// Load the settings
	case GUI_MESSAGE_LOAD_SETTINGS:
		retValue = g.settings.ToJSON()

	// Save the settings
	case GUI_MESSAGE_SAVE_SETTINGS:
		if err := g.saveSettings(msg.Payload); err != nil {
			g.logger.Error(err)
			g.Send(GUI_MESSAGE_ERROR, err.Error())
			return ""
		}

	// Get the missing games
	case GUI_MESSAGE_MISSING_GAMES:
		missingGames := g.getMissingGames()
		g.Send(GUI_MESSAGE_MISSING_GAMES, string(missingGames))

	// Update the local library
	case GUI_MESSAGE_UPDATE_LOCAL_LIBRARY:
		ignoreCache, _ := strconv.ParseBool(msg.Payload)

		// Update the local DB
		if err := g.buildLocalDB(ignoreCache); err != nil {
			g.logger.Error(err)
			g.Send(GUI_MESSAGE_ERROR, err.Error())
			return ""
		}

		// Update the local library
		localLibrary := g.updateLocalLibrary()
		g.Send(GUI_MESSAGE_LIBRARY_LOADED, string(localLibrary))

	// Update the database
	case GUI_MESSAGE_UPDATE_DATABASE:
		// Only if not loaded already
		if g.db.titles == nil {
			if err := g.buildSwitchDb(); err != nil {
				g.logger.Error(err)
				g.Send(GUI_MESSAGE_ERROR, err.Error())
				return ""
			}
		}

	// Get the missing updates
	case GUI_MESSAGE_MISSING_UPDATES:
		retValue = g.getMissingUpdates()

	// Get the missing DLCs
	case GUI_MESSAGE_MISSING_DLC:
		retValue = g.getMissingDLC()

	// Check for update
	case GUI_MESSAGE_CHECK_UPDATE:
		newUpdate, err := settings.CheckForUpdates()
		if err != nil {
			g.logger.Error(err)
			g.Send(GUI_MESSAGE_ERROR, err.Error())
			return ""
		}
		retValue = strconv.FormatBool(newUpdate)
	}

	g.logger.Debugf("Server response [%v]", retValue)

	return retValue
}

// Update the local database
func (g *GUI) buildLocalDB(ignoreCache bool) error {
	folders := g.settings.ScanFolders
	folders = append(folders, g.settings.Folder)

	var err error
	g.db.files, err = g.db.manager.CreateLocalSwitchFilesDB(folders, g, g.settings.ScanRecursively, ignoreCache)

	return err
}

// Organize the library
func (g *GUI) organizeLibrary() {
	// Bail if options are invalid
	if !process.IsOptionsValid(g.settings.OrganizeOptions) {
		g.logger.Error("the organize options in settings.json are not valid, please check that the template contains file/folder name")
		g.Send(GUI_MESSAGE_ERROR, "the organize options in settings.json are not valid, please check that the template contains file/folder name")
		return
	}

	// Organize
	process.OrganizeByFolders(g.settings.Folder, g.db.files, g.db.titles, g, g.settings)

	// Delete old updates if requested
	if g.settings.OrganizeOptions.DeleteOldUpdateFiles {
		process.DeleteOldUpdates(g.workingFolder, g.db.files, g, g.settings)
	}
}

// Get missing games
func (g *GUI) getMissingGames() []byte {
	var result []switchTitle

	// Browser the titles DB
	for k, v := range g.db.titles.TitlesMap {
		// Skip if present in the files
		if _, ok := g.db.files.TitlesMap[k]; ok {
			continue
		}

		// Skip if missing attributes
		if v.Attributes.Name == "" || v.Attributes.Id == "" {
			continue
		}

		// Add to the results if missing
		result = append(result, switchTitle{
			TitleId:     v.Attributes.Id,
			Name:        v.Attributes.Name,
			Icon:        v.Attributes.BannerUrl,
			Region:      v.Attributes.Region,
			ReleaseDate: v.Attributes.ReleaseDate,
		})
	}

	// Marshal into JSON
	missingGames, _ := json.Marshal(result)

	return missingGames
}

// Update the local library
func (g *GUI) updateLocalLibrary() []byte {
	response := LocalLibraryData{}
	libraryData := []LibraryTemplateData{}
	issues := []Issue{}

	// Processed files
	for k, v := range g.db.files.TitlesMap {
		// If the base game exists
		if v.BaseExist {
			version := ""
			name := ""
			if v.File.Metadata.Ncap != nil {
				version = v.File.Metadata.Ncap.DisplayVersion
				name = v.File.Metadata.Ncap.TitleName["AmericanEnglish"].Title
			}

			// Checking updates
			if v.Updates != nil && len(v.Updates) != 0 {
				if v.Updates[v.LatestUpdate].Metadata.Ncap != nil {
					version = v.Updates[v.LatestUpdate].Metadata.Ncap.DisplayVersion
				} else {
					version = ""
				}
			}

			// Checking if available in the titles DB
			if title, ok := g.db.titles.TitlesMap[k]; ok {
				if title.Attributes.Name != "" {
					name = title.Attributes.Name
				}
				libraryData = append(libraryData,
					LibraryTemplateData{
						Icon:    title.Attributes.IconUrl,
						Name:    name,
						TitleId: v.File.Metadata.TitleId,
						Update:  v.LatestUpdate,
						Version: version,
						Region:  title.Attributes.Region,
						Type:    v.Type(),
						Path:    filepath.Join(v.File.ExtendedInfo.BaseFolder, v.File.ExtendedInfo.FileName),
					})
			} else {
				if name == "" {
					name = db.ParseTitleNameFromFileName(v.File.ExtendedInfo.FileName)
				}
				libraryData = append(libraryData,
					LibraryTemplateData{
						Name:    name,
						Update:  v.LatestUpdate,
						Version: version,
						Type:    v.Type(),
						TitleId: v.File.Metadata.TitleId,
						Path:    v.File.ExtendedInfo.FileName,
					})
			}

		} else {
			for _, update := range v.Updates {
				issues = append(issues, Issue{Key: filepath.Join(update.ExtendedInfo.BaseFolder, update.ExtendedInfo.FileName), Value: "base file is missing"})
			}
			for _, dlc := range v.Dlc {
				issues = append(issues, Issue{Key: filepath.Join(dlc.ExtendedInfo.BaseFolder, dlc.ExtendedInfo.FileName), Value: "base file is missing"})
			}
		}
	}

	// Skipped files
	for k, v := range g.db.files.Skipped {
		issues = append(issues, Issue{Key: filepath.Join(k.BaseFolder, k.FileName), Value: v.ReasonText})
	}

	response.LibraryData = libraryData
	response.NumFiles = g.db.files.NumFiles
	response.Issues = issues

	// Marshal into JSON
	localLibrary, _ := json.Marshal(response)

	return localLibrary
}

// Update the titles database
func (g *GUI) buildSwitchDb() error {
	// Load the titles
	g.UpdateProgress(1, 4, "Downloading titles.json")

	titleFile, titlesEtag, titlesErr := db.LoadAndUpdateFile(
		settings.TITLES_JSON_URL,
		filepath.Join(g.workingFolder, settings.TITLE_JSON_FILENAME),
		g.settings.TitlesEtag,
	)

	if titlesErr != nil {
		return fmt.Errorf("failed to download switch titles [reason:%s]", titlesErr)
	}

	// Update the titles Etag
	g.settings.TitlesEtag = titlesEtag

	// Load the versions
	g.UpdateProgress(2, 4, "Downloading versions.json")

	versionsFile, versionsEtag, versionsErr := db.LoadAndUpdateFile(
		settings.VERSIONS_JSON_URL,
		filepath.Join(g.workingFolder, settings.VERSIONS_JSON_FILENAME),
		g.settings.VersionsEtag,
	)

	if versionsErr != nil {
		return fmt.Errorf("failed to download switch updates [reason:%s]", versionsErr)
	}

	// Update the versions Etag
	g.settings.VersionsEtag = versionsEtag

	// Save the settings
	g.settings.Save()

	// Create the switch database
	g.UpdateProgress(3, 4, "Processing switch titles and updates ...")
	var dbErr error
	g.db.titles, dbErr = db.CreateSwitchTitleDB(titleFile, versionsFile)

	// Finishing
	g.UpdateProgress(4, 4, "Finishing up...")
	return dbErr
}

// Get the missing updates
func (g *GUI) getMissingUpdates() string {
	missingUpdates := process.ScanForMissingUpdates(g.db.files.TitlesMap, g.db.titles.TitlesMap)
	values := make([]process.IncompleteTitle, len(missingUpdates))
	i := 0
	for _, missingUpdate := range missingUpdates {
		values[i] = missingUpdate
		i++
	}

	msg, _ := json.Marshal(values)
	return string(msg)
}

// Get the missing DLCs
func (g *GUI) getMissingDLC() string {
	ignoreIds := map[string]struct{}{}
	for _, id := range g.settings.IgnoreDLCTitleIds {
		ignoreIds[strings.ToLower(id)] = struct{}{}
	}
	missingDLC := process.ScanForMissingDLC(g.db.files.TitlesMap, g.db.titles.TitlesMap, ignoreIds)
	values := make([]process.IncompleteTitle, len(missingDLC))
	i := 0
	for _, missingUpdate := range missingDLC {
		values[i] = missingUpdate
		i++
	}

	msg, _ := json.Marshal(values)
	return string(msg)
}

// Save settings
func (g *GUI) saveSettings(settingsJSON string) error {
	// Load the payload
	if err := g.settings.Load([]byte(settingsJSON)); err != nil {
		return err
	}

	// Save the settings
	g.settings.Save()

	return nil
}

// Update progress on operations
// Implements db.ProgressUpdater interface
func (g *GUI) UpdateProgress(curr int, total int, message string) {
	progressMessage := db.ProgressUpdate{
		Curr:    curr,
		Total:   total,
		Message: message,
	}
	g.logger.Debugf("%v (%v/%v)", message, curr, total)

	// Create progress message
	msg, err := json.Marshal(progressMessage)
	if err != nil {
		g.logger.Error(err)
		return
	}

	g.Send(GUI_MESSAGE_UPDATE_PROGRESS, string(msg))
}

// Send a message to the window
func (g *GUI) Send(name string, message string) {
	g.Window.SendMessage(Message{
		Name:    name,
		Payload: message,
	},
		func(m *astilectron.EventMessage) {},
	)
}

// Clear
func (g *GUI) Clear() {
	g.db.manager.ClearScanData()
}
