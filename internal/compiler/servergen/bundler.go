package servergen

import (
	"fmt"
	"github.com/evanw/esbuild/pkg/api"
)

// BundleReact bundles the generated JSX files into static outputs physically in the outdir
// Returns an error if the compilation fails. No NodeJS runtime is required.
func BundleReact(entryFilePath, outDir string, isProduction bool) error {
	options := api.BuildOptions{
		EntryPoints: []string{entryFilePath},
		Outdir:      outDir,
		Bundle:      true,
		Write:       true,
		JSXFactory:  "React.createElement",
		JSXFragment: "React.Fragment",
		Loader: map[string]api.Loader{
			".js":   api.LoaderJSX,
			".jsx":  api.LoaderJSX,
			".css":  api.LoaderCSS,
		},
		External: []string{"react", "react-dom", "react-router-dom"}, // Map externally via CDN
		Format:   api.FormatESModule,
		Engines: []api.Engine{
			{Name: api.EngineChrome, Version: "100"},
		},
	}

	if isProduction {
		options.MinifyWhitespace = true
		options.MinifyIdentifiers = true
		options.MinifySyntax = true
	} else {
		options.Sourcemap = api.SourceMapInline
	}

	result := api.Build(options)

	if len(result.Errors) > 0 {
		// Just extract the first critical error for reporting
		firstErr := result.Errors[0]
		return fmt.Errorf("esbuild compile error: %s at %s:%d", firstErr.Text, firstErr.Location.File, firstErr.Location.Line)
	}

	return nil
}
