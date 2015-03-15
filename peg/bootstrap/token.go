package bootstrap

import (
	"fmt"

	"github.com/PuerkitoBio/exp/peg/ast"
)

type tid int

const (
	invalid tid = iota - 1
	eof         // end-of-file token, id 0

	ident   tid = iota + 127 // identifiers follow the same rules as Go
	keyword                  // keywords are special identifiers
	ruledef                  // rule definition token

	// literals
	char      // character literal, as in Go ('a'i?)
	str       // double-quoted string literal, as in Go ("string"i?)
	rstr      // back-tick quoted raw string literal, as in Go (`string`i?)
	class     // square-brackets character classes ([a\n\t]i?)
	lcomment  // line comment as in Go (// comment or /* comment */ with no newline)
	mlcomment // multi-line comment as in Go (/* comment */)
	code      // code blocks between '{' and '}'

	// operators and delimiters have the value of their char
	// smallest value in that category is 10, for '\n'
	eol         tid = '\n' // end-of-line token, required in the parser
	colon       tid = ':'  // separate variable name from expression ':'
	semicolon   tid = ';'  // optional ';' to terminate rules
	lparen      tid = '('  // parenthesis to group expressions '('
	rparen      tid = ')'  // ')'
	dot         tid = '.'  // any matcher '.'
	ampersand   tid = '&'  // and-predicate '&'
	exclamation tid = '!'  // not-predicate '!'
	question    tid = '?'  // zero-or-one '?'
	plus        tid = '+'  // one-or-more '+'
	star        tid = '*'  // zero-or-more '*'
	slash       tid = '/'  // ordered choice '/'
)

var lookup = map[tid]string{
	invalid:     "invalid",
	eof:         "eof",
	ident:       "ident",
	keyword:     "keyword",
	ruledef:     "ruledef",
	char:        "char",
	str:         "str",
	rstr:        "rstr",
	class:       "class",
	lcomment:    "lcomment",
	mlcomment:   "mlcomment",
	code:        "code",
	eol:         "eol",
	colon:       "colon",
	semicolon:   "semicolon",
	lparen:      "lparen",
	rparen:      "rparen",
	dot:         "dot",
	ampersand:   "ampersand",
	exclamation: "exclamation",
	question:    "question",
	plus:        "plus",
	star:        "star",
	slash:       "slash",
}

func (t tid) String() string {
	if s, ok := lookup[t]; ok {
		return s
	}
	return fmt.Sprintf("tid(%d)", t)
}

var keywords = map[string]struct{}{
	"package": struct{}{},
}

var blacklistedIdents = map[string]struct{}{
	// Go keywords http://golang.org/ref/spec#Keywords
	"break":       struct{}{},
	"case":        struct{}{},
	"chan":        struct{}{},
	"const":       struct{}{},
	"continue":    struct{}{},
	"default":     struct{}{},
	"defer":       struct{}{},
	"else":        struct{}{},
	"fallthrough": struct{}{},
	"for":         struct{}{},
	"func":        struct{}{},
	"go":          struct{}{},
	"goto":        struct{}{},
	"if":          struct{}{},
	"import":      struct{}{},
	"interface":   struct{}{},
	"map":         struct{}{},
	"package":     struct{}{},
	"range":       struct{}{},
	"return":      struct{}{},
	"select":      struct{}{},
	"struct":      struct{}{},
	"switch":      struct{}{},
	"type":        struct{}{},
	"var":         struct{}{},

	// predeclared identifiers http://golang.org/ref/spec#Predeclared_identifiers
	"bool":       struct{}{},
	"byte":       struct{}{},
	"complex64":  struct{}{},
	"complex128": struct{}{},
	"error":      struct{}{},
	"float32":    struct{}{},
	"float64":    struct{}{},
	"int":        struct{}{},
	"int8":       struct{}{},
	"int16":      struct{}{},
	"int32":      struct{}{},
	"int64":      struct{}{},
	"rune":       struct{}{},
	"string":     struct{}{},
	"uint":       struct{}{},
	"uint8":      struct{}{},
	"uint16":     struct{}{},
	"uint32":     struct{}{},
	"uint64":     struct{}{},
	"uintptr":    struct{}{},
	"true":       struct{}{},
	"false":      struct{}{},
	"iota":       struct{}{},
	"nil":        struct{}{},
	"append":     struct{}{},
	"cap":        struct{}{},
	"close":      struct{}{},
	"complex":    struct{}{},
	"copy":       struct{}{},
	"delete":     struct{}{},
	"imag":       struct{}{},
	"len":        struct{}{},
	"make":       struct{}{},
	"new":        struct{}{},
	"panic":      struct{}{},
	"print":      struct{}{},
	"println":    struct{}{},
	"real":       struct{}{},
	"recover":    struct{}{},
}

type token struct {
	id  tid
	lit string
	pos ast.Pos
}

var tokenStringLen = 50

func (t token) String() string {
	v := t.lit
	if len(v) > tokenStringLen {
		v = v[:tokenStringLen/2] + "[...]" + v[len(v)-(tokenStringLen/2):len(v)]
	}
	return fmt.Sprintf("%s: %s %q", t.pos, t.id, v)
}
