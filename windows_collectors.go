//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"
	"unsafe"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

func collectServices() ([]ServiceInfo, error) {
	manager, err := mgr.Connect()
	if err != nil {
		return nil, err
	}
	defer manager.Disconnect()

	serviceNames, err := manager.ListServices()
	if err != nil {
		return nil, err
	}

	services := make([]ServiceInfo, 0, len(serviceNames))
	for _, name := range serviceNames {
		service, err := manager.OpenService(name)
		if err != nil {
			continue
		}

		config, configErr := service.Config()
		status, statusErr := service.Query()
		service.Close()

		if configErr != nil || statusErr != nil {
			continue
		}

		services = append(services, ServiceInfo{
			Name:        name,
			DisplayName: config.DisplayName,
			State:       serviceState(status.State),
			StartType:   serviceStartType(config.StartType),
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
	rows, err := runCSVRowsNoHeader(ctx, "driverquery.exe", "/fo", "csv", "/v", "/nh")
	if err != nil {
		return nil, err
	}

	drivers := make([]DriverInfo, 0, len(rows))
	for _, row := range rows {
		if len(row) < 14 {
			continue
		}

		drivers = append(drivers, DriverInfo{
			Name:        strings.TrimSpace(row[0]),
			DisplayName: strings.TrimSpace(row[1]),
			State:       strings.TrimSpace(row[5]),
			StartMode:   translateDriverStartMode(strings.TrimSpace(row[4])),
			Path:        strings.TrimSpace(row[13]),
			ServiceType: strings.TrimSpace(row[3]),
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
	rows, err := runCSVRowsNoHeader(ctx, "schtasks.exe", "/query", "/fo", "csv", "/v", "/nh")
	if err != nil {
		return nil, err
	}

	tasks := make([]ScheduledTaskInfo, 0, len(rows))
	for _, row := range rows {
		if len(row) < 11 {
			continue
		}

		path, name := splitTaskName(strings.TrimSpace(row[1]))
		tasks = append(tasks, ScheduledTaskInfo{
			Name:        name,
			Path:        path,
			State:       strings.TrimSpace(row[3]),
			Author:      strings.TrimSpace(row[7]),
			Description: strings.TrimSpace(row[10]),
			Command:     strings.TrimSpace(row[8]),
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

func runCSVRowsNoHeader(ctx context.Context, command string, args ...string) ([][]string, error) {
	escapedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		escapedArgs = append(escapedArgs, syscall.EscapeArg(arg))
	}
	commandLine := fmt.Sprintf(
		"chcp 65001>nul & %s %s",
		syscall.EscapeArg(command),
		strings.Join(escapedArgs, " "),
	)

	cmd := exec.CommandContext(ctx, "cmd.exe", "/d", "/c", commandLine)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%s", message)
	}

	decoded, err := decodeCommandOutput(stdout.Bytes())
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(strings.NewReader(decoded))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return records, nil
}

func decodeCommandOutput(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	if utf8.Valid(data) {
		return string(data), nil
	}

	enc := windowsCodePageEncoding(windows.GetACP())
	if enc == nil {
		return string(data), nil
	}

	decoded, _, err := transform.String(enc.NewDecoder(), string(data))
	if err != nil {
		return "", err
	}
	return decoded, nil
}

func windowsCodePageEncoding(codePage uint32) encoding.Encoding {
	switch codePage {
	case 936:
		return simplifiedchinese.GBK
	case 950:
		return traditionalchinese.Big5
	case 932:
		return japanese.ShiftJIS
	case 949:
		return korean.EUCKR
	case 1252:
		return charmap.Windows1252
	default:
		return nil
	}
}

func splitTaskName(value string) (string, string) {
	if value == "" {
		return "\\", ""
	}
	lastIndex := strings.LastIndex(value, `\`)
	if lastIndex <= 0 {
		return "\\", strings.TrimPrefix(value, `\`)
	}
	return value[:lastIndex+1], value[lastIndex+1:]
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

func serviceState(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "Stopped"
	case svc.StartPending:
		return "Start Pending"
	case svc.StopPending:
		return "Stop Pending"
	case svc.Running:
		return "Running"
	case svc.ContinuePending:
		return "Continue Pending"
	case svc.PausePending:
		return "Pause Pending"
	case svc.Paused:
		return "Paused"
	default:
		return fmt.Sprintf("State %d", state)
	}
}

func serviceStartType(value uint32) string {
	switch value {
	case windows.SERVICE_BOOT_START:
		return "启动"
	case windows.SERVICE_SYSTEM_START:
		return "系统"
	case windows.SERVICE_AUTO_START:
		return "自动"
	case windows.SERVICE_DEMAND_START:
		return "手动"
	case windows.SERVICE_DISABLED:
		return "禁用"
	default:
		return fmt.Sprintf("类型 %d", value)
	}
}
