package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/asticode/go-astikit"
	"github.com/asticode/go-astilectron"
	bootstrap "github.com/asticode/go-astilectron-bootstrap"
	"github.com/giwty/switch-library-manager/db"
	"github.com/giwty/switch-library-manager/process"
	"github.com/giwty/switch-library-manager/settings"
	"go.uber.org/zap"
)

type Pair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type LocalLibraryData struct {
	LibraryData []LibraryTemplateData `json:"library_data"`
	Issues      []Pair                `json:"issues"`
	NumFiles    int                   `json:"num_files"`
}

type SwitchTitle struct {
	Name        string `json:"name"`
	TitleId     string `json:"titleId"`
	Icon        string `json:"icon"`
	Region      string `json:"region"`
	ReleaseDate int    `json:"release_date"`
}

type LibraryTemplateData struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Dlc     string `json:"dlc"`
	TitleId string `json:"titleId"`
	Path    string `json:"path"`
	Icon    string `json:"icon"`
	Update  int    `json:"update"`
	Region  string `json:"region"`
	Type    string `json:"type"`
}

type ProgressUpdate struct {
	Curr    int    `json:"curr"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

type State struct {
	sync.Mutex
	switchDB *db.SwitchTitlesDB
	localDB  *db.LocalSwitchFilesDB
	window   *astilectron.Window
}

type Message struct {
	Name    string `json:"name"`
	Payload string `json:"payload"`
}

type GUI struct {
	state          State
	baseFolder     string
	localDbManager *db.LocalSwitchDBManager
	sugarLogger    *zap.SugaredLogger
}

func CreateGUI(baseFolder string, sugarLogger *zap.SugaredLogger) *GUI {
	return &GUI{state: State{}, baseFolder: baseFolder, sugarLogger: sugarLogger}
}
func (g *GUI) Start() {

	localDbManager, err := db.NewLocalSwitchDBManager(g.baseFolder)
	if err != nil {
		g.sugarLogger.Error("Failed to create local files db\n", err)
		return
	}

	settings.InitSwitchKeys(g.baseFolder)

	g.localDbManager = localDbManager
	defer localDbManager.Close()
	// Run bootstrap
	if err := bootstrap.Run(bootstrap.Options{
		Asset:    Asset,
		AssetDir: AssetDir,
		AstilectronOptions: astilectron.Options{
			AppName:            "Switch Library Manager (" + settings.SLM_VERSION + ")",
			AcceptTCPTimeout:   time.Duration(5) * time.Second,
			AppIconDarwinPath:  "resources/icon.icns",
			AppIconDefaultPath: "resources/icon.png",
			SingleInstance:     true,
			//VersionAstilectron: VersionAstilectron,
			//VersionElectron:    VersionElectron,
		},
		Debug:         false,
		Logger:        log.New(log.Writer(), log.Prefix(), log.Flags()),
		RestoreAssets: RestoreAssets,
		Windows: []*bootstrap.Window{{
			Homepage: "app.html",
			Adapter: func(w *astilectron.Window) {
				g.state.window = w
				g.state.window.OnMessage(g.handleMessage)
				//g.state.window.OpenDevTools()
			},
			Options: &astilectron.WindowOptions{
				BackgroundColor: astikit.StrPtr("#333"),
				Center:          astikit.BoolPtr(true),
				Height:          astikit.IntPtr(600),
				Width:           astikit.IntPtr(1200),
				WebPreferences:  &astilectron.WebPreferences{EnableRemoteModule: astikit.BoolPtr(true)},
			},
		}},
		MenuOptions: []*astilectron.MenuItemOptions{
			{
				SubMenu: []*astilectron.MenuItemOptions{
					{
						Accelerator: &astilectron.Accelerator{"CommandOrControl", "C"},
						Role:        astilectron.MenuItemRoleCopy,
					},
					{
						Accelerator: &astilectron.Accelerator{"CommandOrControl", "V"},
						Role:        astilectron.MenuItemRolePaste,
					},
					{Role: astilectron.MenuItemRoleClose},
				},
			},
			{
				Label: astikit.StrPtr("File"),
				SubMenu: []*astilectron.MenuItemOptions{
					{
						Label:       astikit.StrPtr("Rescan"),
						Accelerator: &astilectron.Accelerator{"CommandOrControl", "R"},
						OnClick: func(e astilectron.Event) (deleteListener bool) {
							g.state.window.SendMessage(Message{Name: "rescan", Payload: ""}, func(m *astilectron.EventMessage) {})
							return
						},
					},
					{
						Label: astikit.StrPtr("Hard rescan"),
						OnClick: func(e astilectron.Event) (deleteListener bool) {
							_ = localDbManager.ClearScanData()
							g.state.window.SendMessage(Message{Name: "rescan", Payload: ""}, func(m *astilectron.EventMessage) {})
							return
						},
					},
				},
			},
			{
				Label: astikit.StrPtr("Debug"),
				SubMenu: []*astilectron.MenuItemOptions{
					{
						Label:       astikit.StrPtr("Open DevTools"),
						Accelerator: &astilectron.Accelerator{"CommandOrControl", "D"},
						OnClick: func(e astilectron.Event) (deleteListener bool) {
							g.state.window.OpenDevTools()
							return
						},
					},
				},
			},
		},
	}); err != nil {
		g.sugarLogger.Error(fmt.Errorf("running bootstrap failed: %w", err))
		log.Fatal(err)
	}
}

func (g *GUI) handleMessage(m *astilectron.EventMessage) interface{} {
	var retValue string
	g.state.Lock()
	defer g.state.Unlock()
	msg := Message{}
	err := m.Unmarshal(&msg)

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
		err = g.saveSettings(msg.Payload)
		if err != nil {
			g.sugarLogger.Error(err)
			g.state.window.SendMessage(Message{Name: "error", Payload: err.Error()}, func(m *astilectron.EventMessage) {})
			return ""
		}
	case "missingGames":
		missingGames := g.getMissingGames()
		msg, _ := json.Marshal(missingGames)
		g.state.window.SendMessage(Message{Name: "missingGames", Payload: string(msg)}, func(m *astilectron.EventMessage) {})
	case "updateLocalLibrary":
		ignoreCache, _ := strconv.ParseBool(msg.Payload)
		localDB, err := g.buildLocalDB(g.localDbManager, ignoreCache)
		if err != nil {
			g.sugarLogger.Error(err)
			g.state.window.SendMessage(Message{Name: "error", Payload: err.Error()}, func(m *astilectron.EventMessage) {})
			return ""
		}
		response := LocalLibraryData{}
		libraryData := []LibraryTemplateData{}
		issues := []Pair{}
		for k, v := range localDB.TitlesMap {
			if v.BaseExist {
				version := ""
				name := ""
				if v.File.Metadata.Ncap != nil {
					version = v.File.Metadata.Ncap.DisplayVersion
					name = v.File.Metadata.Ncap.TitleName["AmericanEnglish"].Title
				}

				if v.Updates != nil && len(v.Updates) != 0 {
					if v.Updates[v.LatestUpdate].Metadata.Ncap != nil {
						version = v.Updates[v.LatestUpdate].Metadata.Ncap.DisplayVersion
					} else {
						version = ""
					}
				}
				if title, ok := g.state.switchDB.TitlesMap[k]; ok {
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
							Type:    getType(v),
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
							Type:    getType(v),
							TitleId: v.File.Metadata.TitleId,
							Path:    v.File.ExtendedInfo.FileName,
						})
				}

			} else {
				for _, update := range v.Updates {
					issues = append(issues, Pair{Key: filepath.Join(update.ExtendedInfo.BaseFolder, update.ExtendedInfo.FileName), Value: "base file is missing"})
				}
				for _, dlc := range v.Dlc {
					issues = append(issues, Pair{Key: filepath.Join(dlc.ExtendedInfo.BaseFolder, dlc.ExtendedInfo.FileName), Value: "base file is missing"})
				}
			}
		}
		for k, v := range localDB.Skipped {
			issues = append(issues, Pair{Key: filepath.Join(k.BaseFolder, k.FileName), Value: v.ReasonText})
		}

		response.LibraryData = libraryData
		response.NumFiles = localDB.NumFiles
		response.Issues = issues
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
		newUpdate, err := settings.CheckForUpdates()
		if err != nil {
			g.sugarLogger.Error(err)
			g.state.window.SendMessage(Message{Name: "error", Payload: err.Error()}, func(m *astilectron.EventMessage) {})
			return ""
		}
		retValue = strconv.FormatBool(newUpdate)
	}

	g.sugarLogger.Debugf("Server response [%v]", retValue)

	return retValue
}

func getType(gameFile *db.SwitchGameFiles) string {
	if gameFile.IsSplit {
		return "split"
	}
	if gameFile.MultiContent {
		return "multi-content"
	}
	ext := filepath.Ext(gameFile.File.ExtendedInfo.FileName)
	if len(ext) > 1 {
		return ext[1:]
	}
	return ""
}

func (g *GUI) saveSettings(settingsJson string) error {
	s := settings.AppSettings{}
	err := json.Unmarshal([]byte(settingsJson), &s)
	if err != nil {
		return err
	}
	settings.SaveSettings(&s, g.baseFolder)
	return nil
}

func (g *GUI) getMissingDLC() string {
	settingsObj := settings.ReadSettings(g.baseFolder)
	ignoreIds := map[string]struct{}{}
	for _, id := range settingsObj.IgnoreDLCTitleIds {
		ignoreIds[strings.ToLower(id)] = struct{}{}
	}
	missingDLC := process.ScanForMissingDLC(g.state.localDB.TitlesMap, g.state.switchDB.TitlesMap, ignoreIds)
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

func (g *GUI) buildSwitchDb() (*db.SwitchTitlesDB, error) {
	settingsObj := settings.ReadSettings(g.baseFolder)
	//1. load the titles JSON object
	g.UpdateProgress(1, 4, "Downloading titles.json")
	filename := filepath.Join(g.baseFolder, settings.TITLE_JSON_FILENAME)
	titleFile, titlesEtag, err := db.LoadAndUpdateFile(settings.TITLES_JSON_URL, filename, settingsObj.TitlesEtag)
	if err != nil {
		return nil, errors.New("failed to download switch titles [reason:" + err.Error() + "]")
	}
	settingsObj.TitlesEtag = titlesEtag

	g.UpdateProgress(2, 4, "Downloading versions.json")
	filename = filepath.Join(g.baseFolder, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsEtag, err := db.LoadAndUpdateFile(settings.VERSIONS_JSON_URL, filename, settingsObj.VersionsEtag)
	if err != nil {
		return nil, errors.New("failed to download switch updates [reason:" + err.Error() + "]")
	}
	settingsObj.VersionsEtag = versionsEtag

	settings.SaveSettings(settingsObj, g.baseFolder)

	g.UpdateProgress(3, 4, "Processing switch titles and updates ...")
	switchTitleDB, err := db.CreateSwitchTitleDB(titleFile, versionsFile)
	g.UpdateProgress(4, 4, "Finishing up...")
	return switchTitleDB, err
}

func (g *GUI) buildLocalDB(localDbManager *db.LocalSwitchDBManager, ignoreCache bool) (*db.LocalSwitchFilesDB, error) {
	folderToScan := settings.ReadSettings(g.baseFolder).Folder
	recursiveMode := settings.ReadSettings(g.baseFolder).ScanRecursively

	scanFolders := settings.ReadSettings(g.baseFolder).ScanFolders
	scanFolders = append(scanFolders, folderToScan)
	localDB, err := localDbManager.CreateLocalSwitchFilesDB(scanFolders, g, recursiveMode, ignoreCache)
	g.state.localDB = localDB
	return localDB, err
}

func (g *GUI) organizeLibrary() {
	folderToScan := settings.ReadSettings(g.baseFolder).Folder
	options := settings.ReadSettings(g.baseFolder).OrganizeOptions
	if !process.IsOptionsValid(options) {
		zap.S().Error("the organize options in settings.json are not valid, please check that the template contains file/folder name")
		g.state.window.SendMessage(Message{Name: "error", Payload: "the organize options in settings.json are not valid, please check that the template contains file/folder name"}, func(m *astilectron.EventMessage) {})
		return
	}
	process.OrganizeByFolders(folderToScan, g.state.localDB, g.state.switchDB, g)
	if settings.ReadSettings(g.baseFolder).OrganizeOptions.DeleteOldUpdateFiles {
		process.DeleteOldUpdates(g.baseFolder, g.state.localDB, g)
	}
}

func (g *GUI) UpdateProgress(curr int, total int, message string) {
	progressMessage := ProgressUpdate{curr, total, message}
	g.sugarLogger.Debugf("%v (%v/%v)", message, curr, total)
	msg, err := json.Marshal(progressMessage)
	if err != nil {
		g.sugarLogger.Error(err)
		return
	}

	g.state.window.SendMessage(Message{Name: "updateProgress", Payload: string(msg)}, func(m *astilectron.EventMessage) {})
}

func (g *GUI) getMissingGames() []SwitchTitle {
	var result []SwitchTitle
	for k, v := range g.state.switchDB.TitlesMap {
		if _, ok := g.state.localDB.TitlesMap[k]; ok {
			continue
		}
		if v.Attributes.Name == "" || v.Attributes.Id == "" {
			continue
		}
		result = append(result, SwitchTitle{
			TitleId:     v.Attributes.Id,
			Name:        v.Attributes.Name,
			Icon:        v.Attributes.BannerUrl,
			Region:      v.Attributes.Region,
			ReleaseDate: v.Attributes.ReleaseDate,
		})
	}
	return result

}
