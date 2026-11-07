package rules

import (
	"strings"

	"isogloss/engine/phon"
)

// Format prints a rule back as source text that parses to the same rule
// (given the same class definitions).
func Format(r *Rule) string {
	var b strings.Builder
	b.WriteString("rule ")
	b.WriteString(r.Name)
	if r.Diffuse != "" {
		b.WriteString(" @diffuse(")
		b.WriteString(r.Diffuse)
		b.WriteString(")")
	}
	b.WriteString(":\n")
	for _, c := range r.Clauses {
		b.WriteString("    ")
		b.WriteString(c.Src)
		b.WriteString("\n")
		if len(c.Except) > 0 {
			b.WriteString("    except / ")
			for i, e := range c.Except {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(e.Src)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// FormatClasses prints the class definitions a file declared.
func FormatClasses(f *File) string {
	var b strings.Builder
	for _, c := range f.Classes {
		b.WriteString("class ")
		b.WriteString(c.Name)
		b.WriteString(" = ")
		b.WriteString(c.Pat.srcBody())
		b.WriteString("\n")
	}
	return b.String()
}

func (pat *SegPat) srcBody() string {
	var items []string
	for _, c := range pat.Classes {
		items = append(items, c.Name)
	}
	m := pat.Feat.String()
	if m != "[]" {
		items = append(items, m[1:len(m)-1])
	}
	if len(pat.Alts) > 0 && len(items) == 0 && pat.Stress < 0 && pat.Tone < 0 && pat.Phon < 0 {
		var alts []string
		for _, a := range pat.Alts {
			alts = append(alts, renderF(a))
		}
		return strings.Join(alts, " ")
	}
	for _, a := range pat.Alts {
		items = append(items, renderF(a))
	}
	return "[" + strings.Join(items, " ") + "]"
}

func renderF(f phon.FeatureSet) string { return phon.RenderFeatures(f) }
