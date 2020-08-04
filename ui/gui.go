package ui

import (
	"encoding/json"
	"fmt"
	"github.com/asticode/go-astilog"
	"github.com/giwty/switch-library-manager/db"
	"github.com/giwty/switch-library-manager/process"
	"github.com/giwty/switch-library-manager/settings"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/asticode/go-astikit"
	"github.com/asticode/go-astilectron"
)

type LibraryTemplateData struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
	Dlc     string `json:"dlc"`
	TitleId string `json:"titleId"`
	Path    string `json:"path"`
	Icon    string `json:"icon"`
}

type ProgressUpdate struct {
	Curr    int    `json:"curr"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

type State struct {
	switchDB *db.SwitchTitlesDB
	localDB  *db.LocalSwitchFilesDB
	window   *astilectron.Window
}

type Message struct {
	Name    string `json:"name"`
	Payload string `json:"payload"`
}

type GUI struct {
	state       State
	baseFolder  string
	sugarLogger *zap.SugaredLogger
}

func CreateGUI(baseFolder string, sugarLogger *zap.SugaredLogger) *GUI {
	return &GUI{state: State{}, baseFolder: baseFolder, sugarLogger: sugarLogger}
}
func (g *GUI) Start() {

	webResourcesPath := filepath.Join(g.baseFolder, "web")
	if _, err := os.Stat(webResourcesPath); err != nil {
		g.sugarLogger.Error("Missing web folder, please re-download latest release, and extract all files. aborting")
		return
	}

	//l := log.New(file, log.Prefix(), log.Flags())
	logger := astilog.New(astilog.Configuration{
		Level: astilog.LevelInfo,
	})

	// Create astilectron
	a, err := astilectron.New(logger, astilectron.Options{
		AppName:           "Switch Library Manager",
		BaseDirectoryPath: webResourcesPath,
	})
	if err != nil {
		g.sugarLogger.Error("Failed to create astilectron (Electorn)\n", err)
		return
	}
	defer a.Close()

	// Handle signals
	a.HandleSignals()

	// Start
	zap.S().Infof("Downloading/Validating electron files (web/vendor)")
	fmt.Println("Downloading/Validating electron files .. (can take 1-2min)")
	if err = a.Start(); err != nil {
		g.sugarLogger.Error("Failed to start astilectron (Electorn), please try to delete the web/vendor folder and try again\n", err)
		return
	}

	// New window
	var w *astilectron.Window

	htmlFile := filepath.Join(webResourcesPath, "app.html")
	if w, err = a.NewWindow(htmlFile, &astilectron.WindowOptions{
		Center: astikit.BoolPtr(true),
		Height: astikit.IntPtr(600),
		Width:  astikit.IntPtr(1200),
	}); err != nil {
		g.sugarLogger.Fatal(fmt.Errorf("main: new window failed: %w", err))
	}

	g.state.window = w

	// Create windows
	if err = w.Create(); err != nil {
		g.sugarLogger.Error("Failed creating window (Electorn)", err)
		return
	}

	uiWorker := sync.Mutex{}

	settings.InitSwitchKeys(g.baseFolder)

	localDbManager, err := db.NewLocalSwitchDBManager(g.baseFolder)
	if err != nil {
		g.sugarLogger.Error("Failed to create local files db\n", err)
		return
	}
	defer localDbManager.Close()

	// This will listen to messages sent by Javascript
	w.OnMessage(func(m *astilectron.EventMessage) interface{} {

		var retValue string
		uiWorker.Lock()
		defer uiWorker.Unlock()
		// Unmarshal
		msg := Message{}
		err = m.Unmarshal(&msg)

		if err != nil {
			g.sugarLogger.Error("Failed to parse client message", err)
			return ""
		}

		g.sugarLogger.Debugf("Received message from client [%v]", msg)

		switch msg.Name {
		case "organize":
			g.organizeLibrary()
		case "isKeysFileAvailable":
			keys, _ := settings.SwitchKeys()
			retValue = strconv.FormatBool(keys != nil && keys.GetKey("header_key") != "")
		case "loadSettings":
			retValue = g.loadSettings()
		case "saveSettings":
			g.saveSettings(msg.Payload)
		case "updateLocalLibrary":
			localDB, err := g.buildLocalDB(localDbManager)
			if err != nil {
				g.sugarLogger.Error(err)
				g.state.window.SendMessage(Message{Name: "error", Payload: err.Error()}, func(m *astilectron.EventMessage) {})
				return ""
			}
			response := []LibraryTemplateData{}
			for k, v := range localDB.TitlesMap {
				if v.BaseExist {
					if title, ok := g.state.switchDB.TitlesMap[k]; ok {
						response = append(response,
							LibraryTemplateData{
								Icon:    title.Attributes.IconUrl,
								Name:    title.Attributes.Name,
								TitleId: v.File.Metadata.TitleId,
								Path:    filepath.Join(v.File.BaseFolder, v.File.Info.Name()),
							})
					} else {
						response = append(response,
							LibraryTemplateData{
								Name:    db.ParseTitleNameFromFileName(v.File.Info.Name()),
								TitleId: v.File.Metadata.TitleId,
								Path:    v.File.Info.Name(),
							})
					}

				}
			}
			msg, _ := json.Marshal(response)
			g.state.window.SendMessage(Message{Name: "libraryLoaded", Payload: string(msg)}, func(m *astilectron.EventMessage) {})
		case "updateDB":
			if g.state.switchDB == nil {
				switchDb, err := g.buildSwitchDb()
				if err != nil {
					g.sugarLogger.Error(err)
					g.state.window.SendMessage(Message{Name: "error", Payload: err.Error()}, func(m *astilectron.EventMessage) {})
					return ""
				}
				g.state.switchDB = switchDb
			}
		case "missingUpdates":
			retValue = g.getMissingUpdates()
		case "missingDlc":
			retValue = g.getMissingDLC()
		case "checkUpdate":
			newUpdate, err := settings.CheckForUpdates(g.baseFolder)
			if err != nil {
				g.sugarLogger.Error(err)
			}
			retValue = strconv.FormatBool(newUpdate)
		}

		g.sugarLogger.Debugf("Server response [%v]", retValue)

		return retValue
	})

	/*if settings.ReadSettings(g.baseFolder).Debug {
		w.OpenDevTools()
	}*/

	// Blocking pattern
	a.Wait()
}

func (g *GUI) getMissingDLC() string {
	missingDLC := process.ScanForMissingDLC(g.state.localDB.TitlesMap, g.state.switchDB.TitlesMap)
	values := make([]process.IncompleteTitle, len(missingDLC))
	i := 0
	for _, missingUpdate := range missingDLC {
		values[i] = missingUpdate
		i++
	}

	msg, _ := json.Marshal(values)
	return string(msg)
}

func (g *GUI) getMissingUpdates() string {
	missingUpdates := process.ScanForMissingUpdates(g.state.localDB.TitlesMap, g.state.switchDB.TitlesMap)
	values := make([]process.IncompleteTitle, len(missingUpdates))
	i := 0
	for _, missingUpdate := range missingUpdates {
		values[i] = missingUpdate
		i++
	}

	msg, _ := json.Marshal(values)
	return string(msg)
}

func (g *GUI) loadSettings() string {
	return settings.ReadSettingsAsJSON(g.baseFolder)
}

func (g *GUI) saveSettings(settingsJson string) {
	s := settings.AppSettings{}
	json.Unmarshal([]byte(settingsJson), &s)
	settings.SaveSettings(&s, g.baseFolder)
}

func (g *GUI) buildSwitchDb() (*db.SwitchTitlesDB, error) {
	settingsObj := settings.ReadSettings(g.baseFolder)
	//1. load the titles JSON object
	g.UpdateProgress(1, 4, "Downloading titles.json")
	filename := filepath.Join(g.baseFolder, settings.TITLE_JSON_FILENAME)
	titleFile, titlesEtag, err := db.LoadAndUpdateFile(settings.TITLES_JSON_URL, filename, settingsObj.TitlesEtag)
	if err != nil {
		return nil, err
	}
	settingsObj.TitlesEtag = titlesEtag

	g.UpdateProgress(2, 4, "Downloading versions.json")
	filename = filepath.Join(g.baseFolder, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsEtag, err := db.LoadAndUpdateFile(settings.VERSIONS_JSON_URL, filename, settingsObj.VersionsEtag)
	if err != nil {
		return nil, err
	}
	settingsObj.VersionsEtag = versionsEtag

	settings.SaveSettings(settingsObj, g.baseFolder)

	g.UpdateProgress(3, 4, "Building titles DB ...")
	switchTitleDB, err := db.CreateSwitchTitleDB(titleFile, versionsFile)
	g.UpdateProgress(4, 4, "Done")
	return switchTitleDB, err
}

func (g *GUI) buildLocalDB(localDbManager *db.LocalSwitchDBManager) (*db.LocalSwitchFilesDB, error) {
	folderToScan := settings.ReadSettings(g.baseFolder).Folder
	recursiveMode := settings.ReadSettings(g.baseFolder).ScanRecursively

	files, err := ioutil.ReadDir(folderToScan)
	if err != nil {
		return nil, err
	}

	localDB, err := localDbManager.CreateLocalSwitchFilesDB(files, folderToScan, g, recursiveMode)
	g.state.localDB = localDB
	return localDB, err
}

func (g *GUI) organizeLibrary() {
	folderToScan := settings.ReadSettings(g.baseFolder).Folder
	process.OrganizeByFolders(folderToScan, g.state.localDB, g.state.switchDB, g)

}

func (g *GUI) UpdateProgress(curr int, total int, message string) {
	progressMessage := ProgressUpdate{curr, total, message}
	g.sugarLogger.Debugf("process %v (%v/%v)", message, curr, total)
	msg, err := json.Marshal(progressMessage)
	if err != nil {
		g.sugarLogger.Error(err)
		return
	}

	g.state.window.SendMessage(Message{Name: "updateProgress", Payload: string(msg)}, func(m *astilectron.EventMessage) {})
}
