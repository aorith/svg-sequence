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
	s.OpenSection("Data", "#998800")
	s.AddStep(svgsequence.Step{SourceActor: "Data Owner", TargetActor: "Data Owner", Description: "🔐 encrypt data using global key"})
	s.AddStep(svgsequence.Step{SourceActor: "Data Owner", TargetActor: "Smart Contract", Description: "send encrypted data", Color: "#667777"})
	s.CloseSection()
	s.AddStep(svgsequence.Step{SourceActor: "Engineer", TargetActor: "Engineer", Description: "🔑 generate key pair"})
	s.OpenSection("Calculations", "#008899")
	s.AddStep(svgsequence.Step{SourceActor: "Engineer", TargetActor: "Smart Contract", Description: "request calculations"})
	s.AddStep(svgsequence.Step{SourceActor: "Smart Contract", TargetActor: "Smart Contract", Description: "process calculations against data"})
	s.AddStep(svgsequence.Step{SourceActor: "Engineer", TargetActor: "Smart Contract", Description: "send public key"})
	s.AddStep(svgsequence.Step{SourceActor: "Smart Contract", TargetActor: "Smart Contract", Description: "🔐 encrypt with engineer's public key"})
	s.AddStep(svgsequence.Step{SourceActor: "Smart Contract", TargetActor: "Engineer", Description: "send encrypted result"})
	s.CloseSection()
	s.AddStep(svgsequence.Step{SourceActor: "Engineer", TargetActor: "Engineer", Description: "🔓 decrypt using private key"})
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
