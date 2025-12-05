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

	var (
		words []string
		cur   []rune
	)

	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = cur[:0]
		}
	}

	var prev rune

	for i, curr := range r {
		if isDelimiter(curr) {
			flush()
			prev = 0
			continue
		}

		cur = append(cur, curr)

		var next rune
		if i < len(r)-1 {
			next = r[i+1]
		}

		if isBoundary(prev, curr, next) {
			flush()
		}

		prev = curr
	}

	flush()
	return words
}

func isBoundary(prev, curr, next rune) bool {
	if isLowerToUpperCaseBoundary(curr, next) {
		return true
	}

	if isUpperToLowerCaseBoundary(prev, curr, next) {
		return true
	}

	if isDigitBoundary(curr, next) {
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

// isUpperToLowerCaseBoundary detects transitions within acronym sequences.
// When we have multiple uppercase letters followed by lowercase, split before the last uppercase.
//
// Examples:
//   - "HTTPServer" at 'e': P(upper) -> S(upper) -> e(lower) = boundary before 'S' (HTTPS|erver)
//   - "FOOBar" at 'a': O(upper) -> B(upper) -> a(lower) = boundary before 'a' (FOOB|ar)
func isUpperToLowerCaseBoundary(prev, curr, next rune) bool {
	return unicode.IsUpper(prev) && unicode.IsUpper(curr) && unicode.IsLower(next)
}

// isLowerToUpperCaseBoundary detects transitions from lowercase to uppercase in Latin characters.
// This is the standard camelCase boundary detection for Latin alphabet characters only.
//
// Examples:
//   - "fooBar"     -> boundary between 'o' and 'B' (foo|Bar)
//   - "myVariable" -> boundary between 'y' and 'V' (my|Variable)
//   - "testCase"   -> boundary between 't' and 'C' (test|Case)
func isLowerToUpperCaseBoundary(curr, next rune) bool {
	return unicode.IsLower(curr) && unicode.IsUpper(next)
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
