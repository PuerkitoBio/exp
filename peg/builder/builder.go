// Package builder generates the parser code for a given grammar. It makes
// no attempt to verify the correctness of the grammar.
package builder

import (
	"fmt"
	"io"
	"strconv"

	"github.com/PuerkitoBio/exp/peg/ast"
)

var funcTemplate = `func %s(%s) (interface{}, error) {
%s
}
`

func BuildParser(w io.Writer, g *ast.Grammar) error {
	b := &builder{w: w}
	return b.buildParser(g)
}

type builder struct {
	w         io.Writer
	err       error
	ruleName  string
	exprIndex int
	argsStack []map[string]interface{}
}

func (b *builder) buildParser(g *ast.Grammar) error {
	b.writePackage(g.Package)
	b.writeInit(g.Init)

	for _, rule := range g.Rules {
		b.writeRule(rule)
	}

	return b.err
}

func (b *builder) writePackage(pkg *ast.Package) {
	if pkg == nil {
		return
	}
	b.writelnf("package %s\n", pkg.Name.Val)
}

func (b *builder) writeInit(init *ast.CodeBlock) {
	if init == nil {
		return
	}

	// remove opening and closing braces
	val := init.Val[1 : len(init.Val)-1]
	b.writelnf("%s\n", val)
}

func (b *builder) writeRule(rule *ast.Rule) {
	if rule == nil || rule.Name == nil {
		return
	}

	// keep trace of the current rule, as the code blocks are created
	// in functions named "on<RuleName><#ExprIndex>".
	b.ruleName = rule.Name.Val
	b.exprIndex = 0
	b.writeExpr(rule.Expr)
}

func (b *builder) writeExpr(expr ast.Expression) {
	b.exprIndex++
	switch expr := expr.(type) {
	case *ast.ActionExpr:
		// TODO : how/when?
		//b.pushArgsSet()
		b.writeExpr(expr)
		b.writeActionExpr(expr)
		//b.popArgsSet()

	case *ast.AndCodeExpr:
		// TODO : should be able to access labeled vars too, but when to
		// start a new args set?
		b.writeAndCodeExpr(expr)

	case *ast.LabeledExpr:
		// TODO : add argument to argsset
		b.writeExpr(expr.Expr)

	case *ast.NotCodeExpr:
		// TODO : should be able to access labeled vars too, but when to
		// start a new args set?
		b.writeNotCodeExpr(expr)

	case *ast.AndExpr:
		b.writeExpr(expr.Expr)
	case *ast.ChoiceExpr:
		for _, alt := range expr.Alternatives {
			b.writeExpr(alt)
		}
	case *ast.NotExpr:
		b.writeExpr(expr.Expr)
	case *ast.OneOrMoreExpr:
		b.writeExpr(expr.Expr)
	case *ast.SeqExpr:
		for _, sub := range expr.Exprs {
			b.writeExpr(sub)
		}
	case *ast.ZeroOrMoreExpr:
		b.writeExpr(expr.Expr)
	case *ast.ZeroOrOneExpr:
		b.writeExpr(expr.Expr)
	}
}

func (b *builder) writeActionExpr(act *ast.ActionExpr) {
	if act == nil {
		return
	}
	b.writeFunc(act.Code)
}

func (b *builder) writeAndCodeExpr(and *ast.AndCodeExpr) {
	if and == nil {
		return
	}
	b.writeFunc(and.Code)
}

func (b *builder) writeNotCodeExpr(not *ast.NotCodeExpr) {
	if not == nil {
		return
	}
	b.writeFunc(not.Code)
}

func (b *builder) writeFunc(code *ast.CodeBlock) {
	if code == nil {
		return
	}
	b.writef(funcTemplate, b.funcName(), "", code.Val)
}

func (b *builder) funcName() string {
	return "on" + b.ruleName + "_" + strconv.Itoa(b.exprIndex)
}

func (b *builder) writef(f string, args ...interface{}) {
	if b.err == nil {
		_, b.err = fmt.Fprintf(b.w, f, args...)
	}
}

func (b *builder) writelnf(f string, args ...interface{}) {
	b.writef(f+"\n", args...)
}
