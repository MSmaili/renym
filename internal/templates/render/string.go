package render

import (
	"encoding/json"
	"strconv"
	"unicode/utf16"
)

// JSON's standard decoder substitutes invalid surrogate sequences. Reject them
// first so the template never silently changes a requested Unicode literal.
func (p *parser) stringLiteral() (string, error) {
	start := p.pos
	p.pos++
	for p.pos < len(p.source) {
		c := p.source[p.pos]
		if c == '"' {
			p.pos++
			var decoded string
			if err := json.Unmarshal([]byte(p.source[start:p.pos]), &decoded); err != nil {
				return "", at(start, "invalid JSON string literal")
			}
			if len(decoded) > MaxValueBytes {
				return "", at(start, "string literal exceeds 1024 bytes")
			}
			return decoded, nil
		}
		if c < 0x20 {
			return "", at(p.pos, "unescaped control character in JSON string")
		}
		if c != '\\' {
			p.pos++
			continue
		}
		p.pos++
		if p.pos >= len(p.source) {
			break
		}
		escape := p.source[p.pos]
		p.pos++
		if escape != 'u' {
			if escape != '"' && escape != '\\' && escape != '/' && escape != 'b' && escape != 'f' && escape != 'n' && escape != 'r' && escape != 't' {
				return "", at(p.pos-1, "invalid JSON escape")
			}
			continue
		}
		value, err := p.hexRune()
		if err != nil {
			return "", err
		}
		if !utf16.IsSurrogate(value) {
			continue
		}
		if value >= 0xdc00 {
			return "", at(p.pos-4, "unpaired Unicode surrogate")
		}
		if p.pos+2 > len(p.source) || p.source[p.pos:p.pos+2] != `\u` {
			return "", at(p.pos, "expected low Unicode surrogate")
		}
		p.pos += 2
		low, err := p.hexRune()
		if err != nil {
			return "", err
		}
		if low < 0xdc00 || low > 0xdfff {
			return "", at(p.pos-4, "expected low Unicode surrogate")
		}
	}
	return "", at(start, "unterminated JSON string literal")
}

func (p *parser) hexRune() (rune, error) {
	if p.pos+4 > len(p.source) {
		return 0, at(p.pos, "expected four hexadecimal digits")
	}
	value, err := strconv.ParseUint(p.source[p.pos:p.pos+4], 16, 16)
	if err != nil {
		return 0, at(p.pos, "expected four hexadecimal digits")
	}
	// ParseUint accepts a leading +; the grammar does not.
	for _, c := range p.source[p.pos : p.pos+4] {
		if !(digit(byte(c)) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return 0, at(p.pos, "expected four hexadecimal digits")
		}
	}
	p.pos += 4
	return rune(value), nil
}
