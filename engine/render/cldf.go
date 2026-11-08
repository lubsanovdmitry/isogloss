package render

import (
	"strconv"
	"strings"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/sim"
)

func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func csvRow(b *strings.Builder, fields ...string) {
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(csvField(f))
	}
	b.WriteByte('\n')
}

func langID(i int) string { return "L" + strconv.Itoa(i) }

// CSV renders the comparative table as CSV (concept, then one column per language).
func (t Table) CSV() string {
	var b strings.Builder
	csvRow(&b, append([]string{"concept"}, t.Langs...)...)
	for _, r := range t.Rows {
		csvRow(&b, append([]string{r.Concept.Label}, r.Forms...)...)
	}
	return b.String()
}

// Newick renders the shared-innovation tree with language names as leaves.
func Newick(s *sim.Sim) string {
	var walk func(id int32) string
	walk = func(id int32) string {
		n := s.Tree[id]
		if n.Lang >= 0 {
			return newickName(s.Langs[n.Lang].Name)
		}
		var kids []string
		for _, c := range n.Children {
			kids = append(kids, walk(c))
		}
		return "(" + strings.Join(kids, ",") + ")"
	}
	if len(s.Tree) == 0 {
		return ";"
	}
	return walk(0) + newickName(s.ProtoName()) + ";"
}

func newickName(n string) string {
	return "'" + strings.ReplaceAll(n, "'", "''") + "'"
}

// CLDF exports the living languages (centre varieties) and the proto-language
// as a CLDF Wordlist: languages, parameters (Concepticon-linked), forms with
// segments, and cognate judgements from the true etymologies. tree.nwk holds
// the true tree for phylogenetic validation.
func CLDF(s *sim.Sim) map[string]string {
	t := Compare(s, false)
	var langs, params, forms, cogs strings.Builder
	csvRow(&langs, "ID", "Name", "Is_Proto")
	for i, l := range t.Langs {
		csvRow(&langs, langID(i), l, strconv.FormatBool(i == 0))
	}
	csvRow(&params, "ID", "Name", "Concepticon_ID", "Concepticon_Gloss")
	for _, c := range lex.Concepts() {
		csvRow(&params, c.Label, c.Label, strconv.Itoa(c.Concepticon), c.Gloss)
	}
	csvRow(&forms, "ID", "Language_ID", "Parameter_ID", "Value", "Form", "Segments")
	csvRow(&cogs, "ID", "Form_ID", "Cognateset_ID")
	vars := []*lex.Variety{s.Proto.Variety}
	for _, l := range s.LivingLanguages() {
		vars = append(vars, s.CentreVariety(l))
	}
	n := 0
	for _, c := range lex.Concepts() {
		for li, v := range vars {
			l := v.Lex.At(c.ID)
			n++
			id := strconv.Itoa(n)
			var segs []string
			for _, sg := range l.Form.Segs {
				segs = append(segs, phon.Render(sg))
			}
			form := phon.RenderWord(l.Form)
			csvRow(&forms, id, langID(li), c.Label, form, form, strings.Join(segs, " "))
			csvRow(&cogs, id, id, strconv.FormatUint(uint64(l.Etym), 10))
		}
	}
	return map[string]string{
		"languages.csv":          langs.String(),
		"parameters.csv":         params.String(),
		"forms.csv":              forms.String(),
		"cognates.csv":           cogs.String(),
		"Wordlist-metadata.json": cldfMetadata(s),
		"tree.nwk":               Newick(s) + "\n",
	}
}

func cldfMetadata(s *sim.Sim) string {
	term := func(t string) string { return "http://cldf.clld.org/v1.0/terms.rdf#" + t }
	var j JSON
	j.Obj().KS("@context", "http://www.w3.org/ns/csvw").KS("dc:conformsTo", term("Wordlist")).
		KS("dc:title", "Isogloss simulated family (seed "+strconv.FormatUint(s.Seed, 10)+", tick "+strconv.Itoa(int(s.Tick))+")").
		K("tables").Arr()
	table := func(url, conforms string, cols [][2]string) {
		j.Obj().KS("url", url).KS("dc:conformsTo", term(conforms)).K("tableSchema").Obj().K("columns").Arr()
		for _, c := range cols {
			j.Obj().KS("name", c[0])
			if c[1] != "" {
				j.KS("propertyUrl", term(c[1]))
			}
			if c[0] == "Segments" {
				j.KS("separator", " ")
			}
			j.KS("datatype", "string").End()
		}
		j.EndArr().K("primaryKey").Arr().S("ID").EndArr().End().End()
	}
	table("languages.csv", "LanguageTable", [][2]string{{"ID", "id"}, {"Name", "name"}, {"Is_Proto", ""}})
	table("parameters.csv", "ParameterTable", [][2]string{{"ID", "id"}, {"Name", "name"}, {"Concepticon_ID", "concepticonReference"}, {"Concepticon_Gloss", ""}})
	table("forms.csv", "FormTable", [][2]string{{"ID", "id"}, {"Language_ID", "languageReference"}, {"Parameter_ID", "parameterReference"}, {"Value", "value"}, {"Form", "form"}, {"Segments", "segments"}})
	table("cognates.csv", "CognateTable", [][2]string{{"ID", "id"}, {"Form_ID", "formReference"}, {"Cognateset_ID", "cognatesetReference"}})
	j.EndArr().End()
	return j.String()
}
