package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	monitor *MonitorManager
	ui      *UISettingsStore
}

func NewApp() *App {
	return &App{
		monitor: NewMonitorManager(),
		ui:      NewUISettingsStore(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.monitor != nil {
		a.monitor.Start(ctx)
	}
}

func (a *App) shutdown(ctx context.Context) {
	if a.monitor != nil {
		a.monitor.Stop()
	}
}

func (a *App) domReady(ctx context.Context) {
	if a.ui != nil {
		_ = a.applyWindowMode(a.ui.Get().WindowMode)
	}
	runtime.WindowShow(ctx)
}

func (a *App) GetSnapshot() (*SystemSnapshot, error) {
	snapshot, err := collectSnapshot(a.ctx)
	if err != nil {
		return nil, err
	}
	if a.monitor != nil {
		snapshot.Monitor = a.monitor.Snapshot()
	}
	return snapshot, nil
}

func (a *App) GetInventory() (*InventorySnapshot, error) {
	return collectInventory(a.ctx)
}

func (a *App) GetProcessDetail(pid int32) (*ProcessDetail, error) {
	return collectProcessDetail(a.ctx, pid)
}

func (a *App) GetUISettings() (*UISettings, error) {
	settings := a.ui.Get()
	return &settings, nil
}

func (a *App) SetWindowMode(mode string) (*UISettings, error) {
	settings, err := a.ui.SetWindowMode(mode)
	if err != nil {
		return nil, err
	}
	if err := a.applyWindowMode(settings.WindowMode); err != nil {
		return nil, err
	}
	return &settings, nil
}

func (a *App) ExportCurrentBundle() (string, error) {
	return exportCurrentBundle(a.ctx)
}

func (a *App) ImportBundleDialog() (*AnalysisBundle, error) {
	return importBundleDialog(a.ctx)
}

func (a *App) KillProcess(pid int32) error {
	return killProcess(a.ctx, pid)
}

func (a *App) DisableAutorun(scope, location, name string) error {
	return disableAutorun(scope, location, name)
}

func (a *App) ControlService(name, action string) error {
	return controlService(name, action)
}

func (a *App) UpdateServiceStartType(name, startType string) error {
	return updateServiceStartType(name, startType)
}
