package engine

import (
	"regexp"
	"strings"
)

type RenameMode interface {
	Transform(input string) string
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
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, "")
}

var wordRegex = regexp.MustCompile(`[A-Za-z0-9]+`)

func splitWords(input string) []string {
	words := wordRegex.FindAllString(input, -1)
	for i := range words {
		words[i] = strings.Title(strings.ToLower(words[i]))
	}
	return words
}

var ModeRegistry = map[string]RenameMode{
	"upper":  UpperCaseMode{},
	"lower":  LowerCaseMode{},
	"pascal": PascalCaseMode{},
}
