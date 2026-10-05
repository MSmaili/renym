package render

import (
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Render produces a full basename without repairing or sanitizing it. Target-
// filesystem validation remains the shared planner's responsibility.
func (p *Program) Render(ctx Context) (string, error) {
	var output strings.Builder
	for _, part := range p.parts {
		value := part.literal
		if part.expr != nil {
			var err error
			value, err = part.expr.evaluate(ctx)
			if err != nil {
				return "", err
			}
		}
		if output.Len()+len(value) > MaxNameBytes {
			return "", at(part.offset, "final basename exceeds 255 bytes")
		}
		output.WriteString(value)
	}
	name := output.String()
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) || !utf8.ValidString(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", at(0, "output must be a nonempty basename without path separators, dot names, or control characters")
	}
	return name, nil
}

func (e *expression) evaluate(ctx Context) (string, error) {
	var text string
	var number int64
	var timestamp time.Time
	current := stringKind
	switch e.field {
	case nameField, stemField, extField:
		if len(ctx.Name) > MaxValueBytes || !utf8.ValidString(ctx.Name) {
			return "", at(e.offset, "file name context must be valid UTF-8 and at most 1024 bytes")
		}
		text = ctx.Name
		if e.field != nameField {
			ext := path.Ext(ctx.Name)
			if ctx.Directory || ext == ctx.Name {
				ext = ""
			}
			if e.field == extField {
				text = ext
			} else {
				text = strings.TrimSuffix(text, ext)
			}
		}
	case sizeField:
		if ctx.Directory || ctx.Size == nil || *ctx.Size < 0 {
			return "", at(e.offset, "file.size is unavailable for this item (regular files only)")
		}
		current, number = integerKind, *ctx.Size
	case indexField:
		if ctx.Index <= 0 {
			return "", at(e.offset, "index is unavailable; expected a positive per-rule ordinal")
		}
		current, number = integerKind, ctx.Index
	case modifiedField:
		if ctx.Modified == nil || ctx.Modified.IsZero() {
			return "", at(e.offset, "file.modified is unavailable for this item")
		}
		current, timestamp = timeKind, *ctx.Modified
	}
	for _, step := range e.steps {
		switch step.helper {
		case "pad":
			text = strconv.FormatInt(number, 10)
			if len(text) < step.width {
				text = strings.Repeat("0", step.width-len(text)) + text
			}
		case "date":
			text = timestamp.UTC().Format(step.layout)
		default:
			text = stringHelper(step.helper, text)
		}
		current = stringKind
		if len(text) > MaxValueBytes {
			return "", at(step.offset, "intermediate value exceeds 1024 bytes")
		}
	}
	if current == integerKind {
		text = strconv.FormatInt(number, 10)
	}
	return text, nil
}
