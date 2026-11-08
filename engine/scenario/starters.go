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
