package app

import (
	"context"
	"fmt"
	"io/fs"
	"sync"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

type State struct {
	mu sync.Mutex

	switchDB           *db.SwitchTitlesDB
	localDB            *db.LocalSwitchFilesDB
	organizationIssues []Pair
}

type App struct {
	state               State
	exportMu            sync.Mutex
	baseFolder          string
	restart             func() error
	quit                func(context.Context)
	restarting          bool
	localDbManager      *db.LocalSwitchDBManager
	sugarLogger         *zap.SugaredLogger
	migrationInfo       *settings.MigrationInfo
	rememberWindowState bool
	windowState         *settings.WindowState
	ctx                 context.Context
}

// Start prepares the application dependencies and starts the Wails event loop.
func Start(baseFolder string, sugarLogger *zap.SugaredLogger, assets fs.FS) error {
	return StartWithOptions(baseFolder, sugarLogger, assets, StartupOptions{})
}

// StartWithMigration prepares the application and optionally shows the older
// settings migration notice after Wails has initialized.
func StartWithMigration(baseFolder string, sugarLogger *zap.SugaredLogger, assets fs.FS, migrationInfo *settings.MigrationInfo) error {
	return StartWithOptions(baseFolder, sugarLogger, assets, StartupOptions{MigrationInfo: migrationInfo})
}

// StartupOptions contains optional GUI lifecycle behavior.
type StartupOptions struct {
	MigrationInfo *settings.MigrationInfo
	Restart       func() error
}

// StartWithOptions prepares the application and starts Wails with the supplied lifecycle options.
func StartWithOptions(baseFolder string, sugarLogger *zap.SugaredLogger, assets fs.FS, startupOptions StartupOptions) error {
	appSettings, err := settings.ReadSettings(baseFolder)
	if err != nil {
		return fmt.Errorf("load GUI settings: %w", err)
	}

	localDbManager, err := db.NewLocalSwitchDBManager(baseFolder)
	if err != nil {
		sugarLogger.Error("failed to create local files db", err)
		return err
	}
	defer localDbManager.Close()

	if _, err := settings.InitSwitchKeys(baseFolder); err != nil {
		sugarLogger.Warnf("failed to initialize Switch keys (deep scan disabled): %v", err)
	}
	application := &App{
		baseFolder:          baseFolder,
		restart:             startupOptions.Restart,
		quit:                wailsruntime.Quit,
		localDbManager:      localDbManager,
		sugarLogger:         sugarLogger,
		migrationInfo:       startupOptions.MigrationInfo,
		rememberWindowState: appSettings.GUI.RememberWindowState,
	}
	application.windowState = loadWindowState(baseFolder, application.rememberWindowState, sugarLogger)
	return application.run(assets)
}

func (a *App) run(assets fs.FS) error {
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		return fmt.Errorf("prepare frontend assets: %w", err)
	}
	width, height := windowSizeForStart(a.rememberWindowState, a.windowState)

	if err := wails.Run(&options.App{
		Title:            "Switch Library Manager (" + settings.SLM_VERSION + ")",
		Width:            width,
		Height:           height,
		AlwaysOnTop:      true,
		BackgroundColour: options.NewRGB(51, 51, 51),
		AssetServer:      &assetserver.Options{Assets: frontend},
		OnStartup:        a.startup,
		OnShutdown:       a.shutdown,
		OnBeforeClose:    a.beforeClose,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.trembon.switch-library-manager",
		},
		Bind: []interface{}{a},
	}); err != nil {
		return fmt.Errorf("running Wails: %w", err)
	}
	return nil
}

// Restart launches a replacement GUI process and closes this instance only
// after the replacement process has started successfully.
func (a *App) Restart() error {
	a.state.mu.Lock()
	if a.restarting {
		a.state.mu.Unlock()
		return fmt.Errorf("application restart is already in progress")
	}
	if a.restart == nil {
		a.state.mu.Unlock()
		return fmt.Errorf("application restart is unavailable")
	}
	if a.ctx == nil {
		a.state.mu.Unlock()
		return fmt.Errorf("application is not ready to restart")
	}
	a.restarting = true
	restart := a.restart
	quit := a.quit
	ctx := a.ctx
	a.state.mu.Unlock()

	if err := restart(); err != nil {
		a.state.mu.Lock()
		a.restarting = false
		a.state.mu.Unlock()
		return fmt.Errorf("launch replacement application: %w", err)
	}
	if quit == nil {
		quit = wailsruntime.Quit
	}
	quit(ctx)
	return nil
}
