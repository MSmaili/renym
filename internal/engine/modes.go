package engine

import "github.com/MSmaili/renym/internal/naming"

// Keep the existing engine mode API while sharing the pure implementations.
type RenameMode = naming.RenameMode
type UpperCaseMode = naming.UpperCaseMode
type LowerCaseMode = naming.LowerCaseMode
type PascalCaseMode = naming.PascalCaseMode
type CamelCaseMode = naming.CamelCaseMode
type SnakeCaseMode = naming.SnakeCaseMode
type KebabCaseMode = naming.KebabCaseMode
type TitleCaseMode = naming.TitleCaseMode
type ScreamingSnakeMode = naming.ScreamingSnakeMode
type SentenceCaseMode = naming.SentenceCaseMode

var ModeRegistry = map[string]RenameMode{
	"upper": UpperCaseMode{}, "lower": LowerCaseMode{},
	"pascal": PascalCaseMode{}, "camel": CamelCaseMode{},
	"snake": SnakeCaseMode{}, "kebab": KebabCaseMode{},
	"title": TitleCaseMode{}, "screaming": ScreamingSnakeMode{},
	"sentence": SentenceCaseMode{},
}
