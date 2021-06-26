package main

import (
	"fmt"

	"github.com/giwty/switch-library-manager/logger"
	"github.com/giwty/switch-library-manager/settings"
	"go.uber.org/zap"
)

// App global vars
var (
	workingFolder string
	appSettings   *settings.AppSettings
	l             *zap.SugaredLogger
)

// Main
func main() {
	// Locate own working dir
	var workingFolderErr error
	var exePath string
	exePath, workingFolder, workingFolderErr = settings.GetWorkingFolder()
	if workingFolderErr != nil {
		fmt.Println("Failed to get executable directory, please ensure app has sufficient permissions. Aborting.")
		return
	}

	// Load the app settings
	appSettings = settings.NewAppSettings(workingFolder)

	// Create a new global logger
	l = logger.GetSugar(workingFolder, appSettings.Debug)
	defer logger.Defer() // flushes buffer, if any

	l.Info("[SLM starts]")
	l.Infof("[Executable: %v]", exePath)
	l.Infof("[Working directory: %v]", workingFolder)

	// Force console if nothing in the asset dir
	files, err := AssetDir(workingFolder)
	if files == nil && err == nil {
		appSettings.GUI = false
	}

	if appSettings.GUI {
		StartGUI()
		// } else {
		// 	CreateConsole(workingFolder, sugar).Start()
	}

}
