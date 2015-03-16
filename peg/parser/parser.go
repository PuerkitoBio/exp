package parser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/exp/peg/ast"
)

var ErrInvalidEncoding = errors.New("invalid encoding")

// Generated parser would expose the following functions:
//
// func ParseFile(filename string) (interface{}, error) {
// 	f, err := os.Open(filename)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer f.Close()
// 	return Parse(filename, f)
// }
//
// func Parse(filename string, r io.Reader) (interface{}, error) {
// 	// g := grammar generated by buildParser()
// 	return parseUsingAST(filename, r, g)
// }

type errList []error

func (e *errList) add(err error) {
	*e = append(*e, err)
}

func (e *errList) err() error {
	if len(*e) == 0 {
		return nil
	}
	return e
}

func (e *errList) Error() string {
	switch len(*e) {
	case 0:
		return ""
	case 1:
		return (*e)[0].Error()
	default:
		var buf bytes.Buffer

		for i, err := range *e {
			if i > 0 {
				buf.WriteRune('\n')
			}
			buf.WriteString(err.Error())
		}
		return buf.String()
	}
}

func parseUsingAST(filename string, r io.Reader, g *ast.Grammar) (interface{}, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	p := &parser{filename: filename, errs: new(errList), data: b}
	return p.parse(g)
}

type parser struct {
	filename string
	data     []byte

	i  int  // starting index of the current rune
	rn rune // current rune
	w  int  // current rune width

	errs  *errList
	rules map[string]*ast.Rule

	peekDepth int
}

// read advances the parser to the next rune.
func (p *parser) read() {
	rn, n := utf8.DecodeRune(p.data[p.i:])
	p.i += p.w
	p.rn = rn
	p.w = n

	if rn == utf8.RuneError {
		if n > 0 {
			p.errs.add(ErrInvalidEncoding)
		}
	}
}

func (p *parser) save() (i int, rn rune, w int) {
	p.peekDepth++
	return p.i, p.rn, p.w
}

func (p *parser) restore(i int, rn rune, w int) {
	p.i, p.rn, p.w = i, rn, w
	p.peekDepth--
}

func (p *parser) buildRulesTable(g *ast.Grammar) {
	p.rules = make(map[string]*ast.Rule, len(g.Rules))
	for _, r := range g.Rules {
		p.rules[r.Name.Val] = r
	}
}

func (p *parser) parse(g *ast.Grammar) (val interface{}, err error) {
	if len(g.Rules) == 0 {
		// TODO: Valid or not?
		return nil, nil
	}

	p.buildRulesTable(g)

	// panic can be used in action code to stop parsing immediately
	// and return the panic as an error.
	defer func() {
		if e := recover(); e != nil {
			val = nil
			switch e := e.(type) {
			case error:
				err = e
			default:
				err = fmt.Errorf("%v", e)
			}
		}
	}()

	// start rule is rule [0]
	val, ok := p.parseRule(g.Rules[0])
	if !ok {
		return nil, p.errs.err()
	}
	return val, p.errs.err()
}

func (p *parser) parseRule(rule *ast.Rule) (interface{}, bool) {
	return p.parseExpr(rule.Expr)
}

func (p *parser) parseExpr(expr ast.Expression) (interface{}, bool) {
	switch expr := expr.(type) {
	case *ast.ActionExpr:
		return p.parseActionExpr(expr)
	case *ast.AndCodeExpr:
		return p.parseAndCodeExpr(expr)
	case *ast.AndExpr:
		return p.parseAndExpr(expr)
	case *ast.AnyMatcher:
		return p.parseAnyMatcher(expr)
	case *ast.CharClassMatcher:
		return p.parseCharClassMatcher(expr)
	case *ast.ChoiceExpr:
		return p.parseChoiceExpr(expr)
	case *ast.LabeledExpr:
		return p.parseLabeledExpr(expr)
	case *ast.LitMatcher:
		return p.parseLitMatcher(expr)
	case *ast.NotCodeExpr:
		return p.parseNotCodeExpr(expr)
	case *ast.NotExpr:
		return p.parseNotExpr(expr)
	case *ast.OneOrMoreExpr:
		return p.parseOneOrMoreExpr(expr)
	case *ast.RuleRefExpr:
		return p.parseRuleRefExpr(expr)
	case *ast.SeqExpr:
		return p.parseSeqExpr(expr)
	case *ast.ZeroOrMoreExpr:
		return p.parseZeroOrMoreExpr(expr)
	case *ast.ZeroOrOneExpr:
		return p.parseZeroOrOneExpr(expr)
	default:
		panic(fmt.Sprintf("unknown expression tye %T", expr))
	}
}

func (p *parser) parseActionExpr(act *ast.ActionExpr) (interface{}, bool) {
	val, ok := p.parseExpr(act.Expr)
	if ok {
		// TODO : invoke code function
	}
	return val, ok
}

func (p *parser) parseAndCodeExpr(and *ast.AndCodeExpr) (interface{}, bool) {
	// TODO : invoke code function
	// val, err := p.invoke(and.Code)
	// ok := val.(bool)
	return nil, ok
}

func (p *parser) parseAndExpr(and *ast.AndExpr) (interface{}, bool) {
	i, rn, w := p.save()
	_, ok := p.parseExpr(and.Expr)
	p.restore(i, rn, w)
	return nil, ok
}

func (p *parser) parseAnyMatcher(any *ast.AnyMatcher) (interface{}, bool) {
	if p.rn != utf8.RuneError {
		p.read()
		return string(p.rn), true
	}
	return nil, false
}

func (p *parser) parseCharClassMatcher(chr *ast.CharClassMatcher) (interface{}, bool) {
	cur := p.rn
	if chr.IgnoreCase {
		cur = unicode.ToLower(cur)
	}

	// try to match in the list of available chars
	for _, rn := range chr.Chars {
		if rn == cur {
			if chr.Inverted {
				return nil, false
			}
			p.read()
			return string(cur), true
		}
	}

	// try to match in the list of ranges
	for i := 0; i < len(chr.Ranges); i += 2 {
		if cur >= chr.Ranges[i] && cur <= chr.Ranges[i+1] {
			if chr.Inverted {
				return nil, false
			}
			p.read()
			return string(cur), true
		}
	}

	// try to match in the list of Unicode classes
	for _, cl := range chr.UnicodeClasses {
		if unicode.Is(cl, cur) {
			if chr.Inverted {
				return nil, false
			}
			p.read()
			return string(cur), true
		}
	}

	if chr.Inverted {
		p.read()
		return string(cur), true
	}
	return nil, false
}

func (p *parser) parseChoiceExpr(ch *ast.ChoiceExpr) (interface{}, bool) {
	for _, alt := range ch.Alternatives {
		val, ok := p.parseExpr(alt)
		if ok {
			return val, ok
		}
	}
	return nil, false
}

func (p *parser) parseLabeledExpr(lab *ast.LabeledExpr) (interface{}, bool) {
	val, ok := p.parseExpr(lab.Expr)
	if ok && lab.Label != nil {
		// TODO : implement storing labeled expression's result
		p.store(lab.Label.Val, val)
	}
	return val, ok
}

func (p *parser) parseLitMatcher(lit *ast.LitMatcher) (interface{}, bool) {
	// TODO : do at the ast generation phase
	if lit.IgnoreCase {
		lit.Val = strings.ToLower(lit.Val)
	}

	var buf bytes.Buffer
	i, rn, w := p.save()
	for _, want := range lit.Val {
		cur := p.rn
		buf.WriteRune(cur)
		if lit.IgnoreCase {
			cur = unicode.ToLower(cur)
		}
		if cur != want {
			p.restore(i, rn, w)
			return nil, false
		}
		p.read()
	}
	return buf.String(), true
}
