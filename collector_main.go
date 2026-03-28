//go:build collector

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	bundle, err := collectAnalysisBundle(context.Background())
	if err != nil {
		os.Exit(1)
	}

	outputDir, err := os.Getwd()
	if err != nil || outputDir == "" {
		outputDir = "."
	}

	filename := fmt.Sprintf("TaskEz_%s.aldb", time.Now().Format("20060102_150405"))
	outputPath := filepath.Join(outputDir, filename)
	if err := writeBundleFile(outputPath, bundle); err != nil {
		os.Exit(1)
	}
}
