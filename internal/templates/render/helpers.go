package render

import (
	"strings"

	"github.com/MSmaili/renym/internal/naming"
)

func checkHelper(name string, offset int, input kind, args []argument, parentheses bool) (step, kind, error) {
	result := step{helper: name, offset: offset}
	switch name {
	case "lower", "upper", "snake", "kebab", "pascal", "camel", "title", "screaming", "sentence":
		if input != stringKind {
			return result, stringKind, at(offset, name+" requires a string")
		}
		if parentheses || len(args) != 0 {
			return result, stringKind, at(offset, name+" takes no arguments or parentheses")
		}
	case "pad":
		if input != integerKind {
			return result, stringKind, at(offset, "pad requires an integer")
		}
		if !parentheses || len(args) != 1 || args[0].kind != integerKind || args[0].number < 1 || args[0].number > 16 {
			return result, stringKind, at(offset, "pad requires one integer width from 1 to 16")
		}
		result.width = int(args[0].number)
	case "date":
		if input != timeKind {
			return result, stringKind, at(offset, "date requires a timestamp")
		}
		if !parentheses || len(args) != 1 || args[0].kind != stringKind {
			return result, stringKind, at(offset, "date requires one string preset")
		}
		switch args[0].text {
		case "date":
			result.layout = "2006-01-02"
		case "month":
			result.layout = "2006-01"
		case "year":
			result.layout = "2006"
		case "timestamp":
			result.layout = "2006-01-02_150405"
		default:
			return result, stringKind, at(offset, "unknown date preset; expected date, month, year, or timestamp")
		}
	default:
		return result, stringKind, at(offset, "unknown helper "+name)
	}
	return result, stringKind, nil
}

func stringHelper(name, value string) string {
	// Use concrete immutable implementations, never the mutable mode registry.
	switch name {
	case "lower":
		return strings.ToLower(value)
	case "upper":
		return strings.ToUpper(value)
	case "snake":
		return (naming.SnakeCaseMode{}).Transform(value)
	case "kebab":
		return (naming.KebabCaseMode{}).Transform(value)
	case "pascal":
		return (naming.PascalCaseMode{}).Transform(value)
	case "camel":
		return (naming.CamelCaseMode{}).Transform(value)
	case "title":
		return (naming.TitleCaseMode{}).Transform(value)
	case "screaming":
		return (naming.ScreamingSnakeMode{}).Transform(value)
	case "sentence":
		return (naming.SentenceCaseMode{}).Transform(value)
	default:
		panic("unvalidated string helper")
	}
}
