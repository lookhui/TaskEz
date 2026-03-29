//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type win32ServiceRecord struct {
	Name        string
	DisplayName string
	State       string
	StartMode   string
}

type win32SystemDriverRecord struct {
	Name        string
	DisplayName string
	State       string
	StartMode   string
	PathName    string
	ServiceType string
}

type scheduledTaskRecord struct {
	TaskName    string
	TaskPath    string
	State       uint32
	Author      string
	Description string
	URI         string
}

func collectServices() ([]ServiceInfo, error) {
	var records []win32ServiceRecord
	query := wmi.CreateQuery(&records, "", "Win32_Service")
	if err := wmi.Query(query, &records); err != nil {
		return nil, err
	}

	services := make([]ServiceInfo, 0, len(records))
	for _, record := range records {
		services = append(services, ServiceInfo{
			Name:        strings.TrimSpace(record.Name),
			DisplayName: strings.TrimSpace(record.DisplayName),
			State:       strings.TrimSpace(record.State),
			StartType:   translateServiceStartMode(record.StartMode),
		})
	}

	sort.Slice(services, func(i, j int) bool {
		if services[i].State == services[j].State {
			return strings.ToLower(services[i].DisplayName) < strings.ToLower(services[j].DisplayName)
		}
		return services[i].State < services[j].State
	})

	return services, nil
}

func collectAutoruns() ([]AutorunEntry, error) {
	entries := []AutorunEntry{}
	registryPaths := []struct {
		root     registry.Key
		scope    string
		location string
		path     string
		view     uint32
	}{
		{registry.LOCAL_MACHINE, "Machine", `HKLM\Software\Microsoft\Windows\CurrentVersion\Run`, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.WOW64_64KEY},
		{registry.LOCAL_MACHINE, "Machine", `HKLM\Software\Microsoft\Windows\CurrentVersion\RunOnce`, `Software\Microsoft\Windows\CurrentVersion\RunOnce`, registry.WOW64_64KEY},
		{registry.LOCAL_MACHINE, "Machine", `HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, registry.WOW64_32KEY},
		{registry.LOCAL_MACHINE, "Machine", `HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\RunOnce`, `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\RunOnce`, registry.WOW64_32KEY},
		{registry.CURRENT_USER, "Current User", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, `Software\Microsoft\Windows\CurrentVersion\Run`, 0},
		{registry.CURRENT_USER, "Current User", `HKCU\Software\Microsoft\Windows\CurrentVersion\RunOnce`, `Software\Microsoft\Windows\CurrentVersion\RunOnce`, 0},
	}

	for _, item := range registryPaths {
		entries = append(entries, readRegistryAutoruns(item.root, item.scope, item.location, item.path, item.view)...)
	}

	startupFolders := []struct {
		scope    string
		location string
		path     string
	}{
		{
			scope:    "Machine",
			location: "Startup Folder (All Users)",
			path:     filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup"),
		},
		{
			scope:    "Current User",
			location: "Startup Folder (Current User)",
			path:     filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup"),
		},
	}

	for _, folder := range startupFolders {
		entries = append(entries, readStartupFolder(folder.scope, folder.location, folder.path)...)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Scope == entries[j].Scope {
			return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		}
		return entries[i].Scope < entries[j].Scope
	})

	return entries, nil
}

func collectDrivers(ctx context.Context) ([]DriverInfo, error) {
	_ = ctx

	var records []win32SystemDriverRecord
	query := wmi.CreateQuery(&records, "", "Win32_SystemDriver")
	if err := wmi.Query(query, &records); err != nil {
		return nil, err
	}

	drivers := make([]DriverInfo, 0, len(records))
	for _, record := range records {
		drivers = append(drivers, DriverInfo{
			Name:        strings.TrimSpace(record.Name),
			DisplayName: strings.TrimSpace(record.DisplayName),
			State:       strings.TrimSpace(record.State),
			StartMode:   translateDriverStartMode(record.StartMode),
			Path:        strings.TrimSpace(record.PathName),
			ServiceType: strings.TrimSpace(record.ServiceType),
		})
	}

	sort.Slice(drivers, func(i, j int) bool {
		if drivers[i].State == drivers[j].State {
			return strings.ToLower(drivers[i].DisplayName) < strings.ToLower(drivers[j].DisplayName)
		}
		return drivers[i].State < drivers[j].State
	})

	return drivers, nil
}

func collectScheduledTasks(ctx context.Context) ([]ScheduledTaskInfo, error) {
	_ = ctx

	var records []scheduledTaskRecord
	query := wmi.CreateQuery(&records, "", "MSFT_ScheduledTask")
	if err := wmi.QueryNamespace(query, &records, `root\Microsoft\Windows\TaskScheduler`); err != nil {
		return nil, err
	}

	tasks := make([]ScheduledTaskInfo, 0, len(records))
	for _, record := range records {
		command := strings.TrimSpace(record.URI)
		if command == "" {
			command = strings.TrimRight(record.TaskPath, `\`) + `\` + record.TaskName
		}

		tasks = append(tasks, ScheduledTaskInfo{
			Name:        strings.TrimSpace(record.TaskName),
			Path:        strings.TrimSpace(record.TaskPath),
			State:       scheduledTaskStateLabel(record.State),
			Author:      strings.TrimSpace(record.Author),
			Description: strings.TrimSpace(record.Description),
			Command:     command,
		})
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Path == tasks[j].Path {
			return strings.ToLower(tasks[i].Name) < strings.ToLower(tasks[j].Name)
		}
		return tasks[i].Path < tasks[j].Path
	})

	return tasks, nil
}

func collectProcessDetail(ctx context.Context, pid int32) (*ProcessDetail, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	proc, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return nil, err
	}

	info, ok := collectProcessInfo(ctx, proc)
	if !ok {
		return nil, fmt.Errorf("无法读取进程 %d 的基本信息", pid)
	}

	allProcesses, _ := collectProcesses(ctx)
	nameCache := make(map[int32]string, len(allProcesses))
	for _, item := range allProcesses {
		nameCache[item.PID] = item.Name
		if item.PID == pid {
			info.ParentName = item.ParentName
		}
	}
	if info.ParentName == "" {
		info.ParentName = processNameFromPID(ctx, info.ParentPID, nameCache)
	}

	children := make([]ProcessRef, 0)
	for _, item := range allProcesses {
		if item.ParentPID == pid {
			children = append(children, ProcessRef{PID: item.PID, Name: item.Name})
		}
	}
	sort.Slice(children, func(i, j int) bool { return children[i].PID < children[j].PID })

	parentChain := buildParentChain(ctx, info.ParentPID, nameCache)

	detail := &ProcessDetail{
		Process:     info,
		ParentChain: parentChain,
		Children:    children,
		Threads:     []ThreadInfo{},
		Modules:     []ModuleInfo{},
		Warnings:    []string{},
	}

	threads, err := collectProcessThreads(uint32(pid))
	if err != nil {
		detail.Warnings = append(detail.Warnings, fmt.Sprintf("线程读取失败: %v", err))
	} else {
		detail.Threads = threads
	}

	modules, err := collectProcessModules(uint32(pid))
	if err != nil {
		detail.Warnings = append(detail.Warnings, fmt.Sprintf("模块读取失败: %v", err))
	} else {
		detail.Modules = modules
	}

	return detail, nil
}

func buildParentChain(ctx context.Context, parentPID int32, nameCache map[int32]string) []ProcessRef {
	chain := make([]ProcessRef, 0, 8)
	current := parentPID
	for depth := 0; current > 0 && depth < 24; depth++ {
		name := processNameFromPID(ctx, current, nameCache)
		chain = append(chain, ProcessRef{PID: current, Name: name})
		proc, err := process.NewProcessWithContext(ctx, current)
		if err != nil {
			break
		}
		next, err := proc.PpidWithContext(ctx)
		if err != nil || next == current {
			break
		}
		current = next
	}

	for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
		chain[left], chain[right] = chain[right], chain[left]
	}
	return chain
}

func collectProcessThreads(pid uint32) ([]ThreadInfo, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	if err := windows.Thread32First(snapshot, &entry); err != nil {
		return nil, err
	}

	threads := make([]ThreadInfo, 0, 32)
	for {
		if entry.OwnerProcessID == pid {
			threads = append(threads, ThreadInfo{
				ThreadID:     entry.ThreadID,
				OwnerPID:     int32(entry.OwnerProcessID),
				BasePriority: entry.BasePri,
			})
		}

		err = windows.Thread32Next(snapshot, &entry)
		if err == nil {
			continue
		}
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			break
		}
		return nil, err
	}

	sort.Slice(threads, func(i, j int) bool {
		return threads[i].ThreadID < threads[j].ThreadID
	})

	return threads, nil
}

func collectProcessModules(pid uint32) ([]ModuleInfo, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPMODULE|windows.TH32CS_SNAPMODULE32, pid)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ModuleEntry32{Size: uint32(unsafe.Sizeof(windows.ModuleEntry32{}))}
	if err := windows.Module32First(snapshot, &entry); err != nil {
		return nil, err
	}

	modules := make([]ModuleInfo, 0, 64)
	for {
		modules = append(modules, ModuleInfo{
			Name:        windows.UTF16ToString(entry.Module[:]),
			Path:        windows.UTF16ToString(entry.ExePath[:]),
			BaseAddress: fmt.Sprintf("0x%X", entry.ModBaseAddr),
			SizeKB:      float64(entry.ModBaseSize) / 1024,
		})

		err = windows.Module32Next(snapshot, &entry)
		if err == nil {
			continue
		}
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			break
		}
		return nil, err
	}

	sort.Slice(modules, func(i, j int) bool {
		return strings.ToLower(modules[i].Name) < strings.ToLower(modules[j].Name)
	})

	return modules, nil
}

func scheduledTaskStateLabel(value uint32) string {
	switch value {
	case 0:
		return "未知"
	case 1:
		return "禁用"
	case 2:
		return "排队"
	case 3:
		return "就绪"
	case 4:
		return "运行中"
	default:
		return fmt.Sprintf("状态 %d", value)
	}
}

func translateDriverStartMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto", "automatic":
		return "自动"
	case "manual":
		return "手动"
	case "disabled":
		return "禁用"
	case "boot":
		return "启动"
	case "system":
		return "系统"
	default:
		return value
	}
}

func translateServiceStartMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto", "automatic":
		return "自动"
	case "manual":
		return "手动"
	case "disabled":
		return "禁用"
	case "boot":
		return "启动"
	case "system":
		return "系统"
	case "delayed auto":
		return "自动(延迟)"
	default:
		return value
	}
}

func readRegistryAutoruns(root registry.Key, scope, location, path string, view uint32) []AutorunEntry {
	access := uint32(registry.QUERY_VALUE)
	if view != 0 {
		access |= view
	}

	key, err := registry.OpenKey(root, path, access)
	if err != nil {
		return nil
	}
	defer key.Close()

	valueNames, err := key.ReadValueNames(-1)
	if err != nil {
		return nil
	}

	entries := make([]AutorunEntry, 0, len(valueNames))
	for _, name := range valueNames {
		if strings.TrimSpace(name) == "" {
			continue
		}

		value, valueType, err := key.GetStringValue(name)
		if err != nil {
			continue
		}

		if valueType == registry.EXPAND_SZ {
			expanded, expandErr := registry.ExpandString(value)
			if expandErr == nil && strings.TrimSpace(expanded) != "" {
				value = expanded
			}
		}

		entries = append(entries, AutorunEntry{
			Scope:    scope,
			Location: location,
			Name:     name,
			Command:  value,
		})
	}

	return entries
}

func readStartupFolder(scope, location, path string) []AutorunEntry {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	items, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	entries := make([]AutorunEntry, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if item.IsDir() && strings.EqualFold(name, "TaskEzDisabled") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(name), ".disabled") {
			continue
		}
		entries = append(entries, AutorunEntry{
			Scope:    scope,
			Location: location,
			Name:     name,
			Command:  filepath.Join(path, name),
		})
	}

	return entries
}
