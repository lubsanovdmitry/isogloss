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
	ticks := fs.Int("ticks", 40, "ticks to simulate (one tick = one generation, ~25 years)")
	path := fs.String("out", "", "write the run to this JSON file")
	fs.Parse(args)

	start := time.Now()
	s := sim.New(*seed, sim.DefaultConfig())
	s.Run(*ticks)
	el := time.Since(start)

	fmt.Fprintf(out, "seed %d: %d ticks (%d years) in %v\n", *seed, s.Tick, s.Tick*25, el.Round(time.Millisecond))
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
			fmt.Fprintf(out, "%s%s%s\n", pad, s.ProtoName(), defs)
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
