package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

func collectSnapshot(ctx context.Context) (*SystemSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	snapshot := &SystemSnapshot{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Processes:   []ProcessInfo{},
		ProcessTree: []ProcessTreeNode{},
		Services:    []ServiceInfo{},
		Connections: []ConnectionInfo{},
		Autoruns:    []AutorunEntry{},
		Highlights:  []Highlight{},
		Warnings:    []string{},
	}

	var (
		overview    Overview
		processes   = []ProcessInfo{}
		services    = []ServiceInfo{}
		connections = []ConnectionInfo{}
		autoruns    = []AutorunEntry{}
		warnings    []string
		mu          sync.Mutex
		wg          sync.WaitGroup
	)

	addWarning := func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}

	wg.Add(5)

	go func() {
		defer wg.Done()
		value, err := collectOverview(ctx)
		if err != nil {
			addWarning("总览采集失败: %v", err)
			return
		}
		overview = value
	}()

	go func() {
		defer wg.Done()
		value, err := collectProcesses(ctx)
		if err != nil {
			addWarning("进程采集失败: %v", err)
			return
		}
		processes = value
	}()

	go func() {
		defer wg.Done()
		value, err := collectServices()
		if err != nil {
			addWarning("服务采集失败: %v", err)
			return
		}
		services = value
	}()

	go func() {
		defer wg.Done()
		value, err := collectConnections(ctx)
		if err != nil {
			addWarning("网络连接采集失败: %v", err)
			return
		}
		connections = value
	}()

	go func() {
		defer wg.Done()
		value, err := collectAutoruns()
		if err != nil {
			addWarning("启动项采集失败: %v", err)
			return
		}
		autoruns = value
	}()

	wg.Wait()

	overview.ProcessCount = len(processes)
	overview.ServiceCount = len(services)
	overview.ConnectionCount = len(connections)
	overview.AutorunCount = len(autoruns)

	snapshot.Overview = overview
	snapshot.Processes = cloneProcessSlice(processes)
	snapshot.ProcessTree = buildProcessTree(processes)
	snapshot.Services = cloneServiceSlice(services)
	snapshot.Connections = cloneConnectionSlice(connections)
	snapshot.Autoruns = cloneAutorunSlice(autoruns)
	snapshot.Highlights = buildHighlights(overview, processes, services, connections, autoruns)
	snapshot.Warnings = append([]string{}, warnings...)

	return snapshot, nil
}

func collectInventory(ctx context.Context) (*InventorySnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	inventory := &InventorySnapshot{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Drivers:     []DriverInfo{},
		Tasks:       []ScheduledTaskInfo{},
		Warnings:    []string{},
	}

	var (
		drivers  = []DriverInfo{}
		tasks    = []ScheduledTaskInfo{}
		warnings []string
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	addWarning := func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}

	wg.Add(2)

	go func() {
		defer wg.Done()
		value, err := collectDrivers(ctx)
		if err != nil {
			addWarning("驱动采集失败: %v", err)
			return
		}
		drivers = value
	}()

	go func() {
		defer wg.Done()
		value, err := collectScheduledTasks(ctx)
		if err != nil {
			addWarning("计划任务采集失败: %v", err)
			return
		}
		tasks = value
	}()

	wg.Wait()

	inventory.Drivers = append(inventory.Drivers, drivers...)
	inventory.Tasks = append(inventory.Tasks, tasks...)
	inventory.Warnings = append(inventory.Warnings, warnings...)

	return inventory, nil
}

func collectOverview(ctx context.Context) (Overview, error) {
	hostInfo, err := host.InfoWithContext(ctx)
	if err != nil {
		return Overview{}, err
	}

	cpuInfo, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return Overview{}, err
	}

	cpuPercent, err := cpu.PercentWithContext(ctx, 250*time.Millisecond, false)
	if err != nil {
		return Overview{}, err
	}

	vmem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return Overview{}, err
	}

	swap, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		swap = &mem.SwapMemoryStat{}
	}

	ioCounters, err := gnet.IOCountersWithContext(ctx, false)
	if err != nil {
		ioCounters = nil
	}

	disks, err := collectDisks(ctx)
	if err != nil {
		disks = []DiskInfo{}
	}

	logicalCores, _ := cpu.CountsWithContext(ctx, true)

	overview := Overview{
		Hostname:        hostInfo.Hostname,
		Platform:        hostInfo.Platform,
		PlatformVersion: hostInfo.PlatformVersion,
		KernelVersion:   hostInfo.KernelVersion,
		Architecture:    hostInfo.KernelArch,
		BootTime:        time.Unix(int64(hostInfo.BootTime), 0).Format("2006-01-02 15:04:05"),
		Uptime:          formatDuration(time.Duration(hostInfo.Uptime) * time.Second),
		CPUModel:        firstCPUModel(cpuInfo),
		LogicalCores:    logicalCores,
		CPULoad:         firstFloat(cpuPercent),
		MemoryTotalGB:   bytesToGB(vmem.Total),
		MemoryUsedGB:    bytesToGB(vmem.Used),
		MemoryUsedPct:   vmem.UsedPercent,
		SwapTotalGB:     bytesToGB(swap.Total),
		SwapUsedGB:      bytesToGB(swap.Used),
		Disks:           disks,
	}

	if len(ioCounters) > 0 {
		overview.ReceivedGB = bytesToGB(ioCounters[0].BytesRecv)
		overview.SentGB = bytesToGB(ioCounters[0].BytesSent)
	}

	return overview, nil
}

func collectDisks(ctx context.Context) ([]DiskInfo, error) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}

	disks := make([]DiskInfo, 0, len(partitions))
	seen := make(map[string]struct{})
	for _, partition := range partitions {
		device := strings.TrimSpace(partition.Device)
		if device == "" {
			continue
		}
		if _, ok := seen[device]; ok {
			continue
		}
		seen[device] = struct{}{}

		usage, err := disk.UsageWithContext(ctx, partition.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}

		label := device
		if partition.Mountpoint != "" && partition.Mountpoint != device {
			label = fmt.Sprintf("%s (%s)", device, partition.Mountpoint)
		}

		disks = append(disks, DiskInfo{
			Path:       partition.Mountpoint,
			Label:      label,
			FileSystem: partition.Fstype,
			TotalGB:    bytesToGB(usage.Total),
			UsedGB:     bytesToGB(usage.Used),
			UsedPct:    usage.UsedPercent,
		})
	}

	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Path < disks[j].Path
	})

	return disks, nil
}

func collectProcesses(ctx context.Context) ([]ProcessInfo, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]ProcessInfo, 0, len(procs))
	nameCache := make(map[int32]string)
	for _, proc := range procs {
		info, ok := collectProcessInfo(ctx, proc)
		if !ok {
			continue
		}
		results = append(results, info)
		nameCache[info.PID] = info.Name
	}

	for index := range results {
		if name, ok := nameCache[results[index].ParentPID]; ok {
			results[index].ParentName = name
			continue
		}
		results[index].ParentName = processNameFromPID(ctx, results[index].ParentPID, nameCache)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].MemoryMB == results[j].MemoryMB {
			return results[i].PID < results[j].PID
		}
		return results[i].MemoryMB > results[j].MemoryMB
	})

	return results, nil
}

func collectProcessInfo(ctx context.Context, proc *process.Process) (ProcessInfo, bool) {
	name, err := proc.NameWithContext(ctx)
	if err != nil || strings.TrimSpace(name) == "" {
		return ProcessInfo{}, false
	}

	memoryInfo, _ := proc.MemoryInfoWithContext(ctx)
	threadCount, _ := proc.NumThreadsWithContext(ctx)
	statuses, _ := proc.StatusWithContext(ctx)
	exe, _ := proc.ExeWithContext(ctx)
	cmdline, _ := proc.CmdlineWithContext(ctx)
	cpuPercent, _ := proc.CPUPercentWithContext(ctx)
	parentPID, _ := proc.PpidWithContext(ctx)

	var memoryMB float64
	if memoryInfo != nil {
		memoryMB = bytesToMB(memoryInfo.RSS)
	}

	return ProcessInfo{
		PID:         proc.Pid,
		ParentPID:   parentPID,
		Name:        name,
		Path:        exe,
		CommandLine: cmdline,
		Status:      joinStatuses(statuses),
		Threads:     threadCount,
		CPUPercent:  cpuPercent,
		MemoryMB:    memoryMB,
	}, true
}

func collectConnections(ctx context.Context) ([]ConnectionInfo, error) {
	conns, err := gnet.ConnectionsWithContext(ctx, "all")
	if err != nil {
		return nil, err
	}

	procNameCache := make(map[int32]string)
	results := make([]ConnectionInfo, 0, len(conns))

	for _, conn := range conns {
		pid := conn.Pid
		results = append(results, ConnectionInfo{
			Protocol:    protocolFromFamily(conn.Type),
			Status:      normalizeStatus(conn.Status),
			LocalAddr:   formatAddr(conn.Laddr.IP, conn.Laddr.Port),
			RemoteAddr:  formatAddr(conn.Raddr.IP, conn.Raddr.Port),
			PID:         pid,
			ProcessName: processNameFromPID(ctx, pid, procNameCache),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].PID == results[j].PID {
			return results[i].LocalAddr < results[j].LocalAddr
		}
		return results[i].PID < results[j].PID
	})

	return results, nil
}

func buildProcessTree(processes []ProcessInfo) []ProcessTreeNode {
	if len(processes) == 0 {
		return []ProcessTreeNode{}
	}

	byPID := make(map[int32]ProcessInfo, len(processes))
	children := make(map[int32][]ProcessInfo, len(processes))
	for _, proc := range processes {
		byPID[proc.PID] = proc
		children[proc.ParentPID] = append(children[proc.ParentPID], proc)
	}

	for parentPID := range children {
		sort.Slice(children[parentPID], func(i, j int) bool {
			if children[parentPID][i].PID == children[parentPID][j].PID {
				return strings.ToLower(children[parentPID][i].Name) < strings.ToLower(children[parentPID][j].Name)
			}
			return children[parentPID][i].PID < children[parentPID][j].PID
		})
	}

	rootItems := make([]ProcessInfo, 0)
	for _, proc := range processes {
		if proc.ParentPID == 0 || proc.ParentPID == proc.PID {
			rootItems = append(rootItems, proc)
			continue
		}
		if _, ok := byPID[proc.ParentPID]; !ok {
			rootItems = append(rootItems, proc)
		}
	}

	sort.Slice(rootItems, func(i, j int) bool {
		return rootItems[i].PID < rootItems[j].PID
	})

	flat := make([]ProcessTreeNode, 0, len(processes))
	seen := make(map[int32]struct{}, len(processes))
	var walk func(ProcessInfo, int)
	walk = func(proc ProcessInfo, depth int) {
		if _, ok := seen[proc.PID]; ok {
			return
		}
		seen[proc.PID] = struct{}{}
		childrenList := children[proc.PID]
		flat = append(flat, ProcessTreeNode{
			PID:         proc.PID,
			ParentPID:   proc.ParentPID,
			Depth:       depth,
			Name:        proc.Name,
			Path:        proc.Path,
			Status:      proc.Status,
			Threads:     proc.Threads,
			CPUPercent:  proc.CPUPercent,
			MemoryMB:    proc.MemoryMB,
			HasChildren: len(childrenList) > 0,
		})
		for _, child := range childrenList {
			walk(child, depth+1)
		}
	}

	for _, proc := range rootItems {
		walk(proc, 0)
	}

	for _, proc := range processes {
		walk(proc, 0)
	}

	return flat
}

func buildHighlights(
	overview Overview,
	processes []ProcessInfo,
	services []ServiceInfo,
	connections []ConnectionInfo,
	autoruns []AutorunEntry,
) []Highlight {
	highlights := []Highlight{
		{
			Title:  "系统运行时间",
			Level:  "good",
			Detail: fmt.Sprintf("主机已持续运行 %s。", overview.Uptime),
		},
		{
			Title:  "活动面概览",
			Level:  "good",
			Detail: fmt.Sprintf("当前有 %d 个进程、%d 项服务、%d 条网络连接。", len(processes), len(services), len(connections)),
		},
	}

	if overview.CPULoad >= 80 {
		highlights = append(highlights, Highlight{
			Title:  "CPU 负载偏高",
			Level:  "warning",
			Detail: fmt.Sprintf("整机 CPU 占用已到 %.1f%%。", overview.CPULoad),
		})
	}

	if overview.MemoryUsedPct >= 85 {
		highlights = append(highlights, Highlight{
			Title:  "内存压力偏高",
			Level:  "warning",
			Detail: fmt.Sprintf("当前内存使用率为 %.1f%%。", overview.MemoryUsedPct),
		})
	}

	if topDisk := fullestDisk(overview.Disks); topDisk != nil && topDisk.UsedPct >= 85 {
		highlights = append(highlights, Highlight{
			Title:  "磁盘空间告急",
			Level:  "warning",
			Detail: fmt.Sprintf("%s 已使用 %.1f%%。", topDisk.Label, topDisk.UsedPct),
		})
	}

	if len(autoruns) >= 20 {
		highlights = append(highlights, Highlight{
			Title:  "启动项偏多",
			Level:  "attention",
			Detail: fmt.Sprintf("检测到 %d 个启动项，建议排查是否存在冗余自启动。", len(autoruns)),
		})
	}

	if len(processes) > 0 {
		topProcess := processes[0]
		highlights = append(highlights, Highlight{
			Title:  "占用内存最高的进程",
			Level:  "attention",
			Detail: fmt.Sprintf("%s (PID %d) 当前占用 %.1f MB。", topProcess.Name, topProcess.PID, topProcess.MemoryMB),
		})
	}

	return highlights
}

func firstCPUModel(entries []cpu.InfoStat) string {
	for _, entry := range entries {
		if strings.TrimSpace(entry.ModelName) != "" {
			return entry.ModelName
		}
	}
	return "Unknown"
}

func joinStatuses(statuses []string) string {
	if len(statuses) == 0 {
		return "Unknown"
	}
	return strings.Join(statuses, ", ")
}

func processNameFromPID(ctx context.Context, pid int32, cache map[int32]string) string {
	if pid == 0 {
		return "System"
	}
	if value, ok := cache[pid]; ok {
		return value
	}
	proc, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		cache[pid] = "Unknown"
		return cache[pid]
	}
	name, err := proc.NameWithContext(ctx)
	if err != nil || name == "" {
		cache[pid] = "Unknown"
		return cache[pid]
	}
	cache[pid] = name
	return name
}

func protocolFromFamily(kind uint32) string {
	switch kind {
	case 1:
		return "TCP"
	case 2:
		return "UDP"
	default:
		return "Other"
	}
}

func normalizeStatus(status string) string {
	if strings.TrimSpace(status) == "" {
		return "LISTEN/IDLE"
	}
	return strings.ToUpper(status)
}

func formatAddr(ip string, port uint32) string {
	if strings.TrimSpace(ip) == "" {
		return "-"
	}
	if port == 0 {
		return ip
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

func fullestDisk(disks []DiskInfo) *DiskInfo {
	if len(disks) == 0 {
		return nil
	}
	index := 0
	for current := 1; current < len(disks); current++ {
		if disks[current].UsedPct > disks[index].UsedPct {
			index = current
		}
	}
	return &disks[index]
}

func firstFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func bytesToGB(value uint64) float64 {
	return float64(value) / 1024 / 1024 / 1024
}

func bytesToMB(value uint64) float64 {
	return float64(value) / 1024 / 1024
}

func formatDuration(value time.Duration) string {
	if value <= 0 {
		return "0m"
	}

	days := value / (24 * time.Hour)
	value -= days * 24 * time.Hour
	hours := value / time.Hour
	value -= hours * time.Hour
	minutes := value / time.Minute

	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}

	return strings.Join(parts, " ")
}

func cloneProcessSlice(values []ProcessInfo) []ProcessInfo {
	if len(values) == 0 {
		return []ProcessInfo{}
	}
	cloned := make([]ProcessInfo, len(values))
	copy(cloned, values)
	return cloned
}

func cloneServiceSlice(values []ServiceInfo) []ServiceInfo {
	if len(values) == 0 {
		return []ServiceInfo{}
	}
	cloned := make([]ServiceInfo, len(values))
	copy(cloned, values)
	return cloned
}

func cloneConnectionSlice(values []ConnectionInfo) []ConnectionInfo {
	if len(values) == 0 {
		return []ConnectionInfo{}
	}
	cloned := make([]ConnectionInfo, len(values))
	copy(cloned, values)
	return cloned
}

func cloneAutorunSlice(values []AutorunEntry) []AutorunEntry {
	if len(values) == 0 {
		return []AutorunEntry{}
	}
	cloned := make([]AutorunEntry, len(values))
	copy(cloned, values)
	return cloned
}
