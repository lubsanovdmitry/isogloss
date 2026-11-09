package scenario

import (
	"embed"
	"slices"
	"strings"
)

//go:embed starters/*.yaml
var starterFS embed.FS

// Starter is a scenario shipped with the engine.
type Starter struct {
	ID  string // file name without extension
	Src string
}

// Starters lists the shipped scenarios, by file name.
func Starters() []Starter {
	ents, _ := starterFS.ReadDir("starters")
	var out []Starter
	for _, e := range ents {
		b, err := starterFS.ReadFile("starters/" + e.Name())
		if err != nil {
			continue
		}
		out = append(out, Starter{strings.TrimSuffix(e.Name(), ".yaml"), string(b)})
	}
	slices.SortFunc(out, func(a, b Starter) int { return strings.Compare(a.ID, b.ID) })
	return out
}

// StarterByID returns a shipped scenario by ID ("empire-collapse").
func StarterByID(id string) (Starter, bool) {
	for _, s := range Starters() {
		if s.ID == id {
			return s, true
		}
	}
	return Starter{}, false
}

// WithLanguage returns scenario source that starts from the real
// proto-language id: src with its proto section's language set, or, for
// an empty src, a minimal scenario named after the language. Families
// that name their own language keep it.
func WithLanguage(src, id, name, summary string) string {
	if src == "" {
		return "name: " + name + "\nsummary: >-\n  " + summary +
			"\nwatch: How its sounds, words and grammar drift as its speakers spread and split.\nproto:\n  language: " + id + "\n"
	}
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	var out []string
	in, done := false, false
	for _, l := range lines {
		top := l != "" && l[0] != ' ' && l[0] != '#'
		if top {
			in = strings.HasPrefix(l, "proto:")
		}
		if in && !top && strings.HasPrefix(strings.TrimSpace(l), "language:") {
			continue
		}
		out = append(out, l)
		if top && in {
			out = append(out, "  language: "+id)
			done = true
		}
	}
	if !done {
		out = append(out, "proto:", "  language: "+id)
	}
	return strings.Join(out, "\n") + "\n"
}
