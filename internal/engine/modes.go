package engine

import (
	"strings"
	"unicode"
)

type RenameMode interface {
	Transform(input string) string
}

var ModeRegistry = map[string]RenameMode{
	"upper":     UpperCaseMode{},
	"lower":     LowerCaseMode{},
	"pascal":    PascalCaseMode{},
	"camel":     CamelCaseMode{},
	"snake":     SnakeCaseMode{},
	"kebab":     KebabCaseMode{},
	"title":     TitleCaseMode{},
	"screaming": ScreamingSnakeMode{},
}

type UpperCaseMode struct{}

func (u UpperCaseMode) Transform(input string) string {
	words := splitWords(input)
	for i, w := range words {
		words[i] = strings.ToUpper(w)
	}
	return strings.Join(words, " ")
}

type LowerCaseMode struct{}

func (u LowerCaseMode) Transform(input string) string {
	words := splitWords(input)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, " ")
}

type PascalCaseMode struct{}

func (u PascalCaseMode) Transform(input string) string {
	words := splitWords(input)
	for i, w := range words {
		words[i] = upperFirst(w)
	}
	return strings.Join(words, "")
}

type CamelCaseMode struct{}

func (c CamelCaseMode) Transform(input string) string {
	words := splitWords(input)
	if len(words) == 0 {
		return input
	}

	if !isAllUpper(words[0]) {
		words[0] = strings.ToLower(words[0])
	}

	for i := 1; i < len(words); i++ {
		words[i] = upperFirst(words[i])
	}
	return strings.Join(words, "")
}

type SnakeCaseMode struct{}

func (c SnakeCaseMode) Transform(input string) string {
	words := splitWords(input)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToLower(word)
		}
	}
	return strings.Join(words, "_")
}

type KebabCaseMode struct{}

func (c KebabCaseMode) Transform(input string) string {
	words := splitWords(input)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToLower(word)
		}
	}
	return strings.Join(words, "-")
}

type TitleCaseMode struct{}

func (c TitleCaseMode) Transform(input string) string {
	words := splitWords(input)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = upperFirst(words[i])
		}
	}
	return strings.Join(words, " ")
}

type ScreamingSnakeMode struct{}

func (s ScreamingSnakeMode) Transform(input string) string {
	words := splitWords(input)
	for i, w := range words {
		words[i] = strings.ToUpper(w)
	}
	return strings.Join(words, "_")
}

func splitWords(s string) []string {
	r := []rune(s)
	if len(r) == 0 {
		return []string{}
	}

	var words []string
	var cur []rune

	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = cur[:0] //reset
		}
	}

	for i := range r {
		curr := r[i]

		if isDelimiter(curr) {
			flush()
			continue
		}

		prev := rune(0)
		next := rune(0)
		if i > 0 {
			prev = r[i-1]
		}
		if i < len(r)-1 {
			next = r[i+1]
		}

		if i > 0 && isBoundary(prev, curr, next) {
			flush()
		}

		cur = append(cur, curr)
	}

	flush()
	return words
}

func isBoundary(prev, curr, next rune) bool {
	if isDigitBoundary(prev, curr) {
		return true
	}

	if isAcronymBoundary(prev, curr, next) {
		return true
	}

	if isLatinCaseBoundary(prev, curr) {
		return true
	}

	return false
}

func isDelimiter(r rune) bool {
	return r == '_' || r == '-' || r == '.' || r == '/' || r == '\\' || unicode.IsSpace(r)
}

// isDigitBoundary detects transitions between digits and non-digits.
// This helps split words at number boundaries.
//
// Examples:
//   - "test1foo" -> boundary between '1' and 'f' (digit to letter)
//   - "foo2bar"  -> boundary between 'o' and '2' (letter to digit)
//   - "123abc"   -> boundary between '3' and 'a' (digit to letter)
func isDigitBoundary(prev, curr rune) bool {
	return (unicode.IsDigit(prev) && !unicode.IsDigit(curr)) ||
		(!unicode.IsDigit(prev) && unicode.IsDigit(curr))
}

// isAcronymBoundary detects the transition point within acronyms followed by regular words.
// It identifies where an acronym ends and a new word begins by checking for the pattern
// of two uppercase letters followed by a lowercase letter.
//
// Examples:
//   - "HTTPServer" -> boundary between 'P' and 'S' (HTTP|Server)
//   - "XMLParser"  -> boundary between 'L' and 'P' (XML|Parser)
//   - "IOError"    -> boundary between 'O' and 'E' (IO|Error)
func isAcronymBoundary(prev, curr, next rune) bool {
	if next == 0 {
		return false
	}
	return unicode.IsUpper(prev) &&
		unicode.IsUpper(curr) &&
		unicode.IsLower(next)
}

// isLatinCaseBoundary detects transitions from lowercase to uppercase in Latin characters.
// This is the standard camelCase boundary detection for Latin alphabet characters only.
//
// Examples:
//   - "fooBar"     -> boundary between 'o' and 'B' (foo|Bar)
//   - "myVariable" -> boundary between 'y' and 'V' (my|Variable)
//   - "testCase"   -> boundary between 't' and 'C' (test|Case)
func isLatinCaseBoundary(prev, curr rune) bool {
	if !unicode.In(prev, unicode.Latin) || !unicode.In(curr, unicode.Latin) {
		return false
	}
	return unicode.IsLower(prev) && unicode.IsUpper(curr)
}

func lowerFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return ""
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func upperFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return ""
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func isAllUpper(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && unicode.ToUpper(r) != r {
			return false
		}
	}
	return true
}
