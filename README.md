# svg-sequence

Small go library and CLI tool to generate sequence diagrams in SVG format.

![](./cmd/cli/examples/complete.svg)

## Usage

### As a library

Check the reference [documentation](https://pkg.go.dev/github.com/aorith/svg-sequence)

```go
package main

import (
	"fmt"

	svgsequence "github.com/aorith/svg-sequence"
)

func main() {
	s := svgsequence.NewSequence()

	s.AddStep(svgsequence.Step{Source: "Bob", Target: "Maria", Text: "Hi! How are you doing?"})
	s.OpenSection("response", nil)
	s.AddStep(svgsequence.Step{
		Source: "Maria", Target: "Maria",
		Text: "*Thinks*\nLong time no see...",
		Color:       "#667777",
	})
	s.AddStep(svgsequence.Step{Source: "Maria", Target: "Bob", Text: "Fine!"})
	s.CloseSection()
	svg, err := s.Generate()
	if err != nil {
		panic(err)
	}
	fmt.Println(svg)
}
```

This will print a basic SVG that you can save and open in the browser.

### Using the CLI

Check the command at [cmd/cli](cmd/cli).

See [cmd/cli/examples](cmd/cli/examples) for config examples.

```sh
# Install the CLI tool or run it from cmd/cli
go install github.com/aorith/svg-sequence/cmd/cli@latest

# Generate a sequence from a config file
$ svgsequence -i complete.cfg -o /tmp/sequence.svg
```
