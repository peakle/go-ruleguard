package ruleguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDebug(t *testing.T) {
	allTests := map[string]map[string][]string{
		`m.Match("f($x)").Where(m["x"].Type.Is("string"))`: {
			`f("abc")`: nil,

			`f(10)`: {
				`input.go:4: [rules.go:5] rejected by m["x"].Type.Is("string")`,
				`  $x int: 10`,
			},
		},

		`m.Match("$x + $y").Where(m["x"].Const && m["y"].Const)`: {
			`sink = 1 + 2`: nil,

			`sink = f().(int) + 2`: {
				`input.go:4: [rules.go:5] rejected by m["x"].Const`,
				`  $x int: f().(int)`,
				`  $y int: 2`,
			},

			`sink = 1 + f().(int)`: {
				`input.go:4: [rules.go:5] rejected by m["y"].Const`,
				`  $x int: 1`,
				`  $y int: f().(int)`,
			},
		},

		// TODO(quasilyte): don't lose "!" in the debug output.
		`m.Match("$x + $_").Where(!m["x"].Type.Is("int"))`: {
			`sink = "a" + "b"`: nil,

			`sink = int(10) + 20`: {
				`input.go:4: [rules.go:5] rejected by m["x"].Type.Is("int")`,
				`  $x int: int(10)`,
			},
		},

		`m.Match("$x + $_").Where(m["x"].Value.Int() >= 10)`: {
			`sink = 20 + 1`: nil,

			// OK: $x is const-folded.
			`sink = (2 << 3) + 1`: nil,

			// Not an int.
			`sink = "20" + "x"`: {
				`input.go:4: [rules.go:5] rejected by m["x"].Value.Int() >= 10`,
				`  $x untyped string: "20"`,
			},

			// Not a const value.
			`sink = f().(int) + 0`: {
				`input.go:4: [rules.go:5] rejected by m["x"].Value.Int() >= 10`,
				`  $x int: f().(int)`,
			},

			// Less than 10.
			`sink = 4 + 1`: {
				`input.go:4: [rules.go:5] rejected by m["x"].Value.Int() >= 10`,
				`  $x untyped int: 4`,
			},
		},

		`m.Match("_ = $x").Where(m["x"].Node.Is("ParenExpr"))`: {
			`_ = (1)`: nil,

			`_ = 10`: {
				`input.go:4: [rules.go:5] rejected by m["x"].Node.Is("ParenExpr")`,
				`  $x int: 10`,
			},
			`_ = "hello"`: {
				`input.go:4: [rules.go:5] rejected by m["x"].Node.Is("ParenExpr")`,
				`  $x string: "hello"`,
			},
			`_ = f((10))`: {
				`input.go:4: [rules.go:5] rejected by m["x"].Node.Is("ParenExpr")`,
				`  $x interface{}: f((10))`,
			},
		},
	}

	exprToRules := func(s string) *GoRuleSet {
		file := fmt.Sprintf(`
			package gorules
			import "github.com/quasilyte/go-ruleguard/dsl/fluent"
			func testrule(m fluent.Matcher) {
				%s.Report("$$")
			}`,
			s)
		fset := token.NewFileSet()
		rset, err := ParseRules("rules.go", fset, strings.NewReader(file))
		if err != nil {
			t.Fatalf("parse %s: %v", s, err)
		}
		return rset
	}

	for expr, testCases := range allTests {
		rset := exprToRules(expr)
		for input, lines := range testCases {
			runner, err := newDebugTestRunner(input)
			if err != nil {
				t.Fatalf("init %s: %s: %v", expr, input, err)
			}
			if err := runner.Run(t, rset); err != nil {
				t.Fatalf("run %s: %s: %v", expr, input, err)
			}
			if diff := cmp.Diff(runner.out, lines); diff != "" {
				t.Errorf("check %s: %s:\n(+want -have)\n%s", expr, input, diff)
			}
		}
	}
}

type debugTestRunner struct {
	ctx *Context
	f   *ast.File
	out []string
}

func (r debugTestRunner) Run(t *testing.T, rset *GoRuleSet) error {
	if err := RunRules(r.ctx, r.f, rset); err != nil {
		return err
	}
	return nil
}

func newDebugTestRunner(input string) (*debugTestRunner, error) {
	fullInput := fmt.Sprintf(`
		package testrule
		func testfunc() {
		  %s
		}
		func f(...interface{}) interface{} { return 10 }
		var sink interface{}`, input)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "input.go", []byte(fullInput), 0)
	if err != nil {
		return nil, err
	}
	var typecheker types.Config
	info := types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
	}
	pkg, err := typecheker.Check("testrule", fset, []*ast.File{f}, &info)
	if err != nil {
		return nil, err
	}
	runner := &debugTestRunner{f: f}
	ctx := &Context{
		Debug: "testrule",
		DebugPrint: func(s string) {
			runner.out = append(runner.out, s)
		},
		Pkg:   pkg,
		Types: &info,
		Sizes: types.SizesFor("gc", runtime.GOARCH),
		Fset:  fset,
		Report: func(info GoRuleInfo, n ast.Node, msg string, s *Suggestion) {
			// Do nothing.
		},
	}
	runner.ctx = ctx
	return runner, nil
}
