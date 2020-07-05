package ui

import (
	"encoding/json"
	"fmt"
	"github.com/asticode/go-astilog"
	"github.com/giwty/switch-backup-manager/db"
	"github.com/giwty/switch-backup-manager/process"
	"github.com/giwty/switch-backup-manager/settings"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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
	logger   *astilog.Logger
}

type Message struct {
	Name    string `json:"name"`
	Payload string `json:"payload"`
}

type GUI struct {
	state State
}

func CreateGUI() *GUI {
	return &GUI{state: State{}}
}
func (g *GUI) Start() {
	file, err := os.OpenFile("./switch-backup-library.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	//l := log.New(file, log.Prefix(), log.Flags())
	logger := astilog.New(astilog.Configuration{
		Filename: "./switch-backup-library.log",
		AppName:  "switch-backup-manager",
		Level:    astilog.LevelWarn,
	})

	// Create astilectron
	a, err := astilectron.New(logger, astilectron.Options{
		AppName:           "Switch Library Manager",
		BaseDirectoryPath: "web",
	})
	if err != nil {
		logger.Fatal(fmt.Errorf("main: creating astilectron failed: %w", err))
	}
	defer a.Close()

	// Handle signals
	a.HandleSignals()

	// Start
	if err = a.Start(); err != nil {
		logger.Fatal(fmt.Errorf("main: starting astilectron failed: %w", err))
	}

	// New window
	var w *astilectron.Window
	basePath := ""
	if _, err := os.Stat("./web/app.html"); err != nil {
		basePath, err = os.Executable()
		if err != nil {
			logger.Fatal(fmt.Errorf("main: starting astilectron failed: %w", err))
		}
	}

	htmlFile := filepath.Join(filepath.Dir(basePath), "web/app.html")
	if w, err = a.NewWindow(htmlFile, &astilectron.WindowOptions{
		Center: astikit.BoolPtr(true),
		Height: astikit.IntPtr(700),
		Width:  astikit.IntPtr(700),
	}); err != nil {
		logger.Fatal(fmt.Errorf("main: new window failed: %w", err))
	}

	g.state.window = w
	g.state.logger = logger

	// Create windows
	if err = w.Create(); err != nil {
		logger.Fatal(fmt.Errorf("main: creating window failed: %w", err))
	}

	uiWorker := sync.Mutex{}

	// This will listen to messages sent by Javascript
	w.OnMessage(func(m *astilectron.EventMessage) interface{} {
		var retValue string
		uiWorker.Lock()
		defer uiWorker.Unlock()
		// Unmarshal
		msg := Message{}
		m.Unmarshal(&msg)

		switch msg.Name {
		case "organize":
			g.organizeLibrary()
		case "loadSettings":
			retValue = g.loadSettings()
		case "saveSettings":
			g.saveSettings(msg.Payload)
		case "updateLocalLibrary":
			localDB, err := g.buildLocalDB()
			if err != nil {
				logger.Error(err)
				g.state.window.SendMessage(Message{Name: "error", Payload: err.Error()}, func(m *astilectron.EventMessage) {})
				return ""
			}
			response := []LibraryTemplateData{}
			for k, v := range localDB.TitlesMap {
				if v.BaseExist {
					response = append(response,
						LibraryTemplateData{
							Icon:    g.state.switchDB.TitlesMap[k].Attributes.IconUrl,
							Name:    g.state.switchDB.TitlesMap[k].Attributes.Name,
							TitleId: v.File.Metadata.TitleId,
							Path:    v.File.Info.Name(),
						})
				}
			}
			msg, _ := json.Marshal(response)
			g.state.window.SendMessage(Message{Name: "libraryLoaded", Payload: string(msg)}, func(m *astilectron.EventMessage) {})
		case "updateDB":
			if g.state.switchDB == nil {
				switchDb, err := g.buildSwitchDb()
				if err != nil {
					logger.Error(err)
					g.state.window.SendMessage(Message{Name: "error", Payload: err.Error()}, func(m *astilectron.EventMessage) {})
					return ""
				}
				g.state.switchDB = switchDb
			}
		case "missingUpdates":
			return g.getMissingUpdates()
		case "missingDlc":
			return g.getMissingDLC()
		}

		return retValue
	})

	w.OpenDevTools()

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
	return settings.ReadSettingsAsJSON()
}

func (g *GUI) saveSettings(settingsJson string) {
	s := settings.AppSettings{}
	json.Unmarshal([]byte(settingsJson), &s)
	settings.SaveSettings(&s)
}

func (g *GUI) buildSwitchDb() (*db.SwitchTitlesDB, error) {
	settingsObj := settings.ReadSettings()
	//1. load the titles JSON object
	g.UpdateProgress(1, 4, "Downloading latest titles.json")
	titleFile, titlesEtag, err := db.LoadAndUpdateFile(settings.TITLES_JSON_URL, settings.TITLE_JSON_FILENAME, settingsObj.TitlesEtag)
	if err != nil {
		return nil, err
	}
	settingsObj.TitlesEtag = titlesEtag

	g.UpdateProgress(2, 4, "Downloading latest versions.json")
	versionsFile, versionsEtag, err := db.LoadAndUpdateFile(settings.VERSIONS_JSON_URL, settings.VERSIONS_JSON_FILENAME, settingsObj.VersionsEtag)
	if err != nil {
		return nil, err
	}
	settingsObj.VersionsEtag = versionsEtag

	settings.SaveSettings(settingsObj)

	g.UpdateProgress(3, 4, "Downloading latest versions.json")
	switchTitleDB, err := db.CreateSwitchTitleDB(titleFile, versionsFile)
	g.UpdateProgress(4, 4, "Done")
	return switchTitleDB, err
}

func (g *GUI) buildLocalDB() (*db.LocalSwitchFilesDB, error) {
	folderToScan := settings.ReadSettings().Folder
	recursiveMode := settings.ReadSettings().ScanRecursively

	files, err := ioutil.ReadDir(folderToScan)
	if err != nil {
		return nil, err
	}

	localDB, err := db.CreateLocalSwitchFilesDB(files, folderToScan, g, recursiveMode)
	g.state.localDB = localDB
	return localDB, err
}

func (g *GUI) organizeLibrary() {
	folderToScan := settings.ReadSettings().Folder
	process.OrganizeByFolders(folderToScan, g.state.localDB, g.state.switchDB, g)

}

func (g *GUI) UpdateProgress(curr int, total int, message string) {
	progressMessage := ProgressUpdate{curr, total, message}
	msg, err := json.Marshal(progressMessage)
	if err != nil {
		g.state.logger.Error(err)
	}

	g.state.window.SendMessage(Message{Name: "updateProgress", Payload: string(msg)}, func(m *astilectron.EventMessage) {})
}
