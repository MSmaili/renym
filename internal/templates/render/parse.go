package render

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Compile admits only the documented field/helper grammar, with static types.
func Compile(source string) (*Program, error) {
	if len(source) == 0 || len(source) > MaxSourceBytes {
		return nil, at(0, "source must contain 1-4096 bytes")
	}
	if !utf8.ValidString(source) {
		return nil, at(0, "source must be valid UTF-8")
	}
	p := &Program{}
	var literal strings.Builder
	literalStart := 0
	flush := func() error {
		if literal.Len() == 0 {
			return nil
		}
		p.parts = append(p.parts, part{literal: literal.String(), offset: literalStart})
		literal.Reset()
		if len(p.parts) > MaxParts {
			return at(literalStart, "at most 64 filename parts are allowed")
		}
		return nil
	}
	for pos := 0; pos < len(source); {
		if source[pos] == '$' && pos+1 < len(source) && source[pos+1] == '{' {
			if err := flush(); err != nil {
				return nil, err
			}
			parser := parser{source: source, pos: pos + 2}
			expr, dep, err := parser.expression(pos)
			if err != nil {
				return nil, err
			}
			p.parts = append(p.parts, part{expr: expr, offset: pos})
			if len(p.parts) > MaxParts {
				return nil, at(pos, "at most 64 filename parts are allowed")
			}
			p.dependencies.Modified = p.dependencies.Modified || dep.Modified
			p.dependencies.Size = p.dependencies.Size || dep.Size
			p.dependencies.Index = p.dependencies.Index || dep.Index
			pos = parser.pos
			literalStart = pos
			continue
		}
		if literal.Len() == 0 {
			literalStart = pos
		}
		literal.WriteByte(source[pos])
		if source[pos] == '$' && pos+1 < len(source) && source[pos+1] == '$' {
			pos++
		}
		pos++
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return p, nil
}

type parser struct {
	source string
	pos    int
	tokens int
}

func (p *parser) space() {
	for p.pos < len(p.source) && strings.ContainsRune(" \t\r\n", rune(p.source[p.pos])) {
		p.pos++
	}
}

func (p *parser) token() error {
	p.tokens++
	if p.tokens > MaxTokens {
		return at(p.pos, "at most 128 tokens per interpolation are allowed")
	}
	return nil
}

func (p *parser) take(char byte) (bool, error) {
	p.space()
	if p.pos >= len(p.source) || p.source[p.pos] != char {
		return false, nil
	}
	if err := p.token(); err != nil {
		return false, err
	}
	p.pos++
	return true, nil
}

func alpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' }
func digit(c byte) bool { return c >= '0' && c <= '9' }

func (p *parser) identifier(dotted bool) (string, int, error) {
	p.space()
	start := p.pos
	if start >= len(p.source) || !alpha(p.source[start]) {
		return "", start, at(start, "expected an identifier")
	}
	for p.pos < len(p.source) {
		c := p.source[p.pos]
		if !alpha(c) && !digit(c) && !(dotted && c == '.') {
			break
		}
		p.pos++
	}
	if err := p.token(); err != nil {
		return "", start, err
	}
	return p.source[start:p.pos], start, nil
}

func (p *parser) expression(start int) (*expression, Dependencies, error) {
	name, offset, err := p.identifier(true)
	if err != nil {
		return nil, Dependencies{}, err
	}
	expr := &expression{offset: start}
	var current kind
	var dep Dependencies
	switch name {
	case "file.name":
		expr.field = nameField
	case "file.stem":
		expr.field = stemField
	case "file.ext":
		expr.field = extField
	case "file.modified":
		expr.field, current, dep.Modified = modifiedField, timeKind, true
	case "file.size":
		expr.field, current, dep.Size = sizeField, integerKind, true
	case "index":
		expr.field, current, dep.Index = indexField, integerKind, true
	default:
		return nil, dep, at(offset, "unknown field "+strconv.Quote(name))
	}
	for {
		pipe, err := p.take('|')
		if err != nil {
			return nil, dep, err
		}
		if !pipe {
			break
		}
		if len(expr.steps) >= MaxSteps {
			return nil, dep, at(p.pos, "at most 16 pipeline steps are allowed")
		}
		name, offset, err := p.identifier(false)
		if err != nil {
			return nil, dep, err
		}
		args, parenthesized, err := p.arguments()
		if err != nil {
			return nil, dep, err
		}
		transform, output, err := checkHelper(name, offset, current, args, parenthesized)
		if err != nil {
			return nil, dep, err
		}
		expr.steps = append(expr.steps, transform)
		current = output
	}
	closed, err := p.take('}')
	if err != nil {
		return nil, dep, err
	}
	if !closed {
		return nil, dep, at(p.pos, "expected | or } to end interpolation")
	}
	if current == timeKind {
		return nil, dep, at(start, "timestamps must be formatted with date(preset)")
	}
	return expr, dep, nil
}

type argument struct {
	kind   kind
	text   string
	number int64
}

func (p *parser) arguments() ([]argument, bool, error) {
	opened, err := p.take('(')
	if err != nil || !opened {
		return nil, false, err
	}
	closed, err := p.take(')')
	if err != nil {
		return nil, true, err
	}
	if closed {
		return nil, true, nil
	}
	var args []argument
	for {
		p.space()
		start := p.pos
		var arg argument
		if start < len(p.source) && p.source[start] == '"' {
			arg.text, err = p.stringLiteral()
		} else {
			arg.kind = integerKind
			for p.pos < len(p.source) && digit(p.source[p.pos]) {
				p.pos++
			}
			text := p.source[start:p.pos]
			if text == "" || len(text) > 1 && text[0] == '0' {
				return nil, true, at(start, "expected a JSON string or canonical nonnegative decimal integer")
			}
			arg.number, err = strconv.ParseInt(text, 10, 64)
			if err != nil {
				return nil, true, at(start, "integer literal is out of range")
			}
		}
		if err != nil {
			return nil, true, err
		}
		if err := p.token(); err != nil {
			return nil, true, err
		}
		args = append(args, arg)
		closed, err = p.take(')')
		if err != nil {
			return nil, true, err
		}
		if closed {
			return args, true, nil
		}
		comma, err := p.take(',')
		if err != nil {
			return nil, true, err
		}
		if !comma {
			return nil, true, at(p.pos, "expected , or ) after helper argument")
		}
	}
}
