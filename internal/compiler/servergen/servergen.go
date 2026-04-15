package servergen

import (
	"fmt"
	"os"
	"path/filepath"
)

// GenerateWorkspace parses the builder config, produces a valid Go module + React App, and prepares the physical hard drive directory for the Go compiler
func GenerateWorkspace(buildDir string, projectID, buildID string, definition map[string]any, isDevelopment bool) ([]string, error) {
	staticDir := filepath.Join(buildDir, "static")
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		return nil, fmt.Errorf("create static dir: %w", err)
	}

	var generatedFiles []string

	// 1. Generate Frontend JSX
	reactFiles, err := GenerateReactApp(definition, isDevelopment)
	if err != nil {
		return nil, fmt.Errorf("generate frontend: %w", err)
	}

	// Write JSX files to disk so esbuild can target them
	for filename, content := range reactFiles {
		path := filepath.Join(buildDir, filename) // Root of workspace
		if filename == "index.html" || filename == "index.css" {
			path = filepath.Join(staticDir, filename) // Move raw assets immediately into static scope
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("write react file %s: %w", filename, err)
		}
		generatedFiles = append(generatedFiles, filename)
	}

	// 2. Transpile React JSX via Go ESBuild
	entryPoint := filepath.Join(buildDir, "index.jsx")
	outDir := staticDir
	if err := BundleReact(entryPoint, outDir, !isDevelopment); err != nil {
		return nil, fmt.Errorf("bundle react: %w", err)
	}
	generatedFiles = append(generatedFiles, "static/index.js")

	// 3. Generate Backend Go Architecture
	goFiles, err := GenerateBackend(projectID, buildID, definition, isDevelopment)
	if err != nil {
		return nil, fmt.Errorf("generate backend: %w", err)
	}

	// Write Go files
	for filename, content := range goFiles {
		path := filepath.Join(buildDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("write go file %s: %w", filename, err)
		}
		generatedFiles = append(generatedFiles, filename)
	}

	// 4. Generate go.mod (zero dependencies!)
	goModContent := "module landing-app\n\ngo 1.23\n"
	if err := os.WriteFile(filepath.Join(buildDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		return nil, fmt.Errorf("write go.mod: %w", err)
	}
	generatedFiles = append(generatedFiles, "go.mod")

	return generatedFiles, nil
}

// GenerateFrontendOnly skips Go backend scaffolding and exclusively re-transpiles the React JSON AST into the target directory's static folder
func GenerateFrontendOnly(buildDir string, definition map[string]any, isDevelopment bool) error {
	staticDir := filepath.Join(buildDir, "static")
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		return fmt.Errorf("create static dir: %w", err)
	}

	// 1. Generate Frontend JSX
	reactFiles, err := GenerateReactApp(definition, isDevelopment)
	if err != nil {
		return fmt.Errorf("generate frontend: %w", err)
	}

	// Write JSX files to disk so esbuild can target them
	for filename, content := range reactFiles {
		path := filepath.Join(buildDir, filename)
		if filename == "index.html" || filename == "index.css" {
			path = filepath.Join(staticDir, filename)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write react file %s: %w", filename, err)
		}
	}

	// 2. Transpile React JSX via Go ESBuild
	entryPoint := filepath.Join(buildDir, "index.jsx")
	outDir := staticDir
	if err := BundleReact(entryPoint, outDir, !isDevelopment); err != nil {
		return fmt.Errorf("bundle react: %w", err)
	}

	return nil
}
