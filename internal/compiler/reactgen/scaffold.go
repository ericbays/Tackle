package reactgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Workspace represents a temporary directory for React transpilation.
type Workspace struct {
	DirPath string
}

// generateID creates a quick unique identifier for the temp directory.
func generateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

// CreateWorkspace initializes a rapid, temporary directory in the OS temp space
// to handle the TSX file generation before esbuild bundles it in-memory.
func CreateWorkspace() (*Workspace, error) {
	targetPath := filepath.Join(os.TempDir(), fmt.Sprintf("tackle-react-%s", generateID()))

	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temporary compilation workspace: %w", err)
	}

	return &Workspace{DirPath: targetPath}, nil
}

// Cleanup permanently deletes the temporary TSX files and directories.
// This is critical to call via defer to prevent /tmp bloat.
func (w *Workspace) Cleanup() {
	_ = os.RemoveAll(w.DirPath)
}

// WriteIndex is a scaffold function that writes the base index.tsx file,
// mounting the main React App component.
func (w *Workspace) WriteIndex() error {
	content := `import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';

const container = document.getElementById('root');
if (container) {
	const root = createRoot(container);
	root.render(<App />);
}
`
	return os.WriteFile(filepath.Join(w.DirPath, "index.tsx"), []byte(content), 0644)
}

// WriteFile writes arbitrary dynamically generated code (e.g., App.tsx) to the workspace.
func (w *Workspace) WriteFile(filename string, content string) error {
	return os.WriteFile(filepath.Join(w.DirPath, filename), []byte(content), 0644)
}
