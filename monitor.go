package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const maxMonitorEvents = 200

type MonitorManager struct {
	mu              sync.RWMutex
	started         bool
	startedAt       time.Time
	watcher         *fsnotify.Watcher
	stop            chan struct{}
	wg              sync.WaitGroup
	fileEvents      []WatchEvent
	registryEvents  []WatchEvent
	warnings        []string
	watchedPaths    []string
	watchedRegistry []string
	autorunState    map[string]string
}

func NewMonitorManager() *MonitorManager {
	return &MonitorManager{
		stop:            make(chan struct{}),
		fileEvents:      []WatchEvent{},
		registryEvents:  []WatchEvent{},
		warnings:        []string{},
		watchedPaths:    []string{},
		watchedRegistry: defaultWatchedRegistry(),
		autorunState:    map[string]string{},
	}
}

func (m *MonitorManager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.startedAt = time.Now()
	m.mu.Unlock()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		m.addWarning(fmt.Sprintf("文件监控初始化失败: %v", err))
		return
	}

	m.mu.Lock()
	m.watcher = watcher
	m.mu.Unlock()

	for _, path := range defaultWatchedPaths() {
		if strings.TrimSpace(path) == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		if err := watcher.Add(path); err != nil {
			m.addWarning(fmt.Sprintf("无法监控目录 %s: %v", path, err))
			continue
		}
		m.mu.Lock()
		m.watchedPaths = append(m.watchedPaths, path)
		m.mu.Unlock()
	}

	if entries, err := collectAutoruns(); err == nil {
		m.mu.Lock()
		m.autorunState = buildAutorunState(entries)
		m.mu.Unlock()
	} else {
		m.addWarning(fmt.Sprintf("注册表监控初始快照失败: %v", err))
	}

	m.wg.Add(2)
	go m.consumeFileEvents()
	go m.pollAutoruns(ctx)
}

func (m *MonitorManager) Stop() {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	select {
	case <-m.stop:
	default:
		close(m.stop)
	}
	watcher := m.watcher
	m.watcher = nil
	m.mu.Unlock()

	if watcher != nil {
		_ = watcher.Close()
	}
	m.wg.Wait()
}

func (m *MonitorManager) Snapshot() MonitorState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state := MonitorState{
		StartedAt:       "",
		WatchedPaths:    append([]string{}, m.watchedPaths...),
		WatchedRegistry: append([]string{}, m.watchedRegistry...),
		FileEvents:      append([]WatchEvent{}, m.fileEvents...),
		RegistryEvents:  append([]WatchEvent{}, m.registryEvents...),
		Warnings:        append([]string{}, m.warnings...),
	}
	if !m.startedAt.IsZero() {
		state.StartedAt = m.startedAt.Format(time.RFC3339)
	}
	return state
}

func (m *MonitorManager) consumeFileEvents() {
	defer m.wg.Done()

	m.mu.RLock()
	watcher := m.watcher
	m.mu.RUnlock()
	if watcher == nil {
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			m.pushFileEvent(WatchEvent{
				Time:   time.Now().Format(time.RFC3339),
				Source: "文件",
				Action: fileOpLabel(event.Op),
				Target: event.Name,
				Detail: event.Op.String(),
			})
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			m.addWarning(fmt.Sprintf("文件监控错误: %v", err))
		case <-m.stop:
			return
		}
	}
}

func (m *MonitorManager) pollAutoruns(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			entries, err := collectAutoruns()
			if err != nil {
				m.addWarning(fmt.Sprintf("注册表监控轮询失败: %v", err))
				continue
			}

			next := buildAutorunState(entries)

			m.mu.Lock()
			prev := m.autorunState
			m.autorunState = next
			m.mu.Unlock()

			for _, event := range diffAutorunState(prev, next) {
				m.pushRegistryEvent(event)
			}
		case <-ctx.Done():
			return
		case <-m.stop:
			return
		}
	}
}

func (m *MonitorManager) pushFileEvent(event WatchEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fileEvents = append([]WatchEvent{event}, m.fileEvents...)
	if len(m.fileEvents) > maxMonitorEvents {
		m.fileEvents = m.fileEvents[:maxMonitorEvents]
	}
}

func (m *MonitorManager) pushRegistryEvent(event WatchEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registryEvents = append([]WatchEvent{event}, m.registryEvents...)
	if len(m.registryEvents) > maxMonitorEvents {
		m.registryEvents = m.registryEvents[:maxMonitorEvents]
	}
}

func (m *MonitorManager) addWarning(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.warnings) >= 20 {
		return
	}
	m.warnings = append(m.warnings, message)
}

func defaultWatchedPaths() []string {
	return []string{
		filepath.Join(os.Getenv("USERPROFILE"), "Desktop"),
		os.Getenv("TEMP"),
		filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup"),
		filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup"),
	}
}

func defaultWatchedRegistry() []string {
	return []string{
		`HKLM\Software\Microsoft\Windows\CurrentVersion\Run`,
		`HKLM\Software\Microsoft\Windows\CurrentVersion\RunOnce`,
		`HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`,
		`HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\RunOnce`,
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		`HKCU\Software\Microsoft\Windows\CurrentVersion\RunOnce`,
	}
}

func buildAutorunState(entries []AutorunEntry) map[string]string {
	state := make(map[string]string)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Location, "HK") {
			continue
		}
		key := strings.Join([]string{entry.Scope, entry.Location, entry.Name}, "|")
		state[key] = entry.Command
	}
	return state
}

func diffAutorunState(previous, next map[string]string) []WatchEvent {
	events := make([]WatchEvent, 0)
	now := time.Now().Format(time.RFC3339)

	for key, value := range next {
		oldValue, exists := previous[key]
		switch {
		case !exists:
			events = append(events, WatchEvent{
				Time:   now,
				Source: "注册表",
				Action: "新增",
				Target: key,
				Detail: value,
			})
		case oldValue != value:
			events = append(events, WatchEvent{
				Time:   now,
				Source: "注册表",
				Action: "修改",
				Target: key,
				Detail: value,
			})
		}
	}

	for key, value := range previous {
		if _, exists := next[key]; exists {
			continue
		}
		events = append(events, WatchEvent{
			Time:   now,
			Source: "注册表",
			Action: "删除",
			Target: key,
			Detail: value,
		})
	}

	sortWatchEvents(events)
	return events
}

func fileOpLabel(op fsnotify.Op) string {
	switch {
	case op&fsnotify.Create == fsnotify.Create:
		return "创建"
	case op&fsnotify.Write == fsnotify.Write:
		return "写入"
	case op&fsnotify.Remove == fsnotify.Remove:
		return "删除"
	case op&fsnotify.Rename == fsnotify.Rename:
		return "重命名"
	case op&fsnotify.Chmod == fsnotify.Chmod:
		return "属性变更"
	default:
		return "变化"
	}
}

func sortWatchEvents(events []WatchEvent) {
	if len(events) < 2 {
		return
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Target < events[j].Target
	})
}
