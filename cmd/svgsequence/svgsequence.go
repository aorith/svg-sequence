// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"os"

	svgsequence "github.com/aorith/svg-sequence"
)

func main() {
	var (
		inputFile  = flag.String("i", "", "Input CFG file (required)")
		outputFile = flag.String("o", "", "Output SVG file (default: stdout)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -i <input.cfg> [-o <output.svg>]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Generate SVG sequence from CFG file.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -i sequence.cfg -o sequence.svg\n", os.Args[0])
	}

	flag.Parse()

	if *inputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	svg, err := svgsequence.GenerateFromCFG(*inputFile)
	if err != nil {
		panic(err)
	}

	// Write output
	if *outputFile == "" {
		fmt.Println(svg)
	} else {
		if err := os.WriteFile(*outputFile, []byte(svg), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Sequence written to %s\n", *outputFile)
	}
}
