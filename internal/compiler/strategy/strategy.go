package strategy

import (
	"fmt"
	"math/rand"
	"strings"
)

// PageDefinition represents a simplified page definition for code generation.
// The real definition comes from the JSON page builder; this captures what
// strategies need to generate code.
type PageDefinition struct {
	Pages      []PageDef
	GlobalCSS  string
	GlobalJS   string
	CampaignID string
	BuildToken string
}

type PageDef struct {
	ID        string
	Title     string
	Path      string // e.g., "/", "/login", "/success"
	HTML      string // page HTML content
	CSS       string // page-specific CSS
	JS        string // page-specific JS
	Forms     []FormDef
	IsDefault bool // true for the landing/index page
}

type FormDef struct {
	ID     string
	Action string   // submission endpoint path
	Method string   // POST
	Fields []string // field names
}

// BuildOutput contains all generated files for a strategy.
type BuildOutput struct {
	Files      map[string][]byte // path -> content (HTML, CSS, JS, Go files)
	EntryPoint string            // main Go file path
	Strategy   string            // "spa", "multifile", or "hybrid"
	Manifest   map[string]any    // generation decisions
}

// CodeGenerator generates build output from a page definition.
type CodeGenerator interface {
	// Generate produces all files for a landing page build.
	Generate(definition PageDefinition, seed int64) (*BuildOutput, error)
}

// StrategyName constants
const (
	StrategySPA       = "spa"
	StrategyMultiFile = "multifile"
	StrategyHybrid    = "hybrid"
)

// SelectStrategy returns a CodeGenerator for the given strategy name.
// If name is empty, a random strategy is selected using the seed.
func SelectStrategy(name string, seed int64) (CodeGenerator, error) {
	if name == "" {
		// Use seed to deterministically select a strategy
		r := rand.New(rand.NewSource(seed))
		strategies := AllStrategies()
		name = strategies[r.Intn(len(strategies))]
	}

	switch name {
	case StrategySPA:
		return &SPAGenerator{}, nil
	case StrategyMultiFile:
		return &MultiFileGenerator{}, nil
	case StrategyHybrid:
		return &HybridGenerator{}, nil
	default:
		return nil, fmt.Errorf("invalid strategy name %q: must be one of %s", name, strings.Join(AllStrategies(), ", "))
	}
}

// AllStrategies returns all available strategy names.
func AllStrategies() []string {
	return []string{StrategySPA, StrategyMultiFile, StrategyHybrid}
}
