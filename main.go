package main

import (
	"fmt"
	"github.com/giwty/switch-backup-manager/settings"
	"github.com/giwty/switch-backup-manager/ui"
	"go.uber.org/zap"
	"os"
	"path"
)

func main() {

	exePath, err := os.Executable()
	if err != nil {
		fmt.Print("failed to get executable directory, please ensure app has sufficient permissions. aborting")
		return
	}

	workingFolder, err := os.Getwd()
	if err != nil {
		fmt.Print("failed to get working directory, please ensure app has sufficient permissions. aborting")
	}

	appSettings := settings.ReadSettings(workingFolder)

	logger := createLogger(workingFolder, appSettings.Debug)

	defer logger.Sync() // flushes buffer, if any
	sugar := logger.Sugar()

	sugar.Info("[SLM starts]")
	sugar.Infof("[Executable: %v]", exePath)
	sugar.Infof("[Working directory: %v]", workingFolder)

	if appSettings.GUI {
		ui.CreateGUI(workingFolder, sugar).Start()
	} else {
		ui.CreateConsole(workingFolder, sugar).Start()
	}

}

func createLogger(workingFolder string, debug bool) *zap.Logger {
	var config zap.Config
	if debug {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}
	logPath := path.Join(workingFolder, "slm.log")

	// delete old file
	os.Remove(logPath)

	config.OutputPaths = []string{logPath}
	config.ErrorOutputPaths = []string{logPath}
	logger, _ := config.Build()
	zap.ReplaceGlobals(logger)
	return logger
}
