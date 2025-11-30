package engine

import (
	"testing"

	"github.com/MSmaili/rnm/internal/common/assert"
)

func TestSplitWords(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"helloWorld", "helloWorld", []string{"hello", "World"}},
		{"HTTPServer", "HTTPServer", []string{"HTTP", "Server"}},
		{"IDTESTFile", "IDTESTFile", []string{"IDTEST", "File"}},
		{"XML", "XML", []string{"XML"}},
		{"file123Name", "file123Name", []string{"file", "123", "Name"}},
		{"hello_world-test", "hello_world-test", []string{"hello", "world", "test"}},
		{"hello-world.test", "hello-world.test", []string{"hello", "world", "test"}},
		{"hello123world456", "hello123world456", []string{"hello", "123", "world", "456"}},
		{"empty_string", "", []string{}},
		{"spaces_only", "   ", []string{}},
		{"double_dash", "--", []string{}},
		{"single_underscore", "_", []string{}},
		{"single_dot", ".", []string{}},
		{"snake_case_example", "snake_case_example", []string{"snake", "case", "example"}},
		{"kebab-case-example", "kebab-case-example", []string{"kebab", "case", "example"}},
		{"HTTPRequest", "HTTPRequest", []string{"HTTP", "Request"}},
		{"HTMLParser", "HTMLParser", []string{"HTML", "Parser"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitWords(tt.input)
			assert.SliceEqual(t, got, tt.want)
		})
	}
}

func TestModes(t *testing.T) {
	tests := []struct {
		name string
		mode string
		in   string
		want string
	}{
		{"upper_HelloWorld", "upper", "HelloWorld", "HELLOWORLD"},
		{"upper_FOObar", "upper", "FOObar", "FOOBAR"},
		{"upper_fooBar", "upper", "fooBar", "FOOBAR"},
		{"upper_HTTPServer", "upper", "HTTPServer", "HTTPSERVER"},

		{"lower_fooBar", "lower", "fooBar", "foobar"},
		{"lower_FOObar", "lower", "FOObar", "foobar"},
		{"lower_HelloWorld", "lower", "HelloWorld", "helloworld"},

		{"pascal_hello_world", "pascal", "hello world", "HelloWorld"},
		{"pascal_ID_test", "pascal", "ID test", "IDTest"},

		{"camel_XML_parser", "camel", "XML parser", "XMLParser"},
		{"camel_Hello_World", "camel", "Hello World", "helloWorld"},
		{"camel_NASA_Project", "camel", "NASA Project", "NASAProject"},
		{"camel_file_name_loader", "camel", "file name loader", "fileNameLoader"},

		{"snake_Hello_World", "snake", "Hello World", "hello_world"},
		{"snake_IDTEST_File", "snake", "IDTEST File", "idtest_file"},

		{"kebab_Hello_World", "kebab", "Hello World", "hello-world"},
		{"kebab_XML_Parser", "kebab", "XML Parser", "xml-parser"},
		{"kebab_XMLParser", "kebab", "XMLParser", "xml-parser"},

		{"title_hello_world", "title", "hello world", "Hello World"},

		{"screaming_hello_world", "screaming", "hello world", "HELLO_WORLD"},
		{"screaming_FileID_Test123", "screaming", "FileID Test123", "FILE_ID_TEST_123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		name             string
		prev, curr, next rune
		want             bool
	}{
		{"digit_boundary_a_to_1", 'a', '1', 'b', true},
		{"digit_boundary_1_to_a", '1', 'a', 'b', true},

		{"acronym_boundary_PIN", 'P', 'I', 'n', true},
		{"no_boundary_HTT", 'H', 'T', 'T', false},
		{"acronym_boundary_TPs", 'T', 'P', 's', true},

		{"case_boundary_a_to_B", 'a', 'B', 'c', true},
		{"no_boundary_Bc", 'B', 'c', 'd', false},
		{"acronym_boundary_PSe", 'P', 'S', 'e', true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBoundary(tt.prev, tt.curr, tt.next)
			assert.Equal(t, got, tt.want)
		})
	}
}
