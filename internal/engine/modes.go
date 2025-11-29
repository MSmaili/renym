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
	return strings.ToUpper(input)
}

type LowerCaseMode struct{}

func (u LowerCaseMode) Transform(input string) string {
	return strings.ToLower(input)
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
			cur = cur[:0]
		}
	}

	for i := 0; i < len(r); i++ {
		c := r[i]

		if isDelimiter(c) {
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

		if i > 0 && isBoundary(prev, c, next) {
			flush()
		}

		cur = append(cur, c)
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

func isDigitBoundary(prev, r rune) bool {
	return (unicode.IsDigit(prev) && !unicode.IsDigit(r)) ||
		(!unicode.IsDigit(prev) && unicode.IsDigit(r))
}

func isAcronymBoundary(prev, r, next rune) bool {
	if next == 0 {
		return false
	}
	return unicode.IsUpper(prev) &&
		unicode.IsUpper(r) &&
		unicode.IsLower(next)
}

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
