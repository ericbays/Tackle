package typosquat

import (
	"strings"
	"unicode"
)

// CalculateSimilarity computes a normalized similarity score between 0.0 and 1.0.
// 1.0 means the candidate looks identical to the original (e.g. homoglyph replacement).
// 0.0 means completely dissimilar.
func CalculateSimilarity(original, candidate string) float64 {
	original = strings.ToLower(strings.TrimSpace(original))
	candidate = strings.ToLower(strings.TrimSpace(candidate))

	if original == candidate {
		return 1.0
	}
	if original == "" || candidate == "" {
		return 0.0
	}

	origSLD, origTLD := splitDomain(original)
	candSLD, candTLD := splitDomain(candidate)

	// Base: normalized Levenshtein on the SLD.
	sldScore := normalizedLevenshtein(origSLD, candSLD)

	// TLD bonus: same TLD adds 0.1.
	tldBonus := 0.0
	if origTLD == candTLD {
		tldBonus = 0.1
	}

	// Homoglyph bonus: if the candidate SLD is a homoglyph of the original SLD,
	// add a bonus since it looks identical but is technically different.
	homoglyphBonus := 0.0
	if isHomoglyphMatch(origSLD, candSLD) {
		homoglyphBonus = 0.15
	}

	// Length penalty: penalize if lengths differ significantly.
	lengthDiff := abs(len([]rune(origSLD)) - len([]rune(candSLD)))
	maxLen := max(len([]rune(origSLD)), len([]rune(candSLD)))
	lengthPenalty := 0.0
	if maxLen > 0 {
		lengthPenalty = float64(lengthDiff) / float64(maxLen) * 0.05
	}

	score := sldScore + tldBonus + homoglyphBonus - lengthPenalty
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}
	return score
}

// normalizedLevenshtein returns 1.0 - (editDistance / max(len(a), len(b))).
func normalizedLevenshtein(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	dist := levenshtein(ra, rb)
	maxLen := max(len(ra), len(rb))
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(dist)/float64(maxLen)
}

// levenshtein computes the Levenshtein edit distance between two rune slices.
func levenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			curr[j] = minOf(ins, del, sub)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// isHomoglyphMatch returns true if every rune in candidate is either equal to or
// a registered homoglyph of the corresponding rune in original.
func isHomoglyphMatch(original, candidate string) bool {
	ro, rc := []rune(original), []rune(candidate)
	if len(ro) != len(rc) {
		return false
	}
	anyDiffers := false
	for i, o := range ro {
		c := rc[i]
		if o == c {
			continue
		}
		// Check if c is a homoglyph of o.
		glyphs, ok := homoglyphs[unicode.ToLower(o)]
		if !ok {
			return false
		}
		found := false
		for _, g := range glyphs {
			if g == c {
				found = true
				break
			}
		}
		if !found {
			return false
		}
		anyDiffers = true
	}
	return anyDiffers // at least one char was different but all were homoglyphs
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minOf(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
