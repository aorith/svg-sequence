// SPDX-License-Identifier: MIT

package svgsequence_test

import (
	_ "embed"
	"fmt"
	"os"
	"testing"

	svgsequence "github.com/aorith/svg-sequence"
)

//go:embed tests/test1.svg
var test1 string

func TestNewSequence(t *testing.T) {
	s := svgsequence.NewSequence()
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
	s.SetDistance(240)
	got, err := s.Generate()
	if err != nil {
		fmt.Printf("%v\n", err)
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
