// Package typosquat implements typosquat domain generation using 9 techniques,
// visual similarity scoring, and bulk availability checking via registrar APIs.
package typosquat

import (
	"strings"
	"unicode"
)

// TyposquatCandidate is a generated typosquat domain candidate.
type TyposquatCandidate struct {
	Domain     string  `json:"domain"`
	Technique  string  `json:"technique"`
	Similarity float64 `json:"similarity"`
}

// homoglyphs maps ASCII characters to visually similar Unicode alternatives.
// These are the most common characters used in IDN homoglyph attacks.
var homoglyphs = map[rune][]rune{
	'a': {'а', 'ä', 'à', 'á', 'â', 'ã', 'å'}, // Cyrillic а and accented
	'b': {'Ь', 'ḃ'},
	'c': {'с', 'ç', 'ĉ'},                       // Cyrillic с
	'd': {'ḋ', 'ď'},
	'e': {'е', 'è', 'é', 'ê', 'ë'},             // Cyrillic е
	'g': {'ġ', 'ĝ'},
	'h': {'ĥ', 'ħ'},
	'i': {'і', 'ï', 'ì', 'í', 'î', 'ĩ', 'ī'}, // Cyrillic і
	'j': {'ĵ'},
	'k': {'κ'},                                  // Greek kappa
	'l': {'І', '1', 'ĺ', 'ľ'},                 // Cyrillic capital І
	'm': {'ṁ'},
	'n': {'ñ', 'ń', 'ņ'},
	'o': {'о', 'ο', '0', 'ö', 'ò', 'ó', 'ô', 'õ'}, // Cyrillic о, Greek omicron
	'p': {'р', 'ṗ'},                             // Cyrillic р
	'q': {'ġ'},
	'r': {'ŕ', 'ř'},
	's': {'ś', 'ŝ', 'š'},
	't': {'ţ', 'ť', 'ṫ'},
	'u': {'υ', 'ü', 'ù', 'ú', 'û', 'ũ'},       // Greek upsilon
	'v': {'ν'},                                  // Greek nu
	'w': {'ω', 'ŵ'},                             // Greek omega
	'x': {'х', 'χ'},                             // Cyrillic х, Greek chi
	'y': {'у', 'ý', 'ÿ'},                        // Cyrillic у
	'z': {'ź', 'ž', 'ż'},
}

// charSubstitutions maps common letter-to-symbol substitutions.
var charSubstitutions = map[rune][]string{
	'a': {"4", "@"},
	'b': {"8"},
	'c': {"k"},
	'e': {"3"},
	'g': {"9"},
	'i': {"1", "l"},
	'l': {"1", "i"},
	'o': {"0"},
	'q': {"9"},
	's': {"5"},
	't': {"7"},
	'z': {"2"},
}

// qwertyAdjacent defines keys adjacent to each key on a standard QWERTY keyboard.
var qwertyAdjacent = map[rune]string{
	'a': "qwsz",
	'b': "vghn",
	'c': "xdfv",
	'd': "srfec",
	'e': "wrsd",
	'f': "drtgv",
	'g': "ftyhbv",
	'h': "gyujnb",
	'i': "uojk",
	'j': "huikm",
	'k': "jilon",
	'l': "kop",
	'm': "njk",
	'n': "bhjm",
	'o': "iklp",
	'p': "ol",
	'q': "wa",
	'r': "edft",
	's': "awedxz",
	't': "rfgy",
	'u': "yhij",
	'v': "cfgb",
	'w': "qase",
	'x': "zsdc",
	'y': "tghu",
	'z': "asx",
}

// commonTLDs are the TLD alternatives used for TLD swapping.
var commonTLDs = []string{
	"com", "net", "org", "co", "io", "info", "biz", "us", "xyz", "co.uk",
}

// splitDomain splits a domain name into labels and TLD.
// e.g. "example.com" -> ("example", "com")
// e.g. "sub.example.com" -> ("sub.example", "com")
func splitDomain(domain string) (sld, tld string) {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return domain, ""
	}
	// Handle two-part TLDs like co.uk.
	if len(parts) >= 3 {
		last2 := parts[len(parts)-2] + "." + parts[len(parts)-1]
		twoPartTLDs := map[string]bool{"co.uk": true, "com.au": true, "co.nz": true, "org.uk": true}
		if twoPartTLDs[last2] {
			return strings.Join(parts[:len(parts)-2], "."), last2
		}
	}
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

// GenerateTyposquats generates all typosquat candidates for a domain using all 9 techniques.
// Results are deduplicated and tagged with the generating technique.
func GenerateTyposquats(domain string) ([]TyposquatCandidate, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	sld, tld := splitDomain(domain)

	seen := make(map[string]bool)
	var candidates []TyposquatCandidate

	add := func(candidate, technique string) {
		candidate = strings.ToLower(candidate)
		if candidate == domain || seen[candidate] || candidate == "" {
			return
		}
		// Minimal sanity: must contain at least one dot.
		if !strings.Contains(candidate, ".") {
			return
		}
		seen[candidate] = true
		candidates = append(candidates, TyposquatCandidate{
			Domain:    candidate,
			Technique: technique,
		})
	}

	// 1. Character substitution.
	for i, ch := range sld {
		if subs, ok := charSubstitutions[ch]; ok {
			for _, sub := range subs {
				newSLD := sld[:i] + sub + sld[i+1:]
				add(newSLD+"."+tld, "char_substitution")
			}
		}
	}

	// 2. Homoglyph replacement.
	runes := []rune(sld)
	for i, ch := range runes {
		lower := unicode.ToLower(ch)
		if glyphs, ok := homoglyphs[lower]; ok {
			for _, g := range glyphs {
				// Skip multi-char homoglyphs stored as rune (only single rune replacements here).
				newRunes := make([]rune, len(runes))
				copy(newRunes, runes)
				newRunes[i] = g
				newSLD := string(newRunes)
				add(newSLD+"."+tld, "homoglyph")
			}
		}
	}

	// 3. Adjacent key typos.
	for i, ch := range sld {
		if adj, ok := qwertyAdjacent[ch]; ok {
			for _, a := range adj {
				newSLD := sld[:i] + string(a) + sld[i+1:]
				add(newSLD+"."+tld, "adjacent_key")
			}
		}
	}

	// 4. Character omission.
	for i := range sld {
		newSLD := sld[:i] + sld[i+1:]
		if len(newSLD) >= 2 {
			add(newSLD+"."+tld, "char_omission")
		}
	}

	// 5. Character insertion (character doubling).
	for i, ch := range sld {
		newSLD := sld[:i] + string(ch) + sld[i:]
		add(newSLD+"."+tld, "char_insertion")
	}

	// 6. Character transposition.
	runes2 := []rune(sld)
	for i := 0; i < len(runes2)-1; i++ {
		newRunes := make([]rune, len(runes2))
		copy(newRunes, runes2)
		newRunes[i], newRunes[i+1] = newRunes[i+1], newRunes[i]
		newSLD := string(newRunes)
		add(newSLD+"."+tld, "transposition")
	}

	// 7. TLD variations.
	for _, altTLD := range commonTLDs {
		if altTLD != tld {
			add(sld+"."+altTLD, "tld_variation")
		}
	}

	// 8. Hyphenation — insert hyphen between each adjacent pair.
	runes3 := []rune(sld)
	for i := 1; i < len(runes3); i++ {
		newSLD := string(runes3[:i]) + "-" + string(runes3[i:])
		add(newSLD+"."+tld, "hyphenation")
	}

	// 9. Subdomain spoofing — prepend original domain as subdomain of common TLDs.
	for _, rootTLD := range []string{"com", "net", "org"} {
		if rootTLD != tld {
			add(sld+"."+tld+"."+rootTLD, "subdomain_spoof")
		}
	}

	return candidates, nil
}
