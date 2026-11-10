package rules

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"isogloss/engine/phon"
)

// Error is a parse error with a source position.
type Error struct {
	Line, Col int
	Msg       string
}

func (e *Error) Error() string {
	return "line " + strconv.Itoa(e.Line) + ":" + strconv.Itoa(e.Col) + ": " + e.Msg
}

type parser struct {
	classes map[string]*Class
	file    *File
	line    int
}

// Parse parses a rule file.
func Parse(src string) (*File, error) { return ParseWith(src, nil) }

// ParseWith parses src with the classes of base (a prelude) predefined.
// The returned file lists only the classes src itself declares.
func ParseWith(src string, base *File) (*File, error) {
	p := &parser{classes: map[string]*Class{}, file: &File{}}
	p.predefine("C", "[-syl]")
	p.predefine("V", "[+syl]")
	if base != nil {
		for _, c := range base.Classes {
			p.classes[c.Name] = c
		}
	}
	var cur *Rule
	var last *Clause
	for i, raw := range strings.Split(src, "\n") {
		p.line = i + 1
		line := strings.TrimRight(stripComment(raw), " \t\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		indented := line[0] == ' ' || line[0] == '\t'
		text := strings.TrimSpace(line)
		col := strings.Index(line, text) + 1
		switch {
		case !indented && strings.HasPrefix(text, "class "):
			cur, last = nil, nil
			if err := p.parseClass(text[len("class "):], col+len("class ")); err != nil {
				return nil, err
			}
		case !indented && strings.HasPrefix(text, "rule "):
			r, err := p.parseHeader(text[len("rule "):], col)
			if err != nil {
				return nil, err
			}
			p.file.Rules = append(p.file.Rules, r)
			cur, last = r, nil
		case indented && cur != nil && strings.HasPrefix(text, "except"):
			if last == nil {
				return nil, p.errf(col, "except before any clause")
			}
			rest := strings.TrimSpace(text[len("except"):])
			if !strings.HasPrefix(rest, "/") {
				return nil, p.errf(col, "expected 'except / ENV'")
			}
			envs, err := p.parseEnvs(rest[1:], col)
			if err != nil {
				return nil, err
			}
			last.Except = append(last.Except, envs...)
		case indented && cur != nil:
			c, err := p.parseClause(text, col)
			if err != nil {
				return nil, err
			}
			cur.Clauses = append(cur.Clauses, c)
			last = c
		default:
			return nil, p.errf(col, "expected 'class', 'rule' or an indented clause")
		}
	}
	for _, r := range p.file.Rules {
		if len(r.Clauses) == 0 {
			return nil, &Error{r.Line, 1, "rule " + r.Name + " has no clauses"}
		}
	}
	return p.file, nil
}

// MustParse parses built-in rule text and panics on error.
func MustParse(src string) *File {
	f, err := Parse(src)
	if err != nil {
		panic(err)
	}
	return f
}

func (p *parser) errf(col int, msg string) error { return &Error{p.line, col, msg} }

func (p *parser) predefine(name, src string) {
	pat, err := p.parseMatrix(src[1:len(src)-1], 0)
	if err != nil {
		panic(err)
	}
	pat.Src = src
	p.classes[name] = &Class{Name: name, Pat: pat}
}

// stripComment removes a '#' comment while keeping '#' word boundaries.
// '#' starts a comment at the start of a line or anywhere before the
// environment slash. Inside an environment, a '#' before the target marker
// '_' is always a boundary (# _ V), also in a later environment of a
// list (V _ C, # _ V). After '_', a '#' is a boundary when it
// ends the line or is followed by another '#'; a '#' followed by whitespace
// and other text starts a comment (nothing can follow a word-final boundary).
func stripComment(line string) string {
	slash := -1
	underscore := false
	depth := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '[', '{', '(':
			depth++
		case ']', '}', ')':
			depth--
		case '/':
			if slash < 0 && depth == 0 {
				slash = i
			}
		case '_':
			if slash >= 0 {
				underscore = true
			}
		case ',':
			if slash >= 0 && depth == 0 {
				underscore = false // a new environment begins
			}
		case '#':
			if slash < 0 || strings.TrimSpace(line[:i]) == "" {
				return line[:i]
			}
			if !underscore {
				continue
			}
			rest := line[i+1:]
			trimmed := strings.TrimLeft(rest, " \t")
			if trimmed == "" || trimmed[0] == '#' || len(trimmed) == len(rest) {
				continue
			}
			return line[:i]
		}
	}
	return line
}

func (p *parser) parseClass(text string, col int) error {
	eq := strings.Index(text, "=")
	if eq < 0 {
		return p.errf(col, "expected 'class NAME = ...'")
	}
	name := strings.TrimSpace(text[:eq])
	if !isIdent(name) {
		return p.errf(col, "bad class name "+strconv.Quote(name))
	}
	body := strings.TrimSpace(text[eq+1:])
	var pat *SegPat
	var err error
	switch {
	case strings.HasPrefix(body, "[") && strings.HasSuffix(body, "]"):
		pat, err = p.parseMatrix(body[1:len(body)-1], col+eq+2)
	case strings.HasPrefix(body, "{") && strings.HasSuffix(body, "}"):
		pat, err = p.parseList(body[1:len(body)-1], col+eq+2)
	default:
		pat, err = p.parseList(body, col+eq+2)
	}
	if err != nil {
		return err
	}
	pat.Src = name
	c := &Class{Name: name, Pat: pat}
	p.classes[name] = c
	p.file.Classes = append(p.file.Classes, c)
	return nil
}

func (p *parser) parseHeader(text string, col int) (*Rule, error) {
	if !strings.HasSuffix(text, ":") {
		return nil, p.errf(col, "rule header must end with ':'")
	}
	text = strings.TrimSpace(text[:len(text)-1])
	r := &Rule{Line: p.line}
	if at := strings.Index(text, "@"); at >= 0 {
		ann := strings.TrimSpace(text[at:])
		text = strings.TrimSpace(text[:at])
		if !strings.HasPrefix(ann, "@diffuse") {
			return nil, p.errf(col, "unknown annotation "+ann)
		}
		arg := strings.TrimPrefix(ann, "@diffuse")
		if arg == "" {
			arg = "()"
		}
		if arg[0] != '(' || arg[len(arg)-1] != ')' {
			return nil, p.errf(col, "expected @diffuse(ARG)")
		}
		r.Diffuse = strings.TrimSpace(arg[1 : len(arg)-1])
		if r.Diffuse == "" {
			r.Diffuse = "freq-first"
		}
	}
	if text == "" || strings.ContainsAny(text, " \t") {
		return nil, p.errf(col, "bad rule name "+strconv.Quote(text))
	}
	r.Name = text
	return r, nil
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(r == '_' || r == '-' || r == '$' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r))) {
			return false
		}
	}
	return true
}

// splitTop splits s on sep outside brackets.
func splitTop(s string, sep string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[', '{', '(':
			depth++
		case ']', '}', ')':
			depth--
		}
		if depth == 0 && strings.HasPrefix(s[i:], sep) {
			out = append(out, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	return append(out, s[start:])
}

func (p *parser) parseClause(text string, col int) (*Clause, error) {
	c := &Clause{Line: p.line, Src: text}
	parts := splitTop(text, "=>")
	if len(parts) != 2 {
		return nil, p.errf(col, "expected TARGET => REPLACEMENT")
	}
	lhs := parts[0]
	rhs := parts[1]
	envSrc := ""
	if sl := splitTop(rhs, "/"); len(sl) > 1 {
		rhs = sl[0]
		envSrc = strings.Join(sl[1:], "/")
	}
	toks, err := p.tokenize(lhs, col)
	if err != nil {
		return nil, err
	}
	if len(toks) == 1 && toks[0].text == "*" {
		// insertion
	} else {
		c.Target, err = p.parseTarget(toks)
		if err != nil {
			return nil, err
		}
	}
	rcol := col + len(parts[0]) + 2
	rtoks, err := p.tokenize(rhs, rcol)
	if err != nil {
		return nil, err
	}
	c.Repl, err = p.parseRepl(rtoks, rcol)
	if err != nil {
		return nil, err
	}
	if err := p.checkShape(c, col); err != nil {
		return nil, err
	}
	if envSrc != "" {
		c.Envs, err = p.parseEnvs(envSrc, col+len(text)-len(envSrc))
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (p *parser) checkShape(c *Clause, col int) error {
	if len(c.Target) == 0 {
		for _, o := range c.Repl {
			if o.Kind != OLit {
				return p.errf(col, "insertion (* => ...) needs literal segments")
			}
		}
		if len(c.Repl) == 0 {
			return p.errf(col, "empty insertion")
		}
		return nil
	}
	for _, o := range c.Repl {
		if o.Kind == OList {
			if len(c.Repl) != len(c.Target) {
				return p.errf(col, "list replacement needs one target element per replacement element")
			}
		}
	}
	for i, o := range c.Repl {
		if o.Kind == OList && i < len(c.Target) {
			t := c.Target[i].Seg
			if t.Alts == nil {
				return p.errf(col, "list replacement needs a list target (p t k => f θ x)")
			}
			if len(o.List) != len(t.Alts) && len(o.List) != 1 {
				return p.errf(col, "list lengths differ: "+strconv.Itoa(len(t.Alts))+" vs "+strconv.Itoa(len(o.List)))
			}
		}
	}
	if len(c.Repl) != len(c.Target) {
		// Span replacement: only literals or a single deletion.
		allLit := true
		for _, o := range c.Repl {
			if o.Kind != OLit {
				allLit = false
			}
		}
		if !allLit && !(len(c.Repl) == 1 && c.Repl[0].Kind == ODelete) {
			return p.errf(col, "target has "+strconv.Itoa(len(c.Target))+" elements but replacement has "+strconv.Itoa(len(c.Repl))+"; only literal or * replacements may differ in length")
		}
	}
	return nil
}

func (p *parser) parseEnvs(src string, col int) ([]Env, error) {
	var envs []Env
	for _, part := range splitTop(src, ",") {
		toks, err := p.tokenize(part, col)
		if err != nil {
			return nil, err
		}
		under := -1
		for i, t := range toks {
			if t.text == "_" {
				if under >= 0 {
					return nil, p.errf(t.col, "two '_' in one environment")
				}
				under = i
			}
		}
		if under < 0 {
			return nil, p.errf(col, "environment needs '_'")
		}
		left, err := p.parseContext(toks[:under])
		if err != nil {
			return nil, err
		}
		right, err := p.parseContext(toks[under+1:])
		if err != nil {
			return nil, err
		}
		envs = append(envs, Env{Left: left, Right: right, Src: strings.TrimSpace(part)})
		col += len(part) + 1
	}
	return envs, nil
}

type token struct {
	text string
	col  int
	glue bool // no whitespace before this token
}

const specials = "[]{}()_*#.+=,"

// tokenize splits pattern text into tokens. Brackets and braces keep their
// contents as one token.
func (p *parser) tokenize(s string, col int) ([]token, error) {
	var toks []token
	i := 0
	space := true
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) {
			i += n
			space = true
			continue
		}
		start := i
		switch {
		case r == '[' || r == '{':
			closer := byte(']')
			if r == '{' {
				closer = '}'
			}
			j := strings.IndexByte(s[i:], closer)
			if j < 0 {
				return nil, p.errf(col+i, "unclosed "+string(r))
			}
			i += j + 1
		case strings.ContainsRune(specials, r):
			i += n
		default:
			for i < len(s) {
				r, n := utf8.DecodeRuneInString(s[i:])
				if unicode.IsSpace(r) || strings.ContainsRune(specials, r) {
					break
				}
				i += n
			}
		}
		toks = append(toks, token{text: s[start:i], col: col + start, glue: !space})
		space = false
	}
	return toks, nil
}

// parseAtom parses one token into a segment pattern sequence: a class name,
// matrix, list, or IPA literal (which may contain several segments).
func (p *parser) parseAtom(t token) ([]*SegPat, error) {
	switch {
	case t.text[0] == '[':
		pat, err := p.parseMatrix(t.text[1:len(t.text)-1], t.col+1)
		if err != nil {
			return nil, err
		}
		pat.Src = t.text
		return []*SegPat{pat}, nil
	case t.text[0] == '{':
		pat, err := p.parseList(t.text[1:len(t.text)-1], t.col+1)
		if err != nil {
			return nil, err
		}
		pat.Src = t.text
		return []*SegPat{pat}, nil
	}
	if c, ok := p.classes[t.text]; ok {
		pat := newPat()
		pat.Classes = []*Class{c}
		pat.Src = c.Name
		return []*SegPat{pat}, nil
	}
	w, err := phon.ParseWord(t.text)
	if err != nil {
		return nil, p.errf(t.col, "not a class or IPA: "+strconv.Quote(t.text)+" ("+err.Error()+")")
	}
	if len(w.Bounds) > 0 {
		return nil, p.errf(t.col, "boundary inside literal")
	}
	var out []*SegPat
	for _, s := range w.Segs {
		pat := newPat()
		pat.Alts = []phon.FeatureSet{s.F}
		pat.Src = phon.RenderFeatures(s.F)
		out = append(out, pat)
	}
	return out, nil
}

func (p *parser) parseMatrix(body string, col int) (*SegPat, error) {
	pat := newPat()
	for _, item := range strings.FieldsFunc(body, func(r rune) bool { return unicode.IsSpace(r) || r == ',' }) {
		switch {
		case item[0] == '+' || item[0] == '-':
			f, ok := phon.FeatureByName(item[1:])
			if !ok {
				return nil, p.errf(col, "unknown feature "+strconv.Quote(item[1:]))
			}
			pat.Feat = pat.Feat.Set(f, item[0] == '+')
		case strings.Contains(item, ":"):
			k, v, _ := strings.Cut(item, ":")
			if err := p.setProsody(k, v, &pat.Stress, &pat.Tone, &pat.Phon, col); err != nil {
				return nil, err
			}
		default:
			if c, ok := p.classes[item]; ok {
				pat.Classes = append(pat.Classes, c)
				continue
			}
			w, err := phon.ParseWord(item)
			if err != nil || len(w.Segs) != 1 {
				return nil, p.errf(col, "matrix item "+strconv.Quote(item)+" is not a feature, class or single IPA segment")
			}
			pat.Alts = append(pat.Alts, w.Segs[0].F)
		}
	}
	return pat, nil
}

func (p *parser) setProsody(k, v string, stress, tone, ph *int8, col int) error {
	switch k {
	case "stress":
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 2 {
			return p.errf(col, "stress must be 0, 1 or 2")
		}
		*stress = int8(n)
	case "tone":
		t, ok := phon.ToneByName(v)
		if !ok {
			return p.errf(col, "unknown tone "+strconv.Quote(v))
		}
		*tone = int8(t)
	case "phon":
		ph2, ok := phon.PhonationByName(v)
		if !ok {
			return p.errf(col, "unknown phonation "+strconv.Quote(v))
		}
		*ph = int8(ph2)
	default:
		return p.errf(col, "unknown prosodic key "+strconv.Quote(k))
	}
	return nil
}

func (p *parser) parseList(body string, col int) (*SegPat, error) {
	pat := newPat()
	for _, item := range strings.Fields(body) {
		if c, ok := p.classes[item]; ok && c.Pat.Alts != nil && c.Pat.Feat.Mask == 0 && len(c.Pat.Classes) == 0 {
			pat.Alts = append(pat.Alts, c.Pat.Alts...)
			continue
		}
		w, err := phon.ParseWord(item)
		if err != nil || len(w.Segs) != 1 {
			return nil, p.errf(col, "list item "+strconv.Quote(item)+" is not a single IPA segment")
		}
		pat.Alts = append(pat.Alts, w.Segs[0].F)
	}
	if len(pat.Alts) == 0 {
		return nil, p.errf(col, "empty list")
	}
	return pat, nil
}

// isPlainSingle reports whether the token is an IPA literal of exactly one segment.
func (p *parser) isPlainSingle(t token) bool {
	if strings.ContainsRune("[{", rune(t.text[0])) || strings.ContainsRune(specials, rune(t.text[0])) {
		return false
	}
	if _, ok := p.classes[t.text]; ok {
		return false
	}
	w, err := phon.ParseWord(t.text)
	return err == nil && len(w.Segs) == 1 && len(w.Bounds) == 0
}

func (p *parser) allPlainSingles(toks []token) bool {
	if len(toks) < 2 {
		return false
	}
	for _, t := range toks {
		if !p.isPlainSingle(t) {
			return false
		}
	}
	return true
}

func (p *parser) parseTarget(toks []token) ([]Elem, error) {
	if p.allPlainSingles(toks) {
		var src []string
		for _, t := range toks {
			src = append(src, t.text)
		}
		pat, err := p.parseList(strings.Join(src, " "), toks[0].col)
		if err != nil {
			return nil, err
		}
		pat.Src = "{" + strings.Join(src, " ") + "}"
		return []Elem{{Kind: ESeg, Seg: pat}}, nil
	}
	var out []Elem
	for _, t := range toks {
		if strings.ContainsRune(specials, rune(t.text[0])) && t.text[0] != '[' && t.text[0] != '{' {
			return nil, p.errf(t.col, "unexpected "+strconv.Quote(t.text)+" in target")
		}
		pats, err := p.parseAtom(t)
		if err != nil {
			return nil, err
		}
		for _, pat := range pats {
			out = append(out, Elem{Kind: ESeg, Seg: pat})
		}
	}
	if len(out) == 0 {
		return nil, p.errf(0, "empty target")
	}
	return out, nil
}

func (p *parser) parseRepl(toks []token, col int) ([]Out, error) {
	if len(toks) == 0 {
		return nil, p.errf(col, "empty replacement")
	}
	if p.allPlainSingles(toks) || (len(toks) >= 2 && p.listish(toks)) {
		o := Out{Kind: OList}
		var src []string
		for _, t := range toks {
			src = append(src, t.text)
			if t.text == "*" {
				o.List = append(o.List, nil)
				continue
			}
			w, _ := phon.ParseWord(t.text)
			o.List = append(o.List, w.Segs)
		}
		o.Src = strings.Join(src, " ")
		return []Out{o}, nil
	}
	var out []Out
	for _, t := range toks {
		switch {
		case t.text == "*":
			out = append(out, Out{Kind: ODelete, Src: "*"})
		case t.text[0] == '[':
			o := Out{Kind: OMatrix, Stress: -1, Tone: -1, Phon: -1, Src: t.text}
			for _, item := range strings.FieldsFunc(t.text[1:len(t.text)-1], func(r rune) bool { return unicode.IsSpace(r) || r == ',' }) {
				switch {
				case item[0] == '+' || item[0] == '-':
					f, ok := phon.FeatureByName(item[1:])
					if !ok {
						return nil, p.errf(t.col, "unknown feature "+strconv.Quote(item[1:]))
					}
					o.Delta = o.Delta.Set(f, item[0] == '+')
				case strings.Contains(item, ":"):
					k, v, _ := strings.Cut(item, ":")
					if err := p.setProsody(k, v, &o.Stress, &o.Tone, &o.Phon, t.col); err != nil {
						return nil, err
					}
				default:
					return nil, p.errf(t.col, "replacement matrix may only hold features and prosody")
				}
			}
			out = append(out, o)
		case t.text[0] == '{':
			pat, err := p.parseList(t.text[1:len(t.text)-1], t.col)
			if err != nil {
				return nil, err
			}
			out = append(out, listOut(pat, t.text))
		case strings.ContainsRune(specials, rune(t.text[0])):
			return nil, p.errf(t.col, "unexpected "+strconv.Quote(t.text)+" in replacement")
		default:
			if c, ok := p.classes[t.text]; ok {
				if c.Pat.Alts == nil {
					return nil, p.errf(t.col, "class "+c.Name+" in a replacement must be an IPA list")
				}
				out = append(out, listOut(c.Pat, c.Name))
				continue
			}
			w, err := phon.ParseWord(t.text)
			if err != nil {
				return nil, p.errf(t.col, "not a class or IPA: "+strconv.Quote(t.text))
			}
			out = append(out, Out{Kind: OLit, Lit: w.Segs, Src: t.text})
		}
	}
	return out, nil
}

// listish: replacement made only of single literals and '*' (p t k => f θ *).
func (p *parser) listish(toks []token) bool {
	hasLit := false
	for _, t := range toks {
		if t.text == "*" {
			continue
		}
		if !p.isPlainSingle(t) {
			return false
		}
		hasLit = true
	}
	return hasLit
}

func listOut(pat *SegPat, src string) Out {
	o := Out{Kind: OList, Src: src}
	for _, f := range pat.Alts {
		o.List = append(o.List, []phon.Segment{{F: f}})
	}
	return o
}

// parseContext parses environment tokens: atoms, boundaries, postfix '*', groups.
func (p *parser) parseContext(toks []token) ([]Elem, error) {
	var out []Elem
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch t.text {
		case "#", ".", "+", "=":
			out = append(out, Elem{Kind: EBound, Bound: t.text[0]})
			continue
		case "*":
			if !t.glue || len(out) == 0 || out[len(out)-1].Kind != ESeg || out[len(out)-1].Star {
				return nil, p.errf(t.col, "'*' must directly follow a segment element (C*)")
			}
			out[len(out)-1].Star = true
			continue
		case "(":
			depth := 1
			j := i + 1
			for ; j < len(toks) && depth > 0; j++ {
				switch toks[j].text {
				case "(":
					depth++
				case ")":
					depth--
				}
			}
			if depth != 0 {
				return nil, p.errf(t.col, "unclosed '('")
			}
			inner, err := p.parseContext(toks[i+1 : j-1])
			if err != nil {
				return nil, err
			}
			out = append(out, Elem{Kind: EOpt, Group: inner})
			i = j - 1
			continue
		case ")":
			return nil, p.errf(t.col, "unmatched ')'")
		case "_", ",":
			return nil, p.errf(t.col, "unexpected "+strconv.Quote(t.text))
		}
		pats, err := p.parseAtom(t)
		if err != nil {
			return nil, err
		}
		for _, pat := range pats {
			out = append(out, Elem{Kind: ESeg, Seg: pat})
		}
	}
	return out, nil
}
