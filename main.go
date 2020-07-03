package main

import (
	"github.com/giwty/switch-backup-manager/settings"
	"github.com/giwty/switch-backup-manager/ui"
)

func main() {

	settingsObj := settings.ReadSettings()

	if settingsObj.GUI {
		ui.CreateGUI().Start()
	} else {
		ui.ConsoleGUI()
	}

}
