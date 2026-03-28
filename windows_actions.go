//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func killProcess(ctx context.Context, pid int32) error {
	proc, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return err
	}
	return proc.KillWithContext(ctx)
}

func controlService(name, action string) error {
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(name)
	if err != nil {
		return err
	}
	defer service.Close()

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "start":
		return service.Start()
	case "stop":
		_, err = service.Control(svc.Stop)
		return err
	case "pause":
		_, err = service.Control(svc.Pause)
		return err
	case "continue":
		_, err = service.Control(svc.Continue)
		return err
	case "restart":
		_, err = service.Control(svc.Stop)
		if err != nil {
			return err
		}
		time.Sleep(1200 * time.Millisecond)
		return service.Start()
	default:
		return fmt.Errorf("unsupported service action: %s", action)
	}
}

func updateServiceStartType(name, startType string) error {
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(name)
	if err != nil {
		return err
	}
	defer service.Close()

	config, err := service.Config()
	if err != nil {
		return err
	}

	config.StartType = parseServiceStartType(startType)
	return service.UpdateConfig(config)
}

func disableAutorun(scope, location, name string) error {
	if strings.HasPrefix(location, "HK") {
		return disableRegistryAutorun(scope, location, name)
	}
	return disableStartupFolderAutorun(location, name)
}

func disableRegistryAutorun(scope, location, name string) error {
	root, path, access, err := resolveAutorunRegistryPath(scope, location)
	if err != nil {
		return err
	}

	key, err := registry.OpenKey(root, path, access|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	value, _, err := key.GetStringValue(name)
	if err != nil {
		return err
	}

	if err := backupDisabledAutorun(scope, location, name, value); err != nil {
		return err
	}

	return key.DeleteValue(name)
}

func disableStartupFolderAutorun(location, name string) error {
	targetPath := filepath.Join(resolveStartupFolder(location), name)
	if _, err := os.Stat(targetPath); err != nil {
		targetPath = location
	}

	disabledPath := targetPath + ".disabled"
	return os.Rename(targetPath, disabledPath)
}

func resolveAutorunRegistryPath(scope, location string) (registry.Key, string, uint32, error) {
	switch location {
	case `HKLM\Software\Microsoft\Windows\CurrentVersion\Run`:
		return registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.WOW64_64KEY, nil
	case `HKLM\Software\Microsoft\Windows\CurrentVersion\RunOnce`:
		return registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\RunOnce`, registry.WOW64_64KEY, nil
	case `HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`:
		return registry.LOCAL_MACHINE, `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, registry.WOW64_32KEY, nil
	case `HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\RunOnce`:
		return registry.LOCAL_MACHINE, `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\RunOnce`, registry.WOW64_32KEY, nil
	case `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`:
		return registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, 0, nil
	case `HKCU\Software\Microsoft\Windows\CurrentVersion\RunOnce`:
		return registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\RunOnce`, 0, nil
	default:
		return 0, "", 0, fmt.Errorf("unsupported autorun location: %s", location)
	}
}

func backupDisabledAutorun(scope, location, name, value string) error {
	backupRoot := registry.CURRENT_USER
	backupPath := filepath.Join(`Software\TaskEz\DisabledAutoruns`, sanitizeRegistryName(scope+"_"+name))
	key, _, err := registry.CreateKey(backupRoot, backupPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if err := key.SetStringValue("Scope", scope); err != nil {
		return err
	}
	if err := key.SetStringValue("Location", location); err != nil {
		return err
	}
	if err := key.SetStringValue("Name", name); err != nil {
		return err
	}
	if err := key.SetStringValue("Command", value); err != nil {
		return err
	}
	return key.SetStringValue("DisabledAt", time.Now().Format(time.RFC3339))
}

func sanitizeRegistryName(value string) string {
	replacer := strings.NewReplacer(`\`, "_", "/", "_", " ", "_", ":", "_", ".", "_")
	return replacer.Replace(value)
}

func resolveStartupFolder(location string) string {
	switch location {
	case "Startup Folder (All Users)":
		return filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	case "Startup Folder (Current User)":
		return filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	default:
		return location
	}
}

func parseServiceStartType(value string) uint32 {
	switch strings.TrimSpace(value) {
	case "自动":
		return windows.SERVICE_AUTO_START
	case "手动":
		return windows.SERVICE_DEMAND_START
	case "禁用":
		return windows.SERVICE_DISABLED
	case "启动":
		return windows.SERVICE_BOOT_START
	case "系统":
		return windows.SERVICE_SYSTEM_START
	default:
		return windows.SERVICE_DEMAND_START
	}
}
