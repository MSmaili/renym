package render

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

func fixture() Context {
	modified := time.Date(2026, 10, 5, 1, 2, 3, 0, time.FixedZone("fixture", 2*3600))
	size := int64(1234)
	return Context{Name: "Screenshot One.PNG", Modified: &modified, Size: &size, Index: 7}
}

func TestRenderRecipesAndEscaping(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{`${file.stem | snake}${file.ext}`, "screenshot_one.PNG"},
		{`${ file.stem | snake }${ file.ext | lower }`, "screenshot_one.png"},
		{`shot_${file.modified | date("timestamp")}_${index | pad(3)}${file.ext | lower}`, "shot_2026-10-04_230203_007.png"},
		{`${file.modified | date("date")}`, "2026-10-04"},
		{`${file.modified | date("month")}`, "2026-10"},
		{`${file.modified | date("year")}`, "2026"},
		{`${file.size}`, "1234"},
		{`${index}`, "7"},
		{`${index | pad(1)}`, "7"},
		{`${index | pad(16)}`, "0000000000000007"},
		{`${file.size | pad(2) | upper}`, "1234"},
		{`${file.modified | date("\u0079ear")}`, "2026"},
		{`$${file.stem}`, "${file.stem}"},
		{`$$${file.stem}`, "$Screenshot One"},
		{`$$$$`, "$$"},
		{`cost$5-$HOME-{{literal}}`, "cost$5-$HOME-{{literal}}"},
		{`${file.stem | lower}${file.ext | upper}`, "screenshot one.PNG"},
		{`${file.stem | kebab}`, "screenshot-one"},
		{`${file.stem | pascal}`, "ScreenshotOne"},
		{`${file.stem | camel}`, "screenshotOne"},
		{`${file.stem | title}`, "Screenshot One"},
		{`${file.stem | screaming}`, "SCREENSHOT_ONE"},
		{`${file.stem | sentence}`, "Screenshot One"},
		{"${\nfile.stem\t|\rsnake\n}", "screenshot_one"},
	} {
		t.Run(test.source, func(t *testing.T) {
			program, err := Compile(test.source)
			if err != nil {
				t.Fatal(err)
			}
			got, err := program.Render(fixture())
			if err != nil || got != test.want {
				t.Fatalf("got %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestNameKindsAndNoReinterpretation(t *testing.T) {
	for _, test := range []struct {
		name         string
		directory    bool
		source, want string
	}{
		{".env", false, `${file.stem}${file.ext}`, ".env"},
		{".env", false, `${file.name | snake}`, "env"},
		{".local.env", false, `${file.stem}-copy${file.ext}`, ".local-copy.env"},
		{"archive.tar.gz", false, `${file.stem}-copy${file.ext}`, "archive.tar-copy.gz"},
		{"project.v1", true, `${file.stem}${file.ext}`, "project.v1"},
		{"project.v1", true, `${file.stem | snake}${file.ext}`, "project_v_1"},
		{"Über Grüße.PNG", false, `${file.stem | lower}${file.ext | lower}`, "über grüße.png"},
		{"ÜBER ΔΟΚΙΜΉ.PNG", false, `${file.name | lower}`, "über δοκιμή.png"},
		{"${shell}.txt", false, `${file.name}`, "${shell}.txt"},
		{"${file.stem}.txt", false, `${file.name}`, "${file.stem}.txt"},
	} {
		t.Run(test.name+fmt.Sprint(test.directory), func(t *testing.T) {
			program, err := Compile(test.source)
			if err != nil {
				t.Fatal(err)
			}
			got, err := program.Render(Context{Name: test.name, Directory: test.directory})
			if err != nil || got != test.want {
				t.Fatalf("got %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestCompileRejectsUnsupportedSyntaxTypesAndArguments(t *testing.T) {
	for _, source := range []string{
		"", string([]byte{0xff}), "${", "${}", "${file.stem", "${file.stem|}",
		`${file.created}`, `${file.parent}`, `${env.HOME}`, `${file.name.upper()}`,
		`${file.stem + "x"}`, `${file.stem || lower}`, `${file.stem | shell("rm")}`,
		`${file.stem | lower()}`, `${file.stem | snake(1)}`, `${file.stem | lower.upper}`,
		`${index | snake}`, `${file.stem | pad(3)}`, `${file.size | date("year")}`,
		`${file.modified}`, `${file.modified | lower}`, `${index | pad(3) | pad(4)}`,
		`${file.modified | date}`, `${index | pad}`, `${index | pad()}`,
		`${index | pad(0)}`, `${index | pad(17)}`, `${index | pad("3")}`,
		`${index | pad(3, 4)}`, `${index | pad(3,)}`, `${index | pad(index)}`,
		`${index | pad(010)}`, `${index | pad(+3)}`, `${index | pad(-3)}`,
		`${index | pad(3.0)}`, `${index | pad(3e0)}`, `${index | pad(9223372036854775808)}`,
		`${file.modified | date(3)}`, `${file.modified | date('year')}`,
		`${file.modified | date("YYYY")}`, `${file.modified | date("year", "UTC")}`,
		`${file.modified | date("year)}`, `${file.modified | date("\x79ear")}`,
		`${file.modified | date("\ud800")}`, `${file.modified | date("\udc00")}`,
		`${file.modified | date("\ud800\u0041")}`, `${file.modified | date("\u+123")}`,
		"${file.modified | date(\"ye\nar\")}",
		`${file.stem | if(true)}`, `${file.stem | replace("x", "y")}`,
		`${file..stem}`, `${file.stem[0]}`, `${file.stem | ${index}}`,
	} {
		t.Run(source, func(t *testing.T) {
			program, err := Compile(source)
			var diagnostic *Error
			if err == nil || program != nil || !errors.As(err, &diagnostic) || diagnostic.Offset < 1 {
				t.Fatalf("unsupported source accepted or unlocated: %+v, %v", program, err)
			}
		})
	}
}

func TestQuotedStringsAreLexedBeforePipelineDelimiters(t *testing.T) {
	for _, source := range []string{
		`${file.modified | date("year|month}")}`,
		`${file.modified | date("year\"}|month")}`,
		`${file.modified | date("\ud83d\ude00")}`,
		`${file.modified | date("\\")}`,
	} {
		_, err := Compile(source)
		if err == nil || !strings.Contains(err.Error(), "unknown date preset") {
			t.Fatalf("quoted delimiters/valid JSON parsed incorrectly: %s, %v", source, err)
		}
	}
}

func TestMissingMetadataAndInvalidOutputNeverReturnPartialNames(t *testing.T) {
	for _, test := range []struct {
		source string
		ctx    Context
	}{
		{`prefix_${file.modified | date("date")}`, Context{}},
		{`prefix_${file.modified | date("date")}`, Context{Modified: new(time.Time)}},
		{`prefix_${file.size}`, Context{}},
		{`prefix_${file.size}`, Context{Directory: true, Size: new(int64)}},
		{`prefix_${index}`, Context{}},
		{`prefix_${index}`, Context{Index: -1}},
		{`${file.name}`, Context{Name: "bad/name"}},
		{`${file.name}`, Context{Name: `bad\name`}},
		{`${file.name}`, Context{Name: "bad\nname"}},
		{`${file.name}`, Context{Name: "bad\x00name"}},
		{`${file.name}`, Context{Name: "bad\u0085name"}},
		{`${file.name}`, Context{Name: string([]byte{0xff})}},
		{`${file.name}`, Context{Name: "."}}, {`${file.name}`, Context{Name: ".."}},
		{`${file.ext}`, Context{Name: ".env"}},
		{strings.Repeat("x", MaxNameBytes+1), Context{}},
		{`${file.name}`, Context{Name: strings.Repeat("x", MaxValueBytes+1)}},
	} {
		program, err := Compile(test.source)
		if err != nil {
			t.Fatal(err)
		}
		name, err := program.Render(test.ctx)
		if err == nil || name != "" {
			t.Fatalf("returned partial/invalid output %q, %v for %+v", name, err, test)
		}
	}
	size := int64(-1)
	program, _ := Compile(`${file.size}`)
	if name, err := program.Render(Context{Size: &size}); err == nil || name != "" {
		t.Fatalf("negative size accepted: %q, %v", name, err)
	}
}

func TestDependenciesAndConcurrentReuse(t *testing.T) {
	program, err := Compile(`file_${file.modified | date("year")}_${file.size}_${index | pad(2)}${file.ext}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := program.Dependencies(); got != (Dependencies{Modified: true, Size: true, Index: true}) {
		t.Fatalf("dependencies: %+v", got)
	}
	plain, _ := Compile(`${file.stem}`)
	if got := plain.Dependencies(); got != (Dependencies{}) {
		t.Fatalf("unneeded metadata dependency: %+v", got)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				name, err := program.Render(fixture())
				if err != nil || name != "file_2026_1234_07.PNG" {
					t.Errorf("nondeterministic render: %q, %v", name, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestLimitsAndDiagnosticOffsets(t *testing.T) {
	for _, source := range []string{
		strings.Repeat("x", MaxSourceBytes+1),
		strings.Repeat(`${file.name}`, MaxParts+1),
		`${file.name` + strings.Repeat(` | lower`, MaxSteps+1) + `}`,
		`${file.modified | date("` + strings.Repeat("x", MaxValueBytes+1) + `")}`,
		`${index | pad(` + strings.Repeat(`1,`, MaxTokens) + `1)}`,
	} {
		if p, err := Compile(source); err == nil || p != nil {
			t.Fatalf("limit not enforced: %s", source)
		}
	}
	for _, source := range []string{strings.Repeat("x", MaxSourceBytes), strings.Repeat(`${file.name}`, MaxParts), `${file.name` + strings.Repeat(` | lower`, MaxSteps) + `}`} {
		if _, err := Compile(source); err != nil {
			t.Fatalf("exact compile limit rejected: %v", err)
		}
	}
	program, _ := Compile(`${file.name}`)
	if name, err := program.Render(Context{Name: strings.Repeat("x", MaxNameBytes)}); err != nil || len(name) != MaxNameBytes {
		t.Fatalf("exact name limit rejected: %v", err)
	}
	if _, err := program.Render(Context{Name: strings.Repeat("界", 86)}); err == nil {
		t.Fatal("name limit counted runes instead of bytes")
	}
	// Uppercasing U+0250 expands its UTF-8 representation from 2 to 3 bytes.
	expand, _ := Compile(`${file.name | upper}`)
	if name, err := expand.Render(Context{Name: strings.Repeat("ɐ", 512)}); err == nil || name != "" || !strings.Contains(err.Error(), "intermediate") {
		t.Fatalf("expansion exceeded intermediate limit: %q, %v", name, err)
	}
	_, err := Compile(`é_${unknown}`)
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Offset != 6 {
		t.Fatalf("offset not a 1-based decoded-string byte offset: %v", err)
	}
	pad, _ := Compile(`${file.size | pad(1)}`)
	size := int64(math.MaxInt64)
	if name, err := pad.Render(Context{Size: &size}); err != nil || name != "9223372036854775807" {
		t.Fatalf("pad truncated large value: %q, %v", name, err)
	}
}

func FuzzCompileAndRender(f *testing.F) {
	for _, source := range []string{`${file.stem | snake}${file.ext}`, `shot_${file.modified | date("timestamp")}_${index | pad(3)}.png`, `$${file.name}`, `${file.name | upper}`, `${file.modified | date("\ud800")}`} {
		f.Add(source, "Screenshot One.PNG")
	}
	f.Fuzz(func(t *testing.T, source, name string) {
		program, err := Compile(source)
		if err != nil {
			if program != nil {
				t.Fatal("failed compile returned a program")
			}
			return
		}
		ctx := fixture()
		ctx.Name = name
		first, firstErr := program.Render(ctx)
		second, secondErr := program.Render(ctx)
		if first != second || fmt.Sprint(firstErr) != fmt.Sprint(secondErr) {
			t.Fatal("nondeterministic evaluation")
		}
		if firstErr != nil {
			if first != "" {
				t.Fatal("partial output on error")
			}
			return
		}
		if first == "" || first == "." || first == ".." || len(first) > MaxNameBytes || strings.ContainsAny(first, `/\`) || !utf8.ValidString(first) || strings.IndexFunc(first, unicode.IsControl) >= 0 {
			t.Fatalf("invalid accepted basename: %q", first)
		}
	})
}

func BenchmarkRender10k(b *testing.B) {
	for name, source := range map[string]string{
		"snake":     `${file.stem | snake}${file.ext}`,
		"timestamp": `shot_${file.modified | date("timestamp")}_${index | pad(3)}${file.ext | lower}`,
		"indexed":   `${index | pad(4)}_${file.stem | kebab}${file.ext}`,
	} {
		b.Run(name, func(b *testing.B) {
			program, err := Compile(source)
			if err != nil {
				b.Fatal(err)
			}
			contexts := make([]Context, 10000)
			for i := range contexts {
				contexts[i] = fixture()
				contexts[i].Name = fmt.Sprintf("Screenshot %d.PNG", i+1)
				contexts[i].Index = int64(i + 1)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for _, ctx := range contexts {
					if _, err := program.Render(ctx); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
