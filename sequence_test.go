// SPDX-License-Identifier: MIT

package svgsequence_test

import (
	_ "embed"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	svgsequence "github.com/aorith/svg-sequence"
)

//go:embed tests/test1.svg
var test1 string

func TestNewSequence(t *testing.T) {
	s := svgsequence.NewSequence()
	s.SetDistance(240)
	s.OpenSection("Data", &svgsequence.SectionConfig{Color: "#998800"})
	s.AddStep(svgsequence.Step{Source: "Data Owner", Target: "Data Owner", Text: "🔐 encrypt data using global key"})
	s.AddStep(svgsequence.Step{Source: "Data Owner", Target: "Smart Contract", Text: "send encrypted data", Color: "#667777"})
	s.CloseSection()
	s.AddStep(svgsequence.Step{Source: "Engineer", Target: "Engineer", Text: "🔑 generate key pair"})
	s.OpenSection("Calculations", &svgsequence.SectionConfig{Color: "#008899"})
	s.AddStep(svgsequence.Step{Source: "Engineer", Target: "Smart Contract", Text: "request calculations"})
	s.AddStep(svgsequence.Step{Source: "Smart Contract", Target: "Smart Contract", Text: "process calculations against data"})
	s.AddStep(svgsequence.Step{Source: "Engineer", Target: "Smart Contract", Text: "send public key"})
	s.AddStep(svgsequence.Step{Source: "Smart Contract", Target: "Smart Contract", Text: "🔐 encrypt with engineer's public key"})
	s.AddStep(svgsequence.Step{Source: "Smart Contract", Target: "Engineer", Text: "send encrypted result"})
	s.CloseSection()
	s.AddStep(svgsequence.Step{Source: "Engineer", Target: "Engineer", Text: "🔓 decrypt using private key"})

	got, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	want := test1
	if got != want {
		gotFn := "got_test.svg"
		wantFn := "want_test.svg"
		t.Errorf(`NewSequence() failed, resulting svg files saved as "%s" and "%s"`, gotFn, wantFn)
		_ = os.WriteFile(gotFn, []byte(got), 0o644)
		_ = os.WriteFile(wantFn, []byte(want), 0o644)
	}
}

//go:embed tests/test2.svg
var test2 string

func TestNewSequence2(t *testing.T) {
	s := svgsequence.NewSequence()
	s.SetDistance(240)
	s.SetWidth("1280px")
	s.AddActors("Me", "Coworker", "Boss")
	s.OpenSection("Work", &svgsequence.SectionConfig{Color: "#339922", WithoutBorder: false})
	s.AddStep(svgsequence.Step{Source: "Me", Text: "aaaaaaaaaaaa"})
	s.AddStep(svgsequence.Step{Source: "Me", Target: "Boss", Text: "click clack cluck"})
	s.AddStep(svgsequence.Step{Source: "Me", Text: "zzZzz"})
	s.AddStep(svgsequence.Step{Source: "Me", Text: "ZzZzz"})
	s.AddStep(svgsequence.Step{Source: "Boss", Text: "<.<"})
	s.AddStep(svgsequence.Step{Source: "Me", Text: "zzzZz"})
	s.OpenSection("Dinner", &svgsequence.SectionConfig{Color: "#008899", WithoutBorder: false})
	s.AddStep(svgsequence.Step{Source: "Coworker", Target: "Me", Text: "!!!?#"})
	s.AddStep(svgsequence.Step{Source: "Me", Text: "ZzZzz"})
	s.AddStep(svgsequence.Step{Source: "Me", Text: "O.o"})
	s.CloseSection()
	s.AddStep(svgsequence.Step{Source: "Coworker", Text: "O.o"})
	s.AddStep(svgsequence.Step{Source: "Me", Target: "Coworker", Text: "Rdy!!"})
	s.CloseSection()

	got, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	want := test2
	if got != want {
		gotFn := "got_test2.svg"
		wantFn := "want_test2.svg"
		t.Errorf(`NewSequence() failed, resulting svg files saved as "%s" and "%s"`, gotFn, wantFn)
		_ = os.WriteFile(gotFn, []byte(got), 0o644)
		_ = os.WriteFile(wantFn, []byte(want), 0o644)
	}
}

func TestStepTextTruncationAndTooltip(t *testing.T) {
	s := svgsequence.NewSequence()
	s.SetDistance(120)

	longText := "this is a very long description that will not fit"
	s.AddStep(svgsequence.Step{Source: "A", Target: "B", Text: longText})
	s.AddStep(svgsequence.Step{Source: "B", Target: "B", Text: "short\nmultiline text that is definitely too long to fit here"})
	s.AddStep(svgsequence.Step{Source: "B", Target: "A", Text: "ok"})

	got, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "…") {
		t.Error("expected a truncated description with an ellipsis")
	}

	if !strings.Contains(got, "<title>"+longText+"</title>") {
		t.Errorf("expected a <title> tooltip with the full, untruncated text, got:\n%s", got)
	}

	if !strings.Contains(got, "short&#xA;multiline text that is definitely too long to fit here") {
		t.Errorf("expected the tooltip of a multiline description to preserve its newline, got:\n%s", got)
	}

	if strings.Contains(got, "<title>ok</title>") {
		t.Error("a description that fits should not get a tooltip")
	}
}

func TestStepWithOnlyOneActor(t *testing.T) {
	s := svgsequence.NewSequence()
	s.SetDistance(120)
	s.AddStep(svgsequence.Step{Target: "A", Text: "note on A"}) // Source empty
	s.AddStep(svgsequence.Step{Source: "B", Text: "note on B"}) // Target empty
	s.AddStep(svgsequence.Step{Source: "A", Target: "B", Text: "A to B"})

	got, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "note on A") || !strings.Contains(got, "note on B") {
		t.Errorf("expected both step descriptions to be rendered, got:\n%s", got)
	}

	if strings.Contains(got, `marker-end="url(#seq-arrow-sm)"`) {
		t.Errorf("a step with only one actor set must not draw a self-loop, got:\n%s", got)
	}

	if strings.Count(got, `url(#seq-arrow)`) != 1 {
		t.Errorf("expected exactly one regular arrow (for the A to B step), got:\n%s", got)
	}

	if strings.Count(got, `class="seq-desc seq-desc-no-arrow"`) != 2 {
		t.Errorf(`expected the two single-actor steps to carry both "seq-desc" and "seq-desc-no-arrow" classes, got:\n%s`, got)
	}

	if !strings.Contains(got, `class="seq-desc"`) {
		t.Errorf(`expected the regular A to B step to keep the plain "seq-desc" class, got:\n%s`, got)
	}
}

func TestStepWithBothActorsEmpty(t *testing.T) {
	s := svgsequence.NewSequence()
	s.AddStep(svgsequence.Step{Text: "orphan step"})

	_, err := s.Generate()
	if err == nil {
		t.Error("expected an error when both Source and Target are empty")
	}
}

func TestSectionLink(t *testing.T) {
	s := svgsequence.NewSequence()
	s.OpenSection("with-link", &svgsequence.SectionConfig{Link: "#tx-123"})
	s.AddStep(svgsequence.Step{Source: "A", Target: "B", Text: "step"})
	s.CloseSection()
	s.OpenSection("without-link", nil)
	s.AddStep(svgsequence.Step{Source: "B", Target: "A", Text: "step"})
	s.CloseSection()

	got, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, `<a href="#tx-123">`) {
		t.Errorf(`expected section label to be wrapped in <a href="#tx-123">, got:\n%s`, got)
	}

	idx := strings.Index(got, "without-link")
	if idx == -1 {
		t.Fatalf("section label %q not found in:\n%s", "without-link", got)
	}

	if strings.Contains(got[max(0, idx-80):idx], "<a href") {
		t.Errorf("section without a Link should not be wrapped in <a>, got:\n%s", got)
	}
}

// TestSectionBottomDoesNotOverlapFollowingStep guards against the section's
// bottom edge being derived from its last member's own height. That height
// only matches the true gap to whatever comes next when the two happen to
// have equal height (true for every other test in this file); here the
// section's last member has multiline text (taller than a plain step) and
// is immediately followed by a no-arrow step (half height), so the two
// diverge and the section box must still stop above the following text.
func TestSectionBottomDoesNotOverlapFollowingStep(t *testing.T) {
	s := svgsequence.NewSequence()
	s.OpenSection("Sec", nil)
	s.AddStep(svgsequence.Step{Source: "A", Target: "B", Text: "line one\nline two"})
	s.CloseSection()
	s.AddStep(svgsequence.Step{Source: "A", Text: "note"})

	got, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	sectionRect := regexp.MustCompile(`<rect x="[0-9.]+" y="([0-9.]+)" width="[0-9.]+" height="([0-9.]+)"[^>]*></rect>\s*<text[^>]*>Sec</text>`).FindStringSubmatch(got)
	if sectionRect == nil {
		t.Fatalf("could not find the \"Sec\" section's rect in:\n%s", got)
	}

	noteText := regexp.MustCompile(`<text class="seq-desc seq-desc-no-arrow" x="[0-9.]+" y="([0-9.]+)"[^>]*>note</text>`).FindStringSubmatch(got)
	if noteText == nil {
		t.Fatalf("could not find the \"note\" step's text in:\n%s", got)
	}

	secY, _ := strconv.ParseFloat(sectionRect[1], 64)
	secHeight, _ := strconv.ParseFloat(sectionRect[2], 64)
	secBottom := secY + secHeight

	noteY, _ := strconv.ParseFloat(noteText[1], 64)

	if secBottom > noteY {
		t.Errorf("section %q bottom edge (y=%v) overlaps the following step's text (y=%v), got:\n%s", "Sec", secBottom, noteY, got)
	}
}
