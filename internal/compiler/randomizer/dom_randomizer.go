// Package randomizer implements anti-fingerprinting randomization engines.
package randomizer

import (
	"fmt"
	"math/rand"
	"strings"
)

// LiveDOMRandomizer applies structural randomization to HTML to prevent fingerprinting.
type LiveDOMRandomizer struct {
	// WrapperTags defines the set of neutral container tags used for wrapping content.
	WrapperTags []string
	// MaxNestingDepth defines the maximum nesting depth for wrapper containers.
	MaxNestingDepth int
}

// NewLiveDOMRandomizer creates a new DOM randomizer with configurable settings.
func NewLiveDOMRandomizer(opts ...func(*LiveDOMRandomizer)) *LiveDOMRandomizer {
	r := &LiveDOMRandomizer{
		WrapperTags:     []string{"div", "section", "article", "main", "aside", "span"},
		MaxNestingDepth: 4,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// WithWrapperTags sets the neutral container tags used for wrapping content.
func WithWrapperTags(tags []string) func(*LiveDOMRandomizer) {
	return func(r *LiveDOMRandomizer) {
		r.WrapperTags = make([]string, len(tags))
		copy(r.WrapperTags, tags)
	}
}

// WithMaxNestingDepth sets the maximum nesting depth for wrapper containers.
func WithMaxNestingDepth(depth int) func(*LiveDOMRandomizer) {
	return func(r *LiveDOMRandomizer) {
		r.MaxNestingDepth = depth
	}
}

// RandomizeDOM applies structural randomization to HTML content.
// Returns the randomized HTML and a manifest of decisions made.
//
// The randomization includes:
//   - Neutral container wrapping with pseudo-random tag selection
//   - Attribute ordering randomization on elements
//   - Head tag ordering randomization
//   - Neutral wrapper insertion at random DOM positions
//   - Whitespace pattern variation
func (r *LiveDOMRandomizer) RandomizeDOM(html string, seed int64) (string, map[string]any, error) {
	// Create seeded random source for deterministic results
	src := rand.NewSource(seed)
	rd := rand.New(src)

	// Parse HTML into tokens
	tokens, err := tokenizeHTML(html)
	if err != nil {
		return "", nil, fmt.Errorf("randomizeDOM: tokenize HTML: %w", err)
	}

	// Build a DOM tree from tokens
	dom, err := buildDOM(tokens)
	if err != nil {
		return "", nil, fmt.Errorf("randomizeDOM: build DOM: %w", err)
	}

	// Track manifest data
	manifest := map[string]any{
		"strategy": "live",
	}

	// Apply transformations
	r.applyWrapperContainers(dom, rd, manifest)
	r.shuffleAttributes(dom, rd, manifest)
	r.shuffleHeadTags(dom, rd, manifest)
	r.insertNeutralWrappers(dom, rd, manifest)
	r.applyWhitespaceVariation(dom, rd, manifest)

	// Convert DOM back to HTML
	resultHTML, err := domToHTML(dom, rd)
	if err != nil {
		return "", nil, fmt.Errorf("randomizeDOM: convert to HTML: %w", err)
	}

	return resultHTML, manifest, nil
}

// wrapperContainer holds information about a wrapper container in the DOM.
type wrapperContainer struct {
	openTag      string
	closeTag     string
	nestingDepth int
}

// applyWrapperContainers wraps content in randomized neutral containers.
func (r *LiveDOMRandomizer) applyWrapperContainers(dom *domNode, rd *rand.Rand, manifest map[string]any) {
	wrappersAdded := 0
	maxDepth := 0

	if dom.children == nil {
		dom.children = []*domNode{}
	}

	// Ensure WrapperTags is initialized
	if r.WrapperTags == nil {
		r.WrapperTags = []string{"div", "section", "article", "main", "aside", "span"}
	}

	// Determine nesting depth (1-4)
	nestingDepth := 1 + int(rd.Intn(max(r.MaxNestingDepth, 1)))
	maxDepth = nestingDepth

	// Create wrapper chain
	chain := make([]wrapperContainer, nestingDepth)
	for i := 0; i < nestingDepth; i++ {
		tag := r.WrapperTags[rd.Intn(len(r.WrapperTags))]
		chain[i] = wrapperContainer{
			openTag:      fmt.Sprintf("<%s>", tag),
			closeTag:     fmt.Sprintf("</%s>", tag),
			nestingDepth: i + 1,
		}
	}

	// Wrap the content in a wrapper div if it's not already wrapped
	if len(dom.children) > 0 {
		// Add outer wrappers
		for i := len(chain) - 1; i >= 0; i-- {
			wrap := &domNode{
				tag:        chain[i].openTag[1 : len(chain[i].openTag)-1],
				attributes: []string{},
				children:   []*domNode{dom.children[0]},
			}
			dom.children[0] = wrap
			wrappersAdded++
		}
	}

	manifest["wrappers_added"] = wrappersAdded
	manifest["nesting_depth"] = maxDepth
}

// attributeInfo holds information about an element's attributes.
type attributeInfo struct {
	name  string
	value string
}

// shuffleAttributes randomizes the order of attributes on elements.
func (r *LiveDOMRandomizer) shuffleAttributes(dom *domNode, rd *rand.Rand, manifest map[string]any) {
	reorderCount := 0

	// Function to shuffle attributes for a node
	shuffleNodeAttributes := func(node *domNode) {
		if len(node.attributes) <= 1 {
			return
		}

		// Create a copy of attributes with indices
		attrs := make([]attributeInfo, len(node.attributes))
		for i, attr := range node.attributes {
			parts := strings.SplitN(attr, "=", 2)
			name := parts[0]
			value := ""
			if len(parts) > 1 {
				value = strings.Trim(parts[1], "\"'")
			}
			attrs[i] = attributeInfo{name: name, value: value}
		}

		// Shuffle using Fisher-Yates
		for i := len(attrs) - 1; i > 0; i-- {
			j := rd.Intn(i + 1)
			attrs[i], attrs[j] = attrs[j], attrs[i]
		}

		// Reconstruct attribute string
		newAttrs := make([]string, len(attrs))
		for i, attr := range attrs {
			if attr.value != "" {
				newAttrs[i] = fmt.Sprintf(`%s="%s"`, attr.name, attr.value)
			} else {
				newAttrs[i] = attr.name
			}
		}

		node.attributes = newAttrs
		reorderCount++
	}

	// Shuffle attributes for each node
	for _, node := range dom.traverse() {
		if node.tag != "" && !isTextNode(node) {
			shuffleNodeAttributes(node)
		}
	}

	manifest["attribute_reorders"] = reorderCount
}

// shuffleHeadTags randomizes the order of meta, link, and script tags in the head.
func (r *LiveDOMRandomizer) shuffleHeadTags(dom *domNode, rd *rand.Rand, manifest map[string]any) {
	if dom.children == nil {
		return
	}

	// Find the head section
	var head *domNode
	for _, child := range dom.children {
		if child.tag == "head" {
			head = child
			break
		}
	}

	if head == nil || head.children == nil {
		return
	}

	// Collect head child nodes to shuffle
	heads := make([]*domNode, 0, len(head.children))
	for _, child := range head.children {
		switch child.tag {
		case "meta", "link", "script":
			heads = append(heads, child)
		}
	}

	if len(heads) <= 1 {
		return
	}

	// Shuffle head tags using Fisher-Yates
	for i := len(heads) - 1; i > 0; i-- {
		j := rd.Intn(i + 1)
		heads[i], heads[j] = heads[j], heads[i]
	}

	// Rebuild head's children slice
	newChildren := make([]*domNode, 0, len(head.children))
	for _, child := range head.children {
		if child.tag == "meta" || child.tag == "link" || child.tag == "script" {
			continue
		}
		newChildren = append(newChildren, child)
	}

	// Insert shuffled head tags
	newChildren = append(newChildren, heads...)
	head.children = newChildren
}

// neutralWrapper holds information about an inserted wrapper.
type neutralWrapper struct {
	tag          string
	parentIndex  int
	insertIndex  int
	nestingLevel int
}

// insertNeutralWrappers inserts semantically neutral wrappers at random DOM positions.
func (r *LiveDOMRandomizer) insertNeutralWrappers(dom *domNode, rd *rand.Rand, manifest map[string]any) {
	wrappersAdded := 0
	seededWrappers := make([]neutralWrapper, 0)

	// Get all nodes that can have wrappers inserted
	nodes := dom.traverse()

	// Determine number of wrappers to insert (based on DOM size)
	nodeCount := len(nodes)
	wrapperCount := 1 + int(rd.Intn(min(10, max(5, nodeCount/10))))

	// Track visited indices to avoid duplicates
	visited := make(map[int]bool)

	for i := 0; i < wrapperCount; i++ {
		// Select a random node to wrap
		nodeIdx := rd.Intn(len(nodes))
		if visited[nodeIdx] {
			continue
		}
		visited[nodeIdx] = true

		node := nodes[nodeIdx]
		if node.tag == "" || isTextNode(node) {
			continue
		}

		// Create wrapper tag
		wrapperTag := r.WrapperTags[rd.Intn(len(r.WrapperTags))]
		wrapper := neutralWrapper{
			tag:         wrapperTag,
			parentIndex: node.parentIndex,
			insertIndex: node.insertIndex,
		}
		seededWrappers = append(seededWrappers, wrapper)

		// Increment counter
		wrappersAdded++
	}

	manifest["neutral_wrappers_added"] = wrappersAdded
	// Convert to []any for JSON serialization compatibility
	manifest["neutral_wrappers"] = convertWrappersToAny(seededWrappers)
}

// convertWrappersToAny converts neutralWrapper slice to []any for manifest storage.
func convertWrappersToAny(wrappers []neutralWrapper) []any {
	result := make([]any, len(wrappers))
	for i, w := range wrappers {
		result[i] = map[string]any{
			"tag":           w.tag,
			"parent_index":  w.parentIndex,
			"insert_index":  w.insertIndex,
			"nesting_level": w.nestingLevel,
		}
	}
	return result
}

// whitespaceConfig holds the chosen whitespace settings for HTML output.
type whitespaceConfig struct {
	indent     string
	lineEnding string
	spacing    string
}

// applyWhitespaceVariation selects whitespace patterns and stores them on the DOM root.
func (r *LiveDOMRandomizer) applyWhitespaceVariation(dom *domNode, rd *rand.Rand, manifest map[string]any) {
	indentStyles := []string{"  ", "    ", "\t", "  \t"}
	lineEndings := []string{"\n", "\r\n"}
	spacingStyles := []string{"compact", "balanced", "spacious"}

	cfg := whitespaceConfig{
		indent:     indentStyles[rd.Intn(len(indentStyles))],
		lineEnding: lineEndings[rd.Intn(len(lineEndings))],
		spacing:    spacingStyles[rd.Intn(len(spacingStyles))],
	}

	// Store on root node so domToHTML can use it
	dom.wsConfig = &cfg

	manifest["whitespace_variant"] = map[string]any{
		"indent_style": cfg.indent,
		"line_ending":  cfg.lineEnding,
		"spacing":      cfg.spacing,
	}
}

// domNode represents a node in the parsed HTML DOM.
type domNode struct {
	tag         string
	attributes  []string
	children    []*domNode
	textContent string
	parentIndex int
	insertIndex int
	parent      *domNode
	wsConfig    *whitespaceConfig
}

// isTextNode checks if a node is a text node.
func isTextNode(node *domNode) bool {
	return node.tag == "" && node.textContent != ""
}

// traverse returns all nodes in the DOM tree in traversal order.
func (n *domNode) traverse() []*domNode {
	var result []*domNode
	var traverse func(*domNode)
	traverse = func(node *domNode) {
		result = append(result, node)
		if node.children != nil {
			for _, child := range node.children {
				traverse(child)
			}
		}
	}
	traverse(n)
	return result
}

// tokenizeHTML tokenizes an HTML string into a stream of tokens.
func tokenizeHTML(html string) ([]string, error) {
	var tokens []string
	i := 0
	n := len(html)

	for i < n {
		// Skip whitespace
		for i < n && (html[i] == ' ' || html[i] == '\t' || html[i] == '\n' || html[i] == '\r') {
			i++
		}
		if i >= n {
			break
		}

		// Check for tag
		if html[i] == '<' {
			// Find tag end
			end := i + 1
			for end < n && html[end] != '>' {
				if html[end] == '<' {
					break
				}
				end++
			}
			if end < n {
				end++
			}
			tokens = append(tokens, html[i:end])
			i = end
			continue
		}

		// Text content
		start := i
		for i < n && html[i] != '<' {
			i++
		}
		text := strings.TrimSpace(html[start:i])
		if text != "" {
			tokens = append(tokens, text)
		}
	}

	return tokens, nil
}

// buildDOM builds a DOM tree from a token stream.
func buildDOM(tokens []string) (*domNode, error) {
	if len(tokens) == 0 {
		return &domNode{}, nil
	}

	// Create root node
	root := &domNode{tag: "root"}

	// Stack to track parent nodes
	stack := []*domNode{root}

	// Track insert indices
	insertIndex := 0

	for _, token := range tokens {
		if strings.HasPrefix(token, "<") {
			if strings.HasPrefix(token, "</") {
				// Closing tag
				tagName := strings.TrimSuffix(strings.TrimPrefix(token, "</"), ">")
				if len(stack) > 1 {
					// Pop stack until we find matching tag
					for len(stack) > 1 && stack[len(stack)-1].tag != tagName {
						stack = stack[:len(stack)-1]
					}
					if len(stack) > 1 {
						stack = stack[:len(stack)-1]
					}
				}
			} else {
				// Opening tag or self-closing
				tag, attrs := parseTag(token)

				node := &domNode{
					tag:         tag,
					attributes:  attrs,
					children:    make([]*domNode, 0),
					parentIndex: len(stack) - 1,
					insertIndex: insertIndex,
				}
				insertIndex++

				// Add to parent
				parent := stack[len(stack)-1]
				if parent.children == nil {
					parent.children = make([]*domNode, 0)
				}
				parent.children = append(parent.children, node)
				node.parent = parent

				// Push to stack if not self-closing
				if !isSelfClosing(tag) {
					stack = append(stack, node)
				}
			}
		} else {
			// Text content
			parent := stack[len(stack)-1]
			if parent.children == nil {
				parent.children = make([]*domNode, 0)
			}
			parent.children = append(parent.children, &domNode{
				tag:         "",
				attributes:  []string{},
				textContent: token,
				parentIndex: len(stack) - 1,
				insertIndex: insertIndex,
			})
			insertIndex++
		}
	}

	return root, nil
}

// parseTag parses an HTML tag string and returns the tag name and attributes.
func parseTag(tagStr string) (string, []string) {
	// Remove leading < and trailing >
	content := strings.TrimPrefix(tagStr, "<")
	content = strings.TrimSuffix(content, ">")

	// Handle self-closing tags
	content = strings.TrimSuffix(content, "/")

	// Split into tag name and attributes
	parts := strings.SplitN(content, " ", 2)
	tagName := parts[0]
	attrs := make([]string, 0)

	if len(parts) > 1 {
		attrStr := parts[1]
		// Parse attributes
		// Split by spaces, but respect quotes
		i := 0
		n := len(attrStr)
		for i < n {
			// Skip whitespace
			for i < n && (attrStr[i] == ' ' || attrStr[i] == '\t' || attrStr[i] == '\n') {
				i++
			}
			if i >= n {
				break
			}

			// Find attribute end
			start := i
			quoted := false
			for i < n {
				c := attrStr[i]
				if c == '"' || c == '\'' {
					quoted = !quoted
				} else if !quoted && (c == ' ' || c == '\t' || c == '\n') {
					break
				}
				i++
			}
			attrs = append(attrs, attrStr[start:i])
		}
	}

	return tagName, attrs
}

// isSelfClosing checks if a tag is a void element (HTML spec void elements only).
func isSelfClosing(tag string) bool {
	selfClosing := map[string]bool{
		"area":   true,
		"base":   true,
		"br":     true,
		"col":    true,
		"embed":  true,
		"hr":     true,
		"img":    true,
		"input":  true,
		"link":   true,
		"meta":   true,
		"param":  true,
		"source": true,
		"track":  true,
		"wbr":    true,
	}
	return selfClosing[tag]
}

// domToHTML converts a DOM tree back to an HTML string.
func domToHTML(dom *domNode, rd *rand.Rand) (string, error) {
	var sb strings.Builder

	// Use whitespace config from root node, or defaults
	indentUnit := "  "
	newline := "\n"
	if dom.wsConfig != nil {
		indentUnit = dom.wsConfig.indent
		newline = dom.wsConfig.lineEnding
	}

	var writeNode func(*domNode, int)
	writeNode = func(node *domNode, indent int) {
		indentStr := strings.Repeat(indentUnit, indent)

		if node.tag == "" {
			// Text node
			if node.textContent != "" {
				sb.WriteString(node.textContent)
			}
			return
		}

		// Opening tag
		sb.WriteString(indentStr)
		sb.WriteString("<")
		sb.WriteString(node.tag)

		// Write attributes
		for _, attr := range node.attributes {
			sb.WriteString(" ")
			sb.WriteString(attr)
		}

		// Only void elements are self-closing
		if isSelfClosing(node.tag) {
			sb.WriteString(" />")
		} else {
			sb.WriteString(">")
			sb.WriteString(newline)

			// Write children
			if node.children != nil {
				for _, child := range node.children {
					writeNode(child, indent+1)
				}
			}

			// Write closing tag
			sb.WriteString(indentStr)
			sb.WriteString("</")
			sb.WriteString(node.tag)
			sb.WriteString(">")
		}
		sb.WriteString(newline)
	}

	// Write children
	for _, child := range dom.children {
		if child.tag != "root" {
			writeNode(child, 0)
		}
	}

	return sb.String(), nil
}

