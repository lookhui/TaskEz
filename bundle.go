package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	bundleMagic  = "ALDB1"
	bundleSecret = "TaskEz::offline-bundle::2026"
)

func collectAnalysisBundle(ctx context.Context) (*AnalysisBundle, error) {
	snapshot, err := collectSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	inventory, err := collectInventory(ctx)
	if err != nil {
		return nil, err
	}

	host := snapshot.Overview.Hostname
	if host == "" {
		host = "unknown-host"
	}

	return &AnalysisBundle{
		Version:     bundleMagic,
		Host:        host,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Snapshot:    *snapshot,
		Inventory:   *inventory,
	}, nil
}

func exportCurrentBundle(ctx context.Context) (string, error) {
	bundle, err := collectAnalysisBundle(ctx)
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("TaskEz_%s.aldb", time.Now().Format("20060102_150405"))
	targetPath := defaultBundlePath(filename)

	if ctx != nil {
		selectedPath, dialogErr := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
			DefaultFilename: filename,
			Title:           "导出分析包",
		})
		if dialogErr == nil && selectedPath != "" {
			targetPath = selectedPath
		}
	}

	if err := writeBundleFile(targetPath, bundle); err != nil {
		return "", err
	}

	return targetPath, nil
}

func importBundleDialog(ctx context.Context) (*AnalysisBundle, error) {
	if ctx == nil {
		return nil, fmt.Errorf("window context is not ready")
	}

	selectedPath, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "导入分析包",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "TaskEz Bundle (*.aldb)",
				Pattern:     "*.aldb",
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if selectedPath == "" {
		return nil, fmt.Errorf("no bundle selected")
	}

	return readBundleFile(selectedPath)
}

func defaultBundlePath(filename string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return filename
	}
	return filepath.Join(homeDir, "Desktop", filename)
}

func writeBundleFile(path string, bundle *AnalysisBundle) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	payload, err := json.Marshal(bundle)
	if err != nil {
		return err
	}

	compressed, err := gzipBytes(payload)
	if err != nil {
		return err
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}

	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}

	key := deriveBundleKey(salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	encrypted := gcm.Seal(nil, nonce, compressed, nil)

	buffer := bytes.NewBufferString(bundleMagic)
	buffer.Write(salt)
	buffer.Write(nonce)
	buffer.Write(encrypted)

	return os.WriteFile(path, buffer.Bytes(), 0o600)
}

func readBundleFile(path string) (*AnalysisBundle, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(content) < len(bundleMagic)+16+12 || string(content[:len(bundleMagic)]) != bundleMagic {
		return nil, fmt.Errorf("invalid bundle format")
	}

	offset := len(bundleMagic)
	salt := content[offset : offset+16]
	offset += 16
	nonce := content[offset : offset+12]
	offset += 12
	encrypted := content[offset:]

	key := deriveBundleKey(salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	compressed, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("bundle decrypt failed")
	}

	payload, err := ungzipBytes(compressed)
	if err != nil {
		return nil, err
	}

	var bundle AnalysisBundle
	if err := json.Unmarshal(payload, &bundle); err != nil {
		return nil, err
	}

	return &bundle, nil
}

func deriveBundleKey(salt []byte) []byte {
	sum := sha256.Sum256(append([]byte(bundleSecret), salt...))
	return sum[:]
}

func gzipBytes(payload []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(payload); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func ungzipBytes(payload []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}
