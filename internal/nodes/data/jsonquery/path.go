package jsonquery

// path.go implements the JSONPath engine behind the Query JSON node.
// Expressions follow the classic Goessner JSONPath dialect: `$` roots the
// connected source, `.name` and `['name']` descend into object keys, `[n]`
// indexes lists, and wildcards, slices, unions, recursive descent, and
// `[?(...)]` filters select several values at once. The evaluator walks
// graph-safe values (maps, slices, structs) without re-encoding to JSON, so
// it also understands first-party structs and their json tags.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Query resolves a path against a source value. An empty path returns the
// source itself. Paths rooted with `$` (and any path that contains brackets
// or `..`) are evaluated as JSONPath; plain dotted paths keep the simple
// key.key.index behaviour. A path that cannot
// be parsed yields nil rather than an error: an unknown location is a null
// pick, the same contract ValueAt always had.
func Query(value any, path string) any {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return value
	}
	if trimmed[0] == '$' || strings.Contains(trimmed, "[") || strings.Contains(trimmed, "..") {
		segments, err := parseJSONPath(trimmed)
		if err != nil {
			return nil
		}
		result, _ := evaluateSegments(value, segments)
		return result
	}
	return ValueAt(value, trimmed)
}

/* ------------------------------------------------------------------ */
/* Segments                                                            */
/* ------------------------------------------------------------------ */

type segmentKind int

const (
	segmentName      segmentKind = iota // .name or ['name']
	segmentIndex                        // [0], [-2]
	segmentWildcard                     // .* or [*]
	segmentSlice                        // [start:end:step]
	segmentUnion                        // [0,2] or ['a','b']
	segmentFilter                       // [?(...)]
	segmentRecursive                    // .. (every descendant, self included)
)

type segment struct {
	kind      segmentKind
	name      string
	index     int
	start     int
	end       int
	step      int
	hasStart  bool
	hasEnd    bool
	members   []segment // union members: name and index segments only
	predicate filterExpr
}

// apply appends every value the segment selects from node to out.
func (s segment) apply(node, root any, out *[]any) {
	switch s.kind {
	case segmentName:
		if value, found := objectValueAt(node, s.name); found {
			*out = append(*out, value)
		}
	case segmentIndex:
		if value, found := indexValueAt(node, s.index); found {
			*out = append(*out, value)
		}
	case segmentWildcard:
		if children, ok := childValues(node); ok {
			*out = append(*out, children...)
		}
	case segmentSlice:
		if elements, ok := listElements(node); ok {
			for _, index := range sliceIndices(len(elements), s) {
				*out = append(*out, elements[index])
			}
		}
	case segmentUnion:
		for _, member := range s.members {
			member.apply(node, root, out)
		}
	case segmentFilter:
		if children, ok := childValues(node); ok {
			for _, child := range children {
				if s.predicate.matches(child, root) {
					*out = append(*out, child)
				}
			}
		}
	case segmentRecursive:
		collectDescendants(node, out)
	}
}

// evaluateSegments walks the segments across every candidate node. Exactly
// one match returns the value itself, several matches return a list, and no
// match reports found=false so filter operands can tell "picked null" from
// "picked nothing".
func evaluateSegments(root any, segments []segment) (any, bool) {
	current := []any{root}
	for _, seg := range segments {
		next := make([]any, 0, len(current))
		for _, node := range current {
			seg.apply(node, root, &next)
		}
		current = next
	}
	switch len(current) {
	case 0:
		return nil, false
	case 1:
		return current[0], true
	default:
		return current, true
	}
}

// indexValueAt resolves one list index, negative indices counting from the
// end like Python's.
func indexValueAt(value any, index int) (any, bool) {
	elements, ok := listElements(value)
	if !ok {
		return nil, false
	}
	if index < 0 {
		index += len(elements)
	}
	if index < 0 || index >= len(elements) {
		return nil, false
	}
	return elements[index], true
}

// listElements exposes slices and arrays only: wildcards and filters also
// visit object values, but indexes and slices are list operations.
func listElements(value any) ([]any, bool) {
	resolved := dereference(reflect.ValueOf(value))
	if !resolved.IsValid() || (resolved.Kind() != reflect.Slice && resolved.Kind() != reflect.Array) {
		return nil, false
	}
	elements := make([]any, resolved.Len())
	for index := range elements {
		item := resolved.Index(index)
		if !item.CanInterface() {
			return nil, false
		}
		elements[index] = item.Interface()
	}
	return elements, true
}

// childValues lists the children a wildcard or filter iterates: list
// elements in order, object values sorted by key (Go map iteration is
// randomized, so sorting keeps multi-match output deterministic), and
// exported struct fields in declaration order.
func childValues(value any) ([]any, bool) {
	resolved := dereference(reflect.ValueOf(value))
	if !resolved.IsValid() {
		return nil, false
	}
	switch resolved.Kind() {
	case reflect.Slice, reflect.Array:
		return listElements(value)
	case reflect.Map:
		if resolved.Type().Key().Kind() != reflect.String {
			return nil, false
		}
		keys := make([]string, 0, resolved.Len())
		for _, key := range resolved.MapKeys() {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		items := make([]any, 0, len(keys))
		for _, key := range keys {
			item := resolved.MapIndex(reflect.ValueOf(key).Convert(resolved.Type().Key()))
			if item.IsValid() && item.CanInterface() {
				items = append(items, item.Interface())
			}
		}
		return items, true
	case reflect.Struct:
		items := make([]any, 0, resolved.NumField())
		for index := 0; index < resolved.NumField(); index++ {
			if resolved.Type().Field(index).PkgPath != "" {
				continue
			}
			item := resolved.Field(index)
			if item.CanInterface() {
				items = append(items, item.Interface())
			}
		}
		return items, true
	}
	return nil, false
}

// collectDescendants appends node and every nested value beneath it,
// depth-first with sorted object keys.
func collectDescendants(value any, out *[]any) {
	*out = append(*out, value)
	if children, ok := childValues(value); ok {
		for _, child := range children {
			collectDescendants(child, out)
		}
	}
}

// sliceIndices implements Python-style slice bounds: omitted bounds pick the
// defaults for the step's direction, negatives count from the end, and
// out-of-range bounds clamp.
func sliceIndices(length int, s segment) []int {
	step := s.step
	if step == 0 {
		return nil
	}
	var indices []int
	if step > 0 {
		start, end := 0, length
		if s.hasStart {
			start = s.start
		}
		if s.hasEnd {
			end = s.end
		}
		if start < 0 {
			start += length
		}
		if end < 0 {
			end += length
		}
		start = clampInt(start, 0, length)
		end = clampInt(end, 0, length)
		for index := start; index < end; index += step {
			indices = append(indices, index)
		}
		return indices
	}
	start, end := length-1, -1
	if s.hasStart {
		start = s.start
	}
	if s.hasEnd {
		end = s.end
	}
	if start < 0 {
		start += length
	}
	if s.hasEnd && end < 0 {
		end += length
	}
	start = clampInt(start, -1, length-1)
	end = clampInt(end, -1, length-1)
	for index := start; index > end; index += step {
		indices = append(indices, index)
	}
	return indices
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

/* ------------------------------------------------------------------ */
/* Parser                                                              */
/* ------------------------------------------------------------------ */

type pathParser struct {
	input string
	pos   int
}

func parseJSONPath(path string) ([]segment, error) {
	parser := &pathParser{input: path}
	parser.skipSpace()
	if parser.pos < len(parser.input) && parser.input[parser.pos] == '$' {
		parser.pos++
	}
	segments, err := parser.parseSegments(true)
	if err != nil {
		return nil, err
	}
	parser.skipSpace()
	if parser.pos != len(parser.input) {
		return nil, fmt.Errorf("unexpected %q at position %d", parser.input[parser.pos:], parser.pos)
	}
	return segments, nil
}

// parseSegments consumes `.name`, `..`, and `[...]` selectors until the
// input ends. topLevel also accepts one leading bare name so implicit-root
// paths such as `geonames[0].lng` parse without `$`. Filter sub-paths
// (topLevel=false) instead stop at the first character that is not a
// selector, leaving operators to the filter parser.
func (p *pathParser) parseSegments(topLevel bool) ([]segment, error) {
	segments := make([]segment, 0, 4)
	for {
		p.skipSpace()
		if p.pos >= len(p.input) {
			return segments, nil
		}
		switch p.input[p.pos] {
		case '.':
			parsed, err := p.parseDot()
			if err != nil {
				return nil, err
			}
			segments = append(segments, parsed...)
		case '[':
			parsed, err := p.parseBracket()
			if err != nil {
				return nil, err
			}
			segments = append(segments, parsed...)
		default:
			if !topLevel {
				return segments, nil
			}
			if len(segments) > 0 {
				return nil, fmt.Errorf("unexpected %q at position %d", p.input[p.pos:], p.pos)
			}
			name, err := p.parseName()
			if err != nil {
				return nil, err
			}
			segments = append(segments, segment{kind: segmentName, name: name})
		}
	}
}

// parseDot handles `.name`, `.*`, `..name`, `..*`, and a bare `..` followed
// by a bracket. Recursive descent expands to two segments: the descendant
// walk itself, then the selector that follows it.
func (p *pathParser) parseDot() ([]segment, error) {
	if strings.HasPrefix(p.input[p.pos:], "..") {
		p.pos += 2
		p.skipSpace()
		if p.pos < len(p.input) && p.input[p.pos] == '[' {
			return []segment{{kind: segmentRecursive}}, nil
		}
		if p.pos < len(p.input) && p.input[p.pos] == '*' {
			p.pos++
			return []segment{{kind: segmentRecursive}, {kind: segmentWildcard}}, nil
		}
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		return []segment{{kind: segmentRecursive}, {kind: segmentName, name: name}}, nil
	}
	p.pos++ // consume '.'
	if p.pos < len(p.input) && p.input[p.pos] == '*' {
		p.pos++
		return []segment{{kind: segmentWildcard}}, nil
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	return []segment{{kind: segmentName, name: name}}, nil
}

// parseName reads an unescaped object key: everything up to the next
// structural character. An empty name is a syntax error so malformed paths
// like `$.a.` fail loudly instead of matching an empty key.
func (p *pathParser) parseName() (string, error) {
	start := p.pos
	for p.pos < len(p.input) {
		switch p.input[p.pos] {
		case '.', '[', ']', '*', '(', ')', ' ', '\t', '\n', '\r':
			if p.pos == start {
				return "", fmt.Errorf("expected a name at position %d", start)
			}
			return p.input[start:p.pos], nil
		}
		p.pos++
	}
	if p.pos == start {
		return "", fmt.Errorf("expected a name at position %d", start)
	}
	return p.input[start:p.pos], nil
}

// parseBracket handles every bracket selector: wildcard, filter, quoted
// names, indexes, slices, and unions of the former.
func (p *pathParser) parseBracket() ([]segment, error) {
	p.pos++ // consume '['
	p.skipSpace()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unterminated '[' in path")
	}
	switch p.input[p.pos] {
	case '?':
		return p.parseFilterBracket()
	case '*':
		p.pos++
		p.skipSpace()
		if err := p.expect(']'); err != nil {
			return nil, err
		}
		return []segment{{kind: segmentWildcard}}, nil
	}
	return p.parseSelectorBracket()
}

// parseSelectorBracket parses the bracket body as comma-separated selectors
// (quoted names or integers) with optional `:` slice separators, then folds
// the tokens into one segment.
func (p *pathParser) parseSelectorBracket() ([]segment, error) {
	var tokens []selectorToken
	for {
		p.skipSpace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unterminated '[' in path")
		}
		switch c := p.input[p.pos]; {
		case c == ']':
			p.pos++
			return selectorTokensToSegment(tokens)
		case c == ',':
			p.pos++
		case c == ':':
			p.pos++
			tokens = append(tokens, selectorToken{kind: tokenColon})
		case c == '\'' || c == '"':
			name, err := p.parseQuoted()
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, selectorToken{kind: tokenName, name: name})
		case c == '-' || c == '+' || (c >= '0' && c <= '9'):
			index, err := p.parseInt()
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, selectorToken{kind: tokenIndex, index: index})
		default:
			return nil, fmt.Errorf("unexpected %q inside brackets at position %d", string(c), p.pos)
		}
	}
}

type tokenKind int

const (
	tokenName tokenKind = iota
	tokenIndex
	tokenColon
)

type selectorToken struct {
	kind  tokenKind
	name  string
	index int
}

// selectorTokensToSegment folds parsed tokens into a single segment: one
// plain selector stays itself, several form a union, and colon-separated
// numbers form a slice.
func selectorTokensToSegment(tokens []selectorToken) ([]segment, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty brackets")
	}
	colons := 0
	for _, token := range tokens {
		if token.kind == tokenColon {
			colons++
		}
	}
	if colons > 0 {
		slice, err := tokensToSlice(tokens)
		if err != nil {
			return nil, err
		}
		return []segment{slice}, nil
	}
	if len(tokens) == 1 {
		return []segment{tokens[0].toSegment()}, nil
	}
	members := make([]segment, 0, len(tokens))
	for _, token := range tokens {
		if token.kind == tokenColon {
			return nil, fmt.Errorf("unexpected ':' in selector")
		}
		members = append(members, token.toSegment())
	}
	return []segment{{kind: segmentUnion, members: members}}, nil
}

func (t selectorToken) toSegment() segment {
	if t.kind == tokenName {
		return segment{kind: segmentName, name: t.name}
	}
	return segment{kind: segmentIndex, index: t.index}
}

// tokensToSlice rebuilds [start:end:step] with omitted parts left at their
// defaults.
func tokensToSlice(tokens []selectorToken) (segment, error) {
	result := segment{kind: segmentSlice, step: 1}
	groups := splitBeforeColon(tokens)
	if len(groups) < 2 || len(groups) > 3 {
		return segment{}, fmt.Errorf("a slice needs one or two colons")
	}
	if err := readSliceBound(groups[0], "slice start", &result.start, &result.hasStart); err != nil {
		return segment{}, err
	}
	if err := readSliceBound(groups[1], "slice end", &result.end, &result.hasEnd); err != nil {
		return segment{}, err
	}
	if len(groups) == 3 {
		if len(groups[2]) != 1 {
			return segment{}, fmt.Errorf("slice step must be a single number")
		}
		result.step = groups[2][0].index
		if result.step == 0 {
			return segment{}, fmt.Errorf("slice step cannot be zero")
		}
	}
	return result, nil
}

// splitBeforeColon groups tokens into the colon-separated slot lists.
func splitBeforeColon(tokens []selectorToken) [][]selectorToken {
	var groups [][]selectorToken
	current := make([]selectorToken, 0, 2)
	for _, token := range tokens {
		if token.kind == tokenColon {
			groups = append(groups, current)
			current = make([]selectorToken, 0, 2)
			continue
		}
		current = append(current, token)
	}
	return append(groups, current)
}

func readSliceBound(tokens []selectorToken, label string, target *int, has *bool) error {
	switch len(tokens) {
	case 0:
		*has = false
		return nil
	case 1:
		if tokens[0].kind != tokenIndex {
			return fmt.Errorf("%s must be a number", label)
		}
		*target = tokens[0].index
		*has = true
		return nil
	default:
		return fmt.Errorf("%s must be a single number", label)
	}
}

// parseInt reads a signed integer token.
func (p *pathParser) parseInt() (int, error) {
	start := p.pos
	if p.pos < len(p.input) && (p.input[p.pos] == '-' || p.input[p.pos] == '+') {
		p.pos++
	}
	digits := 0
	for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
		p.pos++
		digits++
	}
	if digits == 0 {
		return 0, fmt.Errorf("expected a number at position %d", start)
	}
	value, err := strconv.Atoi(p.input[start:p.pos])
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", p.input[start:p.pos])
	}
	return value, nil
}

// parseQuoted reads a single- or double-quoted name, honouring backslash
// escapes for the quotes and the backslash itself.
func (p *pathParser) parseQuoted() (string, error) {
	quote := p.input[p.pos]
	p.pos++
	var builder strings.Builder
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		switch {
		case c == '\\' && p.pos+1 < len(p.input):
			builder.WriteByte(p.input[p.pos+1])
			p.pos += 2
		case c == quote:
			p.pos++
			return builder.String(), nil
		default:
			builder.WriteByte(c)
			p.pos++
		}
	}
	return "", fmt.Errorf("unterminated quoted name")
}

func (p *pathParser) expect(c byte) error {
	if p.pos >= len(p.input) || p.input[p.pos] != c {
		return fmt.Errorf("expected %q at position %d", string(c), p.pos)
	}
	p.pos++
	return nil
}

func (p *pathParser) skipSpace() {
	for p.pos < len(p.input) {
		switch p.input[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

/* ------------------------------------------------------------------ */
/* Filter expressions                                                  */
/* ------------------------------------------------------------------ */

// filterExpr is the predicate of one [?(...)] selector. It sees every child
// of the filtered node as @ and the whole source as $.
type filterExpr interface {
	matches(node, root any) bool
}

type filterOr struct{ operands []filterExpr }

func (f filterOr) matches(node, root any) bool {
	for _, operand := range f.operands {
		if operand.matches(node, root) {
			return true
		}
	}
	return false
}

type filterAnd struct{ operands []filterExpr }

func (f filterAnd) matches(node, root any) bool {
	for _, operand := range f.operands {
		if !operand.matches(node, root) {
			return false
		}
	}
	return true
}

type filterNot struct{ operand filterExpr }

func (f filterNot) matches(node, root any) bool { return !f.operand.matches(node, root) }

type filterTruthy struct{ operand filterOperand }

func (f filterTruthy) matches(node, root any) bool {
	value, found := f.operand.resolve(node, root)
	return found && truthy(value)
}

type filterComparison struct {
	left  filterOperand
	op    string
	right filterOperand
}

func (f filterComparison) matches(node, root any) bool {
	left, leftFound := f.left.resolve(node, root)
	right, rightFound := f.right.resolve(node, root)
	if !leftFound || !rightFound {
		bothMissing := !leftFound && !rightFound
		switch f.op {
		case "==":
			return bothMissing
		case "!=":
			return !bothMissing
		default:
			return false
		}
	}
	switch f.op {
	case "==":
		return valuesEqual(left, right)
	case "!=":
		return !valuesEqual(left, right)
	default:
		return valuesOrdered(left, right, f.op)
	}
}

// filterOperand is one side of a filter comparison: a literal, or a path
// relative to @ or rooted at $.
type filterOperand interface {
	resolve(node, root any) (value any, found bool)
}

type literalOperand struct{ value any }

func (o literalOperand) resolve(_, _ any) (any, bool) { return o.value, true }

type pathOperand struct {
	segments []segment
	fromRoot bool
}

func (o pathOperand) resolve(node, root any) (any, bool) {
	base := node
	if o.fromRoot {
		base = root
	}
	return evaluateSegments(base, o.segments)
}

// parseFilterBracket parses [?(predicate)] after the leading '?'.
func (p *pathParser) parseFilterBracket() ([]segment, error) {
	p.pos++ // consume '?'
	p.skipSpace()
	if err := p.expect('('); err != nil {
		return nil, err
	}
	predicate, err := p.parseFilterOr()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if err := p.expect(')'); err != nil {
		return nil, err
	}
	p.skipSpace()
	if err := p.expect(']'); err != nil {
		return nil, err
	}
	return []segment{{kind: segmentFilter, predicate: predicate}}, nil
}

func (p *pathParser) parseFilterOr() (filterExpr, error) {
	first, err := p.parseFilterAnd()
	if err != nil {
		return nil, err
	}
	operands := []filterExpr{first}
	for {
		p.skipSpace()
		if !strings.HasPrefix(p.input[p.pos:], "||") {
			break
		}
		p.pos += 2
		next, err := p.parseFilterAnd()
		if err != nil {
			return nil, err
		}
		operands = append(operands, next)
	}
	if len(operands) == 1 {
		return operands[0], nil
	}
	return filterOr{operands: operands}, nil
}

func (p *pathParser) parseFilterAnd() (filterExpr, error) {
	first, err := p.parseFilterUnary()
	if err != nil {
		return nil, err
	}
	operands := []filterExpr{first}
	for {
		p.skipSpace()
		if !strings.HasPrefix(p.input[p.pos:], "&&") {
			break
		}
		p.pos += 2
		next, err := p.parseFilterUnary()
		if err != nil {
			return nil, err
		}
		operands = append(operands, next)
	}
	if len(operands) == 1 {
		return operands[0], nil
	}
	return filterAnd{operands: operands}, nil
}

func (p *pathParser) parseFilterUnary() (filterExpr, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("filter expression ended unexpectedly")
	}
	switch p.input[p.pos] {
	case '!':
		if p.pos+1 < len(p.input) && p.input[p.pos+1] == '=' {
			return nil, fmt.Errorf("expected an operand before != at position %d", p.pos)
		}
		p.pos++
		operand, err := p.parseFilterUnary()
		if err != nil {
			return nil, err
		}
		return filterNot{operand: operand}, nil
	case '(':
		p.pos++
		inner, err := p.parseFilterOr()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if err := p.expect(')'); err != nil {
			return nil, err
		}
		return inner, nil
	}
	return p.parseFilterComparison()
}

func (p *pathParser) parseFilterComparison() (filterExpr, error) {
	left, err := p.parseFilterOperand()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	op := ""
	for _, candidate := range []string{"==", "!=", "<=", ">=", "<", ">"} {
		if strings.HasPrefix(p.input[p.pos:], candidate) {
			op = candidate
			p.pos += len(candidate)
			break
		}
	}
	if op == "" {
		return filterTruthy{operand: left}, nil
	}
	right, err := p.parseFilterOperand()
	if err != nil {
		return nil, err
	}
	return filterComparison{left: left, op: op, right: right}, nil
}

// parseFilterOperand reads @, @.path, $path, or a literal (number, quoted
// string, true, false, null).
func (p *pathParser) parseFilterOperand() (filterOperand, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("filter operand ended unexpectedly")
	}
	switch c := p.input[p.pos]; {
	case c == '@' || c == '$':
		p.pos++
		segments, err := p.parseSegments(false)
		if err != nil {
			return nil, err
		}
		return pathOperand{segments: segments, fromRoot: c == '$'}, nil
	case c == '\'' || c == '"':
		text, err := p.parseQuoted()
		if err != nil {
			return nil, err
		}
		return literalOperand{value: text}, nil
	case strings.HasPrefix(p.input[p.pos:], "true"):
		p.pos += len("true")
		return literalOperand{value: true}, nil
	case strings.HasPrefix(p.input[p.pos:], "false"):
		p.pos += len("false")
		return literalOperand{value: false}, nil
	case strings.HasPrefix(p.input[p.pos:], "null"):
		p.pos += len("null")
		return literalOperand{value: nil}, nil
	default:
		return p.parseNumberLiteral()
	}
}

func (p *pathParser) parseNumberLiteral() (filterOperand, error) {
	start := p.pos
	if p.pos < len(p.input) && (p.input[p.pos] == '-' || p.input[p.pos] == '+') {
		p.pos++
	}
	digits := 0
	for p.pos < len(p.input) && ((p.input[p.pos] >= '0' && p.input[p.pos] <= '9') || p.input[p.pos] == '.') {
		p.pos++
		digits++
	}
	if digits == 0 {
		return nil, fmt.Errorf("expected a filter operand at position %d", start)
	}
	value, err := strconv.ParseFloat(p.input[start:p.pos], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number %q", p.input[start:p.pos])
	}
	return literalOperand{value: value}, nil
}

/* ------------------------------------------------------------------ */
/* Comparison semantics                                                */
/* ------------------------------------------------------------------ */

// truthy mirrors JavaScript's filter truthiness: null, false, zero, and the
// empty string drop out, everything else (including empty lists and
// objects) stays.
func truthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	case float64:
		return typed != 0
	case float32:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case json.Number:
		number, err := typed.Float64()
		return err == nil && number != 0
	}
	return true
}

func valuesEqual(left, right any) bool {
	if leftNumber, ok := asFloat(left); ok {
		if rightNumber, ok := asFloat(right); ok {
			return leftNumber == rightNumber
		}
		return false
	}
	return reflect.DeepEqual(left, right)
}

// valuesOrdered applies <, <=, >, and >= with numeric comparison when both
// sides are numbers and lexicographic comparison when both sides are text.
func valuesOrdered(left, right any, op string) bool {
	if leftNumber, ok := asFloat(left); ok {
		rightNumber, ok := asFloat(right)
		if !ok {
			return false
		}
		switch op {
		case "<":
			return leftNumber < rightNumber
		case "<=":
			return leftNumber <= rightNumber
		case ">":
			return leftNumber > rightNumber
		case ">=":
			return leftNumber >= rightNumber
		}
	}
	leftText, leftOK := left.(string)
	rightText, rightOK := right.(string)
	if !leftOK || !rightOK {
		return false
	}
	comparison := strings.Compare(leftText, rightText)
	switch op {
	case "<":
		return comparison < 0
	case "<=":
		return comparison <= 0
	case ">":
		return comparison > 0
	case ">=":
		return comparison >= 0
	}
	return false
}

// asFloat coerces every numeric representation JSON pipelines carry.
func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	}
	return 0, false
}
