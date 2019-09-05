package generate

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Generate struct {
	fileName string
	dirName  string
	*bytes.Buffer
	indent string
}

// New creates a new generator and allocates the request and response protobufs.
func New(dirName, fileName string) *Generate {
	g := &Generate{
		Buffer:   new(bytes.Buffer),
		dirName:  dirName,
		fileName: fileName,
	}
	return g
}

// P prints the arguments to the generated output.  It handles strings and int32s, plus
// handling indirections because they may be *string, etc.
func (g *Generate) P(str ...interface{}) *Generate {
	g.WriteString(g.indent)
	for _, v := range str {
		switch s := v.(type) {
		case string:
			g.WriteString(s)
		case *string:
			g.WriteString(*s)
		case bool:
			fmt.Fprintf(g, "%t", s)
		case *bool:
			fmt.Fprintf(g, "%t", *s)
		case int:
			fmt.Fprintf(g, "%d", s)
		case int32:
			fmt.Fprintf(g, "%d", s)
		case *int32:
			fmt.Fprintf(g, "%d", *s)
		case int64:
			fmt.Fprintf(g, "%d", s)
		case *int64:
			fmt.Fprintf(g, "%d", *s)
		case float64:
			fmt.Fprintf(g, "%g", s)
		case *float64:
			fmt.Fprintf(g, "%g", *s)
		case []byte:
			fmt.Fprintf(g, "%v", s)
		default:
			g.Fail(fmt.Sprintf("unknown type in printer: %T", v))
		}
	}
	g.WriteByte('\n')
	return g
}

// In Indents the output one tab stop.
func (g *Generate) In() *Generate {
	g.indent += "\t"
	return g
}

// In Indents the output one tab stop.
func (g *Generate) Symbol(symbol byte, str ...interface{}) string {
	rst := string(symbol)

	for _, v := range str {
		switch s := v.(type) {
		case string:
			rst += s
		case *string:
			rst += *s
		//case bool:
		//	fmt.Fprintf(g, "%t", s)
		//case *bool:
		//	fmt.Fprintf(g, "%t", *s)
		case int:
			rst += fmt.Sprintf("%d", s)
		case int32:
			rst += fmt.Sprintf("%d", s)
		case *int32:
			rst += fmt.Sprintf("%d", *s)
		case int64:
			rst += fmt.Sprintf("%d", s)
		case *int64:
			rst += fmt.Sprintf("%d", *s)
		case float64:
			rst += fmt.Sprintf("%g", s)
		case *float64:
			rst += fmt.Sprintf("%g", *s)
		case []byte:
			rst += fmt.Sprintf("%v", s)
		default:
			g.Fail(fmt.Sprintf("Symbol unknown type in printer: %T", v))
		}
	}
	switch symbol {
	case '(':
		rst += ")"
	case '"':
		rst += "\""
	default:
		g.Fail("unknown symbol", string(symbol))
	}
	return rst
}

func (g *Generate) InFunc(str string, fn func()) *Generate {
	g.In()
	fn()
	g.Out()
	g.P(str)
	return g
}

// Out unindents the output one tab stop.
func (g *Generate) Out() *Generate {
	if len(g.indent) > 0 {
		g.indent = g.indent[1:]
	}
	return g
}

// Error reports a problem, including an error, and exits the program.
func (g *Generate) Error(err error, msgs ...string) {
	s := strings.Join(msgs, " ") + ":" + err.Error()
	log.Print("generate: error:", s)
	os.Exit(1)
}

// Fail reports a problem and exits the program.
func (g *Generate) Fail(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Print("generate: error:", s)
	os.Exit(1)
}

func (g *Generate) WriteFile(outFilePath string) {
	os.MkdirAll(path.Join(outFilePath, g.dirName), os.ModePerm)
	f, err := os.Create(filepath.Join(outFilePath, g.dirName, g.fileName))
	if nil != err {
		log.Panicln("WriteFile create file err: ", err)
	}
	if strings.HasSuffix(g.fileName, ".go") {
		f.Write(g.gofmt(outFilePath, g.fileName, g.Bytes()))
	} else {
		f.Write(g.Bytes())
	}

	f.Close()
}

func (g *Generate) gofmt(outFilePath, fileName string, srcCode []byte) []byte {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", srcCode, parser.ParseComments)
	if err != nil {
		log.Println("---------->>>>>>", outFilePath, fileName, err)
		return srcCode
	}

	var buf bytes.Buffer

	if err = format.Node(&buf, fset, file); err != nil {
		log.Println("Node failed:", err)
		return srcCode
	}
	return buf.Bytes()
}

// CamelCase returns the CamelCased name.
// If there is an interior underscore followed by a lower case letter,
// drop the underscore and convert the letter to upper case.
// There is a remote possibility of this rewrite causing a name collision,
// but it's so remote we're prepared to pretend it's nonexistent - since the
// C++ generator lowercases names, it's extremely unlikely to have two fields
// with different capitalizations.
// In short, _my_field_name_2 becomes XMyFieldName_2.
func CamelCase(s string) string {
	if s == "" {
		return ""
	}
	t := make([]byte, 0, 32)
	i := 0
	if s[0] == '_' {
		// Need a capital letter; drop the '_'.
		t = append(t, 'X')
		i++
	}
	// Invariant: if the next letter is lower case, it must be converted
	// to upper case.
	// That is, we process a word at a time, where words are marked by _ or
	// upper case letter. Digits are treated as words.
	for ; i < len(s); i++ {
		c := s[i]
		if c == '_' && i+1 < len(s) && isASCIILower(s[i+1]) {
			continue // Skip the underscore in s.
		}
		if isASCIIDigit(c) {
			t = append(t, c)
			continue
		}
		// Assume we have a letter now - if not, it's a bogus identifier.
		// The next word is a sequence of characters that must start upper case.
		if isASCIILower(c) {
			c ^= ' ' // Make it a capital letter.
		}
		t = append(t, c) // Guaranteed not lower case.
		// Accept lower case sequence that follows.
		for i+1 < len(s) && isASCIILower(s[i+1]) {
			i++
			t = append(t, s[i])
		}
	}
	return string(t)
}

// Is c an ASCII lower-case letter?
func isASCIILower(c byte) bool {
	return 'a' <= c && c <= 'z'
}

// Is c an ASCII digit?
func isASCIIDigit(c byte) bool {
	return '0' <= c && c <= '9'
}
