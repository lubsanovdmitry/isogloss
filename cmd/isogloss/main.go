// Command isogloss is the native front end to the engine.
//
//	isogloss sca [-trace] rules.isg words.txt
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"isogloss/engine/phon"
	"isogloss/engine/rules"
)

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  isogloss sca [-trace] RULES.isg WORDS.txt   apply sound changes to a word list`)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "sca":
		err = runSCA(os.Args[2:], os.Stdout)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "isogloss:", err)
		os.Exit(1)
	}
}

func runSCA(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("sca", flag.ExitOnError)
	trace := fs.Bool("trace", false, "print every rule that changes a word")
	fs.Parse(args)
	if fs.NArg() != 2 {
		usage()
	}
	src, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	file, err := rules.Parse(string(src))
	if err != nil {
		return fmt.Errorf("%s: %w", fs.Arg(0), err)
	}
	words, err := readWords(fs.Arg(1))
	if err != nil {
		return err
	}
	w := bufio.NewWriter(out)
	defer w.Flush()
	for _, line := range words {
		in, err := phon.ParseWord(line.word)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", fs.Arg(1), line.n, err)
		}
		res, steps := file.ApplyAll(in, nil)
		gloss := ""
		if line.gloss != "" {
			gloss = "\t" + line.gloss
		}
		fmt.Fprintf(w, "%s → %s%s\n", phon.RenderWord(stressed(in)), phon.RenderWord(res), gloss)
		if *trace {
			for _, s := range steps {
				fmt.Fprintf(w, "    %-24s %s → %s\n", s.Rule.Name, phon.RenderWord(s.Before), phon.RenderWord(s.After))
			}
		}
	}
	return nil
}

func stressed(w phon.Word) phon.Word {
	phon.CleanStress(&w, nil)
	return w
}

type wordLine struct {
	n            int
	word, gloss string
}

// readWords reads one word per line; anything after the first whitespace is a
// gloss. Blank lines and lines starting with '#' are skipped.
func readWords(path string) ([]wordLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []wordLine
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		word, gloss := t, ""
		if i := strings.IndexFunc(t, unicode.IsSpace); i >= 0 {
			word, gloss = t[:i], strings.TrimSpace(t[i:])
		}
		out = append(out, wordLine{n, word, gloss})
	}
	return out, sc.Err()
}
