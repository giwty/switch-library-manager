package main

import (
	"fmt"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/giwty/switch-library-manager/ui"
	"go.uber.org/zap"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
)

func main() {

	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("failed to get executable directory, please ensure app has sufficient permissions. aborting")
		return
	}

	workingFolder, err := os.Getwd()
	if err != nil {
		fmt.Println("failed to get working directory, please ensure app has sufficient permissions. aborting")
	}

	webResourcesPath := filepath.Join(workingFolder, "web")
	if _, err := os.Stat(webResourcesPath); err != nil {
		workingFolder = filepath.Dir(exePath)
		webResourcesPath = filepath.Join(workingFolder, "web")
		if _, err := os.Stat(webResourcesPath); err != nil {
			fmt.Println("Missing web folder, please re-download latest release, and extract all files. aborting", err)
			return
		}
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
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	logPath := filepath.Join(workingFolder, "slm.log")
	// delete old file
	os.Remove(logPath)

	if runtime.GOOS == "windows" {
		zap.RegisterSink("winfile", func(u *url.URL) (zap.Sink, error) {
			// Remove leading slash left by url.Parse()
			return os.OpenFile(u.Path[1:], os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		})
		logPath = "winfile:///" + logPath
	}

	config.OutputPaths = []string{logPath}
	config.ErrorOutputPaths = []string{logPath}
	logger, err := config.Build()
	if err != nil {
		fmt.Printf("failed to create logger - %v", err)
		panic(1)
	}
	zap.ReplaceGlobals(logger)
	return logger
}
