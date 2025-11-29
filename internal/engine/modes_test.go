package engine

import (
	"fmt"
	"testing"

	"github.com/MSmaili/rnm/internal/common/assert"
)

func TestSplitWords(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"helloWorld", []string{"hello", "World"}},
		{"HTTPServer", []string{"HTTP", "Server"}},
		{"IDTESTFile", []string{"IDTEST", "File"}},
		{"XML", []string{"XML"}},
		{"file123Name", []string{"file", "123", "Name"}},
		{"hello_world-test", []string{"hello", "world", "test"}},
		{"hello-world.test", []string{"hello", "world", "test"}},
		{"hello123world456", []string{"hello", "123", "world", "456"}},
		{"", []string{}},
		{"   ", []string{}},
		{"--", []string{}},
		{"_", []string{}},
		{".", []string{}},
		{"snake_case_example", []string{"snake", "case", "example"}},
		{"kebab-case-example", []string{"kebab", "case", "example"}},
		{"HTTPRequest", []string{"HTTP", "Request"}},
		{"HTMLParser", []string{"HTML", "Parser"}},
	}

	for _, tt := range tests {
		t.Run("input="+tt.input, func(t *testing.T) {
			got := splitWords(tt.input)
			assert.SliceEqual(t, got, tt.want)
		})
	}
}

func TestModes(t *testing.T) {
	tests := []struct {
		mode string
		in   string
		want string
	}{
		{"upper", "HelloWorld", "HELLOWORLD"},

		{"lower", "HelloWorld", "helloworld"},
		{"pascal", "hello world", "HelloWorld"},
		{"pascal", "ID test", "IDTest"},

		{"camel", "XML parser", "XMLParser"},
		{"camel", "Hello World", "helloWorld"},
		{"camel", "NASA Project", "NASAProject"},
		{"camel", "file name loader", "fileNameLoader"},

		{"snake", "Hello World", "hello_world"},
		{"snake", "IDTEST File", "idtest_file"},

		{"kebab", "Hello World", "hello-world"},
		{"kebab", "XML Parser", "xml-parser"},

		{"title", "hello world", "Hello World"},

		{"screaming", "hello world", "HELLO_WORLD"},
		{"screaming", "FileID Test123", "FILE_ID_TEST_123"},
	}

	for _, tt := range tests {
		t.Run("input="+tt.in, func(t *testing.T) {
			mode := ModeRegistry[tt.mode]
			got := mode.Transform(tt.in)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestUpperLowerFirst(t *testing.T) {
	assert.Equal(t, lowerFirst("Hello"), "hello")
	assert.Equal(t, upperFirst("hello"), "Hello")

	assert.Equal(t, upperFirst("äbc"), "Äbc")
	assert.Equal(t, lowerFirst("ÄBC"), "äBC")

	assert.Equal(t, lowerFirst(""), "")
	assert.Equal(t, upperFirst(""), "")
}

func TestBoundaries(t *testing.T) {
	tests := []struct {
		prev, curr, next rune
		want             bool
	}{
		{'a', '1', 'b', true},
		{'1', 'a', 'b', true},

		{'P', 'I', 'n', true},
		{'H', 'T', 'T', false},
		{'T', 'P', 's', true},

		{'a', 'B', 'c', true},
		{'B', 'c', 'd', false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%c_%c_%c", tt.prev, tt.curr, tt.next), func(t *testing.T) {
			got := isBoundary(tt.prev, tt.curr, tt.next)
			assert.Equal(t, got, tt.want)
		})
	}
}
