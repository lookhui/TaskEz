package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	defaultWindowWidth  = 1480
	defaultWindowHeight = 920
)

type UISettings struct {
	WindowMode string `json:"windowMode"`
}

type UISettingsStore struct {
	mu       sync.RWMutex
	path     string
	settings UISettings
}

func NewUISettingsStore() *UISettingsStore {
	store := &UISettingsStore{
		path:     uiSettingsPath(),
		settings: defaultUISettings(),
	}
	store.load()
	return store
}

func defaultUISettings() UISettings {
	return UISettings{WindowMode: "half"}
}

func uiSettingsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		return filepath.Join(".", "ui_settings.json")
	}
	return filepath.Join(configDir, "TaskEz", "ui_settings.json")
}

func (s *UISettingsStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	content, err := os.ReadFile(s.path)
	if err != nil {
		return
	}

	var loaded UISettings
	if err := json.Unmarshal(content, &loaded); err != nil {
		return
	}

	if isSupportedWindowMode(loaded.WindowMode) {
		s.settings.WindowMode = loaded.WindowMode
	}
}

func (s *UISettingsStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, content, 0o644)
}

func (s *UISettingsStore) Get() UISettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *UISettingsStore) SetWindowMode(mode string) (UISettings, error) {
	if !isSupportedWindowMode(mode) {
		return UISettings{}, fmt.Errorf("unsupported window mode: %s", mode)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings.WindowMode = mode
	if err := s.saveLocked(); err != nil {
		return UISettings{}, err
	}
	return s.settings, nil
}

func isSupportedWindowMode(mode string) bool {
	switch mode {
	case "standard", "half", "fullscreen":
		return true
	default:
		return false
	}
}

func (a *App) applyWindowMode(mode string) error {
	if a.ctx == nil {
		return nil
	}

	switch mode {
	case "fullscreen":
		runtime.WindowFullscreen(a.ctx)
		return nil
	case "half":
		runtime.WindowUnfullscreen(a.ctx)
		width, height := halfWindowSize(a.ctx)
		runtime.WindowSetSize(a.ctx, width, height)
		runtime.WindowCenter(a.ctx)
		return nil
	case "standard":
		runtime.WindowUnfullscreen(a.ctx)
		runtime.WindowSetSize(a.ctx, defaultWindowWidth, defaultWindowHeight)
		runtime.WindowCenter(a.ctx)
		return nil
	default:
		return fmt.Errorf("unsupported window mode: %s", mode)
	}
}

func halfWindowSize(ctx context.Context) (int, int) {
	screens, err := runtime.ScreenGetAll(ctx)
	if err != nil || len(screens) == 0 {
		return 1080, defaultWindowHeight
	}

	screen := screens[0]
	for _, item := range screens {
		if item.IsCurrent || item.IsPrimary {
			screen = item
			break
		}
	}

	width := screen.Size.Width / 2
	height := int(float64(screen.Size.Height) * 0.92)
	if width < 980 {
		width = 980
	}
	if height < 780 {
		height = 780
	}
	return width, height
}
