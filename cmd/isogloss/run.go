package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/render"
	"isogloss/engine/scenario"
	"isogloss/engine/sim"
)

// runFile is the on-disk run: the save (enough to replay) plus a summary.
type runFile struct {
	Save    sim.Save        `json:"save"`
	Digest  string          `json:"digest"`
	Proto   string          `json:"proto"`
	Summary json.RawMessage `json:"summary"`
}

func runRun(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	seed := fs.Uint64("seed", 42, "random seed")
	ticks := fs.Int("ticks", 0, "ticks to simulate (one tick = one generation, ~25 years); default 40 or the scenario's length")
	path := fs.String("out", "", "write the run to this JSON file")
	scen := fs.String("scenario", "", "start from a scenario: a starter ID ("+starterIDs()+") or a YAML file")
	lang := fs.String("language", "", "start from a real proto-language ("+languageIDs()+")")
	fs.Parse(args)

	var sc *scenario.Scenario
	src := ""
	if *scen != "" {
		if st, ok := scenario.StarterByID(*scen); ok {
			src = st.Src
		} else {
			b, err := os.ReadFile(*scen)
			if err != nil {
				return fmt.Errorf("no starter %q and no such file: %w", *scen, err)
			}
			src = string(b)
		}
	}
	if *lang != "" {
		info, ok := lex.RealByID(*lang)
		if !ok {
			return fmt.Errorf("no proto-language %q (one of: %s)", *lang, languageIDs())
		}
		src = scenario.WithLanguage(src, *lang, info.Name, info.Summary)
	}
	if src != "" {
		var err error
		if sc, err = scenario.Read(src); err != nil {
			return err
		}
	}
	start := time.Now()
	s, err := sim.NewScenario(*seed, sim.DefaultConfig(), sc)
	if err != nil {
		return err
	}
	n := *ticks
	if n == 0 {
		n = 40
		if sc != nil {
			n = int(sc.Ticks)
		}
	}
	s.Run(n)
	el := time.Since(start)

	if sc != nil {
		fmt.Fprintf(out, "scenario: %s\n  %s\n  watch: %s\n", sc.Name, sc.Summary, sc.Watch)
	}
	fmt.Fprintf(out, "seed %d: %d ticks (%d years) in %v\n", s.Seed, s.Tick, s.Tick*25, el.Round(time.Millisecond))
	fmt.Fprintf(out, "%s → %d languages, %d communities, %d innovations\n\n", s.ProtoName(), len(s.LivingLanguages()), len(s.Communities()), len(s.Reg.All))
	printTree(out, s)

	if *path != "" {
		f := runFile{Save: s.SaveState(), Digest: fmt.Sprintf("%x", s.Digest()), Proto: s.ProtoName(), Summary: render.SnapshotJSON(s, 0)}
		b, err := json.MarshalIndent(f, "", " ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*path, b, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "\nwrote %s\n", *path)
	}
	return nil
}

func printTree(out io.Writer, s *sim.Sim) {
	var walk func(id int32, depth int)
	walk = func(id int32, depth int) {
		n := s.Tree[id]
		pad := strings.Repeat("  ", depth)
		var sound []string
		for _, d := range n.Defining {
			if in := s.Reg.Get(d); in.Kind == history.Sound {
				sound = append(sound, in.Template)
			}
		}
		defs := ""
		if len(sound) > 0 {
			shown := sound
			if len(shown) > 6 {
				shown = append(shown[:6:6], fmt.Sprintf("… +%d", len(sound)-6))
			}
			defs = "  [" + strings.Join(shown, ", ") + "]"
		}
		switch {
		case id == 0:
			name := s.ProtoName()
			if len(s.Protos) > 1 {
				var ns []string
				for f := range s.Protos {
					ns = append(ns, s.FamilyName(uint8(f)))
				}
				name = strings.Join(ns, ", ") + " (unrelated families)"
			}
			fmt.Fprintf(out, "%s%s%s\n", pad, name, defs)
		case n.Lang >= 0:
			l := s.Langs[n.Lang]
			fmt.Fprintf(out, "%s%s (%d hexes)%s\n", pad, l.Name, len(l.Members), defs)
		default:
			fmt.Fprintf(out, "%s•%s\n", pad, defs)
		}
		for _, c := range n.Children {
			walk(c, depth+1)
		}
	}
	if len(s.Tree) > 0 {
		walk(0, 0)
	}
}

func loadRun(path string) (*sim.Sim, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f runFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	s := sim.Replay(f.Save)
	if f.Digest != "" && fmt.Sprintf("%x", s.Digest()) != f.Digest {
		return nil, errors.New("replay digest does not match the saved run (engine version changed?)")
	}
	return s, nil
}

// parseFlagsAnywhere lets flags follow positional arguments.
func parseFlagsAnywhere(fs *flag.FlagSet, args []string) []string {
	var pos []string
	for len(args) > 0 {
		fs.Parse(args)
		args = fs.Args()
		if len(args) > 0 {
			pos = append(pos, args[0])
			args = args[1:]
		}
	}
	return pos
}

func runEtym(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("etym", flag.ExitOnError)
	langName := fs.String("lang", "", "only this language")
	form := fs.String("form", "", "an inflected form (paradigm cell such as PL or PST.3SG)")
	pos := parseFlagsAnywhere(fs, args)
	if len(pos) != 2 {
		usage()
	}
	s, err := loadRun(pos[0])
	if err != nil {
		return err
	}
	c, ok := lex.ConceptByLabel(pos[1])
	if !ok {
		var close []string
		for _, x := range lex.Concepts() {
			if strings.Contains(x.Label, pos[1]) {
				close = append(close, x.Label)
			}
		}
		sort.Strings(close)
		return fmt.Errorf("unknown concept %q (similar: %s)", pos[1], strings.Join(close, ", "))
	}
	found := false
	for _, l := range s.LivingLanguages() {
		if *langName != "" && !strings.EqualFold(l.Name, *langName) {
			continue
		}
		found = true
		if *form != "" {
			if err := printCellEtym(out, s, l, c, *form); err != nil {
				return err
			}
			continue
		}
		e, _ := render.EtymOf(s, l.Centre, c)
		fmt.Fprintln(out, e.Text())
	}
	if !found {
		return fmt.Errorf("no living language named %q", *langName)
	}
	return nil
}

func runExport(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	cldf := fs.String("cldf", "", "write a CLDF Wordlist into this directory")
	csv := fs.String("csv", "", "write the comparative table (all concepts) as CSV")
	pos := parseFlagsAnywhere(fs, args)
	if len(pos) != 1 || (*cldf == "" && *csv == "") {
		usage()
	}
	s, err := loadRun(pos[0])
	if err != nil {
		return err
	}
	if *cldf != "" {
		if err := os.MkdirAll(*cldf, 0o755); err != nil {
			return err
		}
		files := render.CLDF(s)
		names := make([]string, 0, len(files))
		for n := range files {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if err := os.WriteFile(filepath.Join(*cldf, n), []byte(files[n]), 0o644); err != nil {
				return err
			}
		}
		fmt.Fprintf(out, "wrote CLDF Wordlist (%d files) to %s\n", len(files), *cldf)
	}
	if *csv != "" {
		if err := os.WriteFile(*csv, []byte(render.Compare(s, false).CSV()), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "wrote %s\n", *csv)
	}
	return nil
}

func printCellEtym(out io.Writer, s *sim.Sim, l *sim.Language, c lex.ConceptID, form string) error {
	i := lex.TrackedIndex(c)
	if i < 0 {
		return fmt.Errorf("%q has no stored paradigm; try one of the frequent nouns or verbs", lex.Concepts()[c].Label)
	}
	pos := lex.Tracked()[i].POS
	k := -1
	var labels []string
	for j, cell := range morph.Cells(pos) {
		labels = append(labels, cell.Label)
		if strings.EqualFold(cell.Label, form) {
			k = j
		}
	}
	if k < 0 {
		return fmt.Errorf("no form %q for a %s (forms: %s)", form, pos, strings.Join(labels, " "))
	}
	var e struct {
		Form, Gloss, Proto, ProtoGloss, ProtoLang, Lang, BaseLabel, Base string
		Irregular                                                        bool
		Steps                                                            []struct {
			Tick                 int
			Label, Before, After string
		}
	}
	if err := json.Unmarshal(render.CellEtymJSON(s, l.Centre, c, k), &e); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s  '%s'  %s  %s\n  < *%s  %s  %s\n", e.Form, lex.Concepts()[c].Label, e.Gloss, e.Lang, e.Proto, e.ProtoLang, e.ProtoGloss)
	for _, st := range e.Steps {
		fmt.Fprintf(out, "  t%-3d %-44s *%s → %s\n", st.Tick, st.Label, st.Before, st.After)
	}
	if e.Base != "" {
		note := ""
		if e.Irregular {
			note = " (this form is irregular)"
		}
		fmt.Fprintf(out, "  cf. %s %s%s\n", e.BaseLabel, e.Base, note)
	}
	fmt.Fprintln(out)
	return nil
}

func runGrammar(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("grammar", flag.ExitOnError)
	langName := fs.String("lang", "", "only this language")
	pos := parseFlagsAnywhere(fs, args)
	if len(pos) != 1 {
		usage()
	}
	s, err := loadRun(pos[0])
	if err != nil {
		return err
	}
	found := false
	for _, l := range s.LivingLanguages() {
		if *langName != "" && !strings.EqualFold(l.Name, *langName) {
			continue
		}
		found = true
		fmt.Fprintln(out, render.GrammarMarkdown(s, l.ID))
	}
	if !found {
		return fmt.Errorf("no living language named %q", *langName)
	}
	return nil
}

func starterIDs() string {
	var ids []string
	for _, st := range scenario.Starters() {
		ids = append(ids, st.ID)
	}
	return strings.Join(ids, ", ")
}

func languageIDs() string {
	var ids []string
	for _, r := range lex.RealProtos() {
		ids = append(ids, r.ID)
	}
	return strings.Join(ids, ", ")
}
