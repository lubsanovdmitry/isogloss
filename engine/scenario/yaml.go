// Package scenario reads scenario files: preset worlds, starting
// conditions and scripted events. Files are a small YAML subset (block
// mappings and sequences, scalars, comments), parsed without reflection.
package scenario

import (
	"strconv"
	"strings"
)

// Node is a parsed YAML value: a mapping (Keys/Vals in file order), a
// sequence (Items) or a scalar (Str).
type Node struct {
	Kind  NodeKind
	Keys  []string
	Vals  []*Node
	Items []*Node
	Str   string
	Line  int
}

// NodeKind tells mappings, sequences and scalars apart.
type NodeKind uint8

const (
	Scalar NodeKind = iota
	Map
	Seq
)

// Error is a parse or validation error with a line number.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string { return "scenario: line " + strconv.Itoa(e.Line) + ": " + e.Msg }

type line struct {
	n      int // 1-based
	indent int
	text   string // without indentation and comment
}

// Parse reads the YAML subset.
func Parse(src string) (*Node, error) {
	var ls []line
	for i, raw := range strings.Split(src, "\n") {
		t := stripComment(raw)
		if strings.TrimSpace(t) == "" {
			continue
		}
		if strings.Contains(t, "\t") {
			return nil, &Error{i + 1, "tabs are not allowed for indentation"}
		}
		ind := len(t) - len(strings.TrimLeft(t, " "))
		ls = append(ls, line{i + 1, ind, strings.TrimRight(t[ind:], " ")})
	}
	if len(ls) == 0 {
		return &Node{Kind: Map}, nil
	}
	p := &parser{ls: ls}
	n, err := p.block(ls[0].indent)
	if err != nil {
		return nil, err
	}
	if p.i < len(ls) {
		return nil, &Error{ls[p.i].n, "unexpected indentation"}
	}
	return n, nil
}

func stripComment(s string) string {
	inQ := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQ != 0:
			if c == inQ {
				inQ = 0
			}
		case c == '"' || c == '\'':
			inQ = c
		case c == '#' && (i == 0 || s[i-1] == ' '):
			return s[:i]
		}
	}
	return s
}

type parser struct {
	ls []line
	i  int
}

// block parses lines at exactly this indentation as one mapping or sequence.
func (p *parser) block(ind int) (*Node, error) {
	first := p.ls[p.i]
	if strings.HasPrefix(first.text, "- ") || first.text == "-" {
		n := &Node{Kind: Seq, Line: first.n}
		for p.i < len(p.ls) && p.ls[p.i].indent == ind && (strings.HasPrefix(p.ls[p.i].text, "- ") || p.ls[p.i].text == "-") {
			l := p.ls[p.i]
			rest := strings.TrimSpace(strings.TrimPrefix(l.text, "-"))
			if rest == "" {
				p.i++
				if p.i >= len(p.ls) || p.ls[p.i].indent <= ind {
					n.Items = append(n.Items, &Node{Kind: Scalar, Line: l.n})
					continue
				}
				item, err := p.block(p.ls[p.i].indent)
				if err != nil {
					return nil, err
				}
				n.Items = append(n.Items, item)
				continue
			}
			// "- key: value" starts an inline mapping item: re-read the rest
			// as a line indented to where the text begins.
			if _, _, ok := splitKey(rest); ok {
				p.ls[p.i] = line{l.n, ind + 2, rest}
				item, err := p.block(ind + 2)
				if err != nil {
					return nil, err
				}
				n.Items = append(n.Items, item)
				continue
			}
			p.i++
			n.Items = append(n.Items, scalar(rest, l.n))
		}
		return n, nil
	}
	n := &Node{Kind: Map, Line: first.n}
	for p.i < len(p.ls) && p.ls[p.i].indent == ind {
		l := p.ls[p.i]
		k, v, ok := splitKey(l.text)
		if !ok {
			return nil, &Error{l.n, "expected 'key: value'"}
		}
		for _, x := range n.Keys {
			if x == k {
				return nil, &Error{l.n, "duplicate key " + k}
			}
		}
		p.i++
		var val *Node
		switch {
		case v == ">" || v == ">-" || v == "|" || v == "|-":
			// Block scalar: following lines indented deeper, folded with
			// spaces (>) or kept as lines (|).
			var parts []string
			for p.i < len(p.ls) && p.ls[p.i].indent > ind {
				parts = append(parts, p.ls[p.i].text)
				p.i++
			}
			sep := " "
			if v[0] == '|' {
				sep = "\n"
			}
			val = &Node{Kind: Scalar, Str: strings.Join(parts, sep), Line: l.n}
		case v != "":
			val = scalar(v, l.n)
		case p.i < len(p.ls) && p.ls[p.i].indent > ind:
			var err error
			if val, err = p.block(p.ls[p.i].indent); err != nil {
				return nil, err
			}
		case p.i < len(p.ls) && p.ls[p.i].indent == ind && strings.HasPrefix(p.ls[p.i].text, "- "):
			var err error // a sequence may sit at its key's indentation
			if val, err = p.block(ind); err != nil {
				return nil, err
			}
		default:
			val = &Node{Kind: Scalar, Line: l.n}
		}
		n.Keys = append(n.Keys, k)
		n.Vals = append(n.Vals, val)
	}
	if p.i < len(p.ls) && p.ls[p.i].indent > ind {
		return nil, &Error{p.ls[p.i].n, "unexpected indentation"}
	}
	return n, nil
}

// splitKey splits "key: value" (the value may be empty).
func splitKey(s string) (k, v string, ok bool) {
	if s == "" || s[0] == '"' || s[0] == '\'' {
		return "", "", false
	}
	i := strings.Index(s, ":")
	if i <= 0 || i+1 < len(s) && s[i+1] != ' ' {
		return "", "", false
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
}

func scalar(s string, n int) *Node {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		s = s[1 : len(s)-1]
	}
	return &Node{Kind: Scalar, Str: s, Line: n}
}

// Get returns a mapping's value for key, or nil.
func (n *Node) Get(key string) *Node {
	if n == nil || n.Kind != Map {
		return nil
	}
	for i, k := range n.Keys {
		if k == key {
			return n.Vals[i]
		}
	}
	return nil
}

// Int reads a scalar as an integer.
func (n *Node) Int() (int64, error) {
	v, err := strconv.ParseInt(n.Str, 10, 64)
	if err != nil {
		return 0, &Error{n.Line, "expected a number, got '" + n.Str + "'"}
	}
	return v, nil
}

// Bool reads a scalar as a boolean.
func (n *Node) Bool() (bool, error) {
	switch n.Str {
	case "true", "yes", "on":
		return true, nil
	case "false", "no", "off":
		return false, nil
	}
	return false, &Error{n.Line, "expected true or false, got '" + n.Str + "'"}
}
