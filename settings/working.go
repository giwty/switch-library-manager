package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func GetWorkingFolder() (string, string, error) {
	// Detect executable own dir as working folder
	exePath, exeErr := os.Executable()
	if exeErr != nil {
		return "", "", exeErr
	}

	workingFolder := filepath.Dir(exePath)

	// Adjust for MacOS
	if runtime.GOOS == "darwin" {
		if strings.Contains(workingFolder, ".app") {
			appIndex := strings.Index(workingFolder, ".app")
			sepIndex := strings.LastIndex(workingFolder[:appIndex], string(os.PathSeparator))
			workingFolder = workingFolder[:sepIndex]
		}
	}

	return exePath, workingFolder, nil
}
