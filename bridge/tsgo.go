// bridge/tsgo.go
package bridge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/execute"
	"github.com/microsoft/typescript-go/internal/tspath"
	"github.com/microsoft/typescript-go/internal/vfs"
	"github.com/microsoft/typescript-go/internal/vfs/osvfs"
)

type bridgeSystem struct {
	writer             io.Writer
	fs                 vfs.FS
	defaultLibraryPath string
	newLine            string
	cwd                string
	start              time.Time
}

func (s *bridgeSystem) SinceStart() time.Duration {
	return time.Since(s.start)
}

func (s *bridgeSystem) Now() time.Time {
	return time.Now()
}

func (s *bridgeSystem) FS() vfs.FS {
	return s.fs
}

func (s *bridgeSystem) DefaultLibraryPath() string {
	return s.defaultLibraryPath
}

func (s *bridgeSystem) GetCurrentDirectory() string {
	return s.cwd
}

func (s *bridgeSystem) NewLine() string {
	return s.newLine
}

func (s *bridgeSystem) Writer() io.Writer {
	return s.writer
}

func (s *bridgeSystem) EndWrite() {
	// do nothing, this is needed in the interface for testing
}

func newBridgeSystem() *bridgeSystem {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	return &bridgeSystem{
		cwd:                tspath.NormalizePath(cwd),
		fs:                 bundled.WrapFS(osvfs.FS()),
		defaultLibraryPath: bundled.LibPath(),
		writer:             os.Stdout,
		newLine:            core.IfElse(runtime.GOOS == "windows", "\r\n", "\n"),
		start:              time.Now(),
	}
}

func Build(configPath string) error {
	sys := newBridgeSystem()
	exit := execute.CommandLine(sys, nil, []string{"-p", configPath})
	if exit != 0 {
		return fmt.Errorf("tsgo build failed with exit code %d", exit)
	}

	distPath := filepath.Join(configPath, "dist")
	fmt.Printf("Build output directory: %s\n", distPath)

	if _, err := os.Stat(distPath); os.IsNotExist(err) {
		fmt.Printf("Dist directory does not exist: %s\n", distPath)
		return nil
	}

	entries, err := os.ReadDir(distPath)
	if err != nil {
		fmt.Printf("Error reading directory %s: %v\n", distPath, err)
		return nil
	}

	fmt.Printf("Contents of %s:\n", distPath)
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("  %s/\n", entry.Name())
		} else {
			fmt.Printf("  %s\n", entry.Name())
		}
	}
	return nil
}
