package engine_test

// Enforces the engine-wide rules from engine/README.md:
//   - no fmt, reflect or encoding/json imports under engine/
//   - no range over a map unless the line is marked //lint:ordered
import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

var banned = map[string]bool{"fmt": true, "reflect": true, "encoding/json": true}

func TestBannedImports(t *testing.T) {
	fset := token.NewFileSet()
	filepath.WalkDir(".", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range f.Imports {
			path, _ := strconv.Unquote(im.Path.Value)
			if banned[path] {
				t.Errorf("%s imports %q (banned in engine/)", p, path)
			}
		}
		return nil
	})
}

func TestNoUnorderedMapRange(t *testing.T) {
	cfg := &packages.Config{Mode: packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedFiles, Dir: "."}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			name := pkg.Fset.File(f.Pos()).Name()
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			src, _ := os.ReadFile(name)
			lines := strings.Split(string(src), "\n")
			ast.Inspect(f, func(n ast.Node) bool {
				rs, ok := n.(*ast.RangeStmt)
				if !ok {
					return true
				}
				tv, ok := pkg.TypesInfo.Types[rs.X]
				if !ok {
					return true
				}
				if _, isMap := tv.Type.Underlying().(*types.Map); !isMap {
					return true
				}
				line := pkg.Fset.Position(rs.Pos()).Line
				if !strings.Contains(lines[line-1], "//lint:ordered") {
					t.Errorf("%s:%d: range over map; iterate sorted keys (ord.Keys) or mark //lint:ordered", name, line)
				}
				return true
			})
		}
	}
}
