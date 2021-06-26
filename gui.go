package main

import (
	"fmt"
	"log"
	"time"

	"github.com/asticode/go-astikit"
	"github.com/asticode/go-astilectron"
	bootstrap "github.com/asticode/go-astilectron-bootstrap"
	"github.com/giwty/switch-library-manager/gui"
	"github.com/giwty/switch-library-manager/settings"
)

// Building and starting the GUI
func StartGUI() {
	g := gui.NewGUI(
		l,
		appSettings,
		workingFolder,
	)

	// Init the GUI
	g.Init()

	// Cleanup
	defer g.Defer()

	// Run bootstrap
	bootstrapErr := bootstrap.Run(bootstrap.Options{
		Asset:    Asset,
		AssetDir: AssetDir,
		AstilectronOptions: astilectron.Options{
			AppName:            fmt.Sprintf("Switch Library Manager (%s)", settings.SLM_VERSION),
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
				g.Window = w
				g.Window.OnMessage(g.HandleMessage)
				//g.state.window.OpenDevTools()
			},
			Options: &astilectron.WindowOptions{
				BackgroundColor: astikit.StrPtr("#333"),
				Center:          astikit.BoolPtr(true),
				Height:          astikit.IntPtr(600),
				Width:           astikit.IntPtr(1200),
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
							g.Send(gui.GUI_MESSAGE_RESCAN, "")
							return
						},
					},
					{
						Label: astikit.StrPtr("Hard rescan"),
						OnClick: func(e astilectron.Event) (deleteListener bool) {
							g.Clear()
							g.Send(gui.GUI_MESSAGE_RESCAN, "")
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
							g.Window.OpenDevTools()
							return
						},
					},
				},
			},
		},
	})

	// If bootstrap failed, bail
	if bootstrapErr != nil {
		l.Fatalf("running bootstrap failed: %w", bootstrapErr)
	}
}
