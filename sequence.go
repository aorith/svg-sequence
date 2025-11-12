// SPDX-License-Identifier: MIT

package svgsequence

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

//go:embed default.css
var defaultCSS string

const (
	margin                  = 20                // left and right margins
	defaultDistance         = 180               // default distance between actors
	defaultStepHeight       = 50                // default height for each step
	actorFontSize           = 16                // actor font size
	dashArraySize           = actorFontSize / 2 // actor line stroke dash-array size
	descriptionOffset       = 7                 // text description offset against the step line
	descriptionOffsetFactor = 2                 // how much is increased the offset for each line in a multiline description
)

type actor struct {
	x float64
}

type section struct {
	name           string
	color          string
	firstStepIndex *int
	lastStepIndex  *int

	x, x2, y float64
	width    float64
	height   int
}

type Step struct {
	// Description: Optional text displayed above the arrow or mark.
	Description string

	// SourceActor: Required name of the actor that initiates the action.
	SourceActor string

	// TargetActor: Required name of the actor that receives the action.
	//
	// It can be the same as sourceActor.
	TargetActor string

	// Color: Optional CSS color value (e.g., "#ff0000", "red").
	//
	// Pass an empty string to use the default color.
	Color string

	x1      float64 // SourceActor x
	x2      float64 // TargetActor x
	y       float64
	section *section
}

type Sequence struct {
	actors    []string
	actorsMap map[string]*actor // map[actorName]actor
	sections  []*section
	steps     []*Step

	width, height       string // SVG width and height (not the viewport)
	distance            int    // distance between actors
	stepHeight          int    // height for each step
	verticalSectionText bool   // whether to position the section text vertically at the left of each section
}

func NewSequence() *Sequence {
	return &Sequence{
		actorsMap:  make(map[string]*actor),
		width:      "100%",
		height:     "100%",
		distance:   defaultDistance,
		stepHeight: defaultStepHeight,
	}
}

// SetDistance sets the distance between actors
func (s *Sequence) SetDistance(d int) {
	s.distance = d
}

// SetWidth sets the SVG width.
//
// Any CSS value for size is valid, including pixels or percentages.
func (s *Sequence) SetWidth(width string) {
	s.width = width
}

// SetHeight sets the SVG height.
//
// Any CSS value for size is valid, including pixels or percentages.
func (s *Sequence) SetHeight(height string) {
	s.height = height
}

// SetStepHeight sets the height of each step in the sequence.
func (s *Sequence) SetStepHeight(h int) {
	s.stepHeight = h
}

// SetVerticalSectionText sets the section text vertically on the left
func (s *Sequence) SetVerticalSectionText(b bool) {
	s.verticalSectionText = b
}

// AddActors adds the given actors to the sequence, in order
// use this to ensure the order of the actors in the sequence
func (s *Sequence) AddActors(actors ...string) {
	// add new actors to the s.actors map and ensure that there are
	// no duplicates in the actors input
	newActors := []string{}
	for _, a := range actors {
		if a == "" {
			continue
		}
		_, ok := s.actorsMap[a]
		if !ok {
			newActors = append(newActors, a)
			s.actorsMap[a] = &actor{}
		}
	}

	// remove any actor from the existing list that is being re-added
	remaining := []string{}
	for _, a := range s.actors {
		if !slices.Contains(actors, a) {
			remaining = append(remaining, a)
		}
	}

	s.actors = append(newActors, remaining...)
}

// AddStep adds a new step to the sequence diagram.
func (s *Sequence) AddStep(step Step) {
	if step.Color == "" {
		step.Color = "#000000"
	}

	var y float64
	if len(s.steps) > 0 {
		// start with last step 'y' value
		y = s.steps[len(s.steps)-1].y
	} else {
		// first step 'y' value
		y = actorFontSize + 2
	}
	// take into account multiline descriptions
	incr := len(strings.Split(step.Description, "\n")) - 1
	y += float64(s.stepHeight) + float64((descriptionOffset*descriptionOffsetFactor)*incr)
	step.y = y

	// iterate over open sections to associate
	for _, sec := range s.sections {
		if sec.firstStepIndex == nil {
			idx := len(s.steps)
			sec.firstStepIndex = &idx
		}
		if sec.lastStepIndex == nil {
			step.section = sec
		}
	}

	if step.SourceActor != "" {
		s.ensureActor(step.SourceActor)
	}
	if step.TargetActor != "" {
		s.ensureActor(step.TargetActor)
	}

	s.steps = append(s.steps, &step)
}

// OpenSection opens a new section to the sequence diagram.
// An open section must be closed after adding the steps which should be placed in it.
//
// Parameters:
//   - name:  Required name of the section.
//   - color: Optional CSS color value (e.g., "#ff0000", "red").
//     Pass an empty string to use the default color.
func (s *Sequence) OpenSection(name, color string) {
	if name == "" {
		return
	}
	if color == "" {
		color = "#000000"
	}
	s.sections = append(s.sections, &section{
		name:   name,
		color:  color,
		height: -10, // negative margin between steps so sections dont overlap
	})
}

// CloseSection closes the last open section
func (s *Sequence) CloseSection() {
	for i := len(s.sections) - 1; i >= 0; i-- {
		sec := s.sections[i]
		// close the last section added that has any step
		if sec.firstStepIndex != nil && sec.lastStepIndex == nil {
			idx := len(s.steps) - 1
			sec.lastStepIndex = &idx
			return
		}
	}
}

// CloseAllSections closes all the sections.
// Use only if you cannot guarantee an open/close sequence for the sections.
func (s *Sequence) CloseAllSections() {
	for i := len(s.sections) - 1; i >= 0; i-- {
		sec := s.sections[i]
		if sec.firstStepIndex != nil && sec.lastStepIndex == nil {
			idx := len(s.steps) - 1
			sec.lastStepIndex = &idx
		}
	}
	// Delete incomplete sections
	complete := []*section{}
	for _, sec := range s.sections {
		if sec.firstStepIndex != nil && sec.lastStepIndex != nil {
			complete = append(complete, sec)
		}
	}
	s.sections = complete
}

// Generate generates a new SVG sequence
func (s *Sequence) Generate() (string, error) {
	if len(s.actors) == 0 {
		return "", fmt.Errorf("sequence has no actors")
	}
	if len(s.steps) == 0 {
		return "", fmt.Errorf("sequence has no steps")
	}
	err := s.setup()
	if err != nil {
		return "", err
	}

	totalWidth := s.totalWidth()
	totalHeight := s.totalHeight()

	root := svg{
		Xmlns:               "http://www.w3.org/2000/svg",
		Width:               s.width,
		Height:              s.height,
		ViewBox:             fmt.Sprintf("0 0 %d %d", totalWidth, totalHeight),
		PreserveAspectRatio: "xMinYMin meet",
	}

	// Definitions
	root.Elements = append(root.Elements,
		svgDefs{
			Elements: []any{
				svgStyle{Content: defaultCSS},

				marker{
					ID: "seq-dot", ViewBox: "0 0 10 10", MarkerWidth: 5, MarkerHeight: 5, RefX: 5, RefY: 5,
					Elements: []any{
						circle{CX: 5, CY: 5, R: 3, Fill: "context-fill"},
					},
				},

				marker{
					ID: "seq-arrow", ViewBox: "0 0 10 10", MarkerWidth: 5, MarkerHeight: 5, RefX: 5, RefY: 5, Orient: "auto-start-reverse",
					Elements: []any{
						path{D: "M 0 0 L 10 5 L 0 10 z", Fill: "context-fill"},
					},
				},
			},
		})

	// Background
	root.Elements = append(root.Elements,
		rect{X: 0, Y: 0, Width: float64(totalWidth), Height: float64(totalHeight), Fill: "#FFFFFF"},
	)

	// Draw actors
	x := margin + s.distance/2
	y := actorFontSize + 2
	for _, name := range s.actors {
		a := s.actorsMap[name]

		root.Elements = append(root.Elements,
			// Actor line
			line{X1: float64(x), Y1: float64(y + dashArraySize), X2: float64(x), Y2: float64(totalHeight), Stroke: "#CCCCCC", StrokeDasharray: fmt.Sprintf("%[1]d %[1]d", dashArraySize), StrokeWidth: 2},
			// Actor text
			text{X: float64(x), Y: float64(y), FontSize: strconv.Itoa(actorFontSize), Stroke: "none", Fill: "#000000", TextAnchor: "middle", Content: name},
		)

		a.x = float64(x)
		x += s.distance
	}

	// Compute steps and section values
	for _, st := range s.steps {
		srcAct := s.actorsMap[st.SourceActor]
		tgtAct := s.actorsMap[st.TargetActor]
		st.x1 = srcAct.x
		st.x2 = tgtAct.x

		if st.section != nil {
			stHeight := s.getHeight(st)
			st.section.height += stHeight

			minSecY := max(0, st.y-float64(stHeight)+float64(s.stepHeight)/2.0)
			if st.section.y == 0 || st.section.y > minSecY {
				st.section.y = minSecY
			}

			minSecX := max(1.0, min(st.x1, st.x2)-float64(s.distance/2.0))
			if st.section.x == 0 || st.section.x > minSecX {
				st.section.x = minSecX
			}

			maxSecX := max(st.x1, st.x2) + float64(s.distance/2.0)
			if st.section.x2 == 0 || st.section.x2 < maxSecX {
				st.section.x2 = maxSecX
			}

			sw := max(0, math.Abs(st.section.x-st.section.x2))
			if st.section.width < sw {
				st.section.width = sw
			}
		}
	}

	// Draw sections
	for _, sec := range s.sections {
		if !s.verticalSectionText {
			// Offset the sections to make space for horizontal labels
			sec.height -= 4
			sec.y += 2
			sec.x2 -= 2
		}

		var secText *text
		if s.verticalSectionText {
			secText = &text{X: sec.x, Y: sec.y - (float64(sec.height / 2.0)), Transform: fmt.Sprintf("rotate(180,%d,%d)", int(sec.x-4), int(sec.y)), Fill: sec.color, Stroke: "none", FontSize: "10", TextAnchor: "middle", WritingMode: "tb", Content: sec.name}
		} else {
			secText = &text{X: sec.x, Y: sec.y - 2, Fill: sec.color, Stroke: "none", FontSize: "10", TextAnchor: "start", Content: sec.name}
		}
		root.Elements = append(root.Elements,
			rect{X: sec.x, Y: sec.y, Height: float64(sec.height), Width: float64(sec.width), Fill: sec.color, FillOpacity: 0.1, Stroke: sec.color, StrokeWidth: 1},
			*secText,
		)
	}

	// Draw steps
	var x2 float64
	for _, st := range s.steps {
		if st.x1 == st.x2 {
			// dot
			root.Elements = append(root.Elements,
				circle{CX: st.x1, CY: st.y, R: 4, Fill: st.Color},
			)
		} else {
			if st.x1 < st.x2 {
				x2 = st.x2 - 5
			} else {
				x2 = st.x2 + 5
			}
			// arrow
			root.Elements = append(root.Elements,
				line{X1: st.x1, Y1: st.y, X2: x2, Y2: st.y, Fill: st.Color, Stroke: st.Color, StrokeWidth: 2, MarkerStart: "url(#seq-dot)", MarkerEnd: "url(#seq-arrow)"},
			)
		}

		// description
		if st.Description != "" {
			parts := strings.Split(st.Description, "\n")
			offset := float64(descriptionOffset)
			for i := len(parts) - 1; i >= 0; i-- {
				p := parts[i]
				root.Elements = append(root.Elements,
					text{Class: "seq-desc", X: float64(st.x1+st.x2) / 2, Y: st.y - offset, Fill: st.Color, Stroke: "none", FontSize: "10", TextAnchor: "middle", Content: p},
				)
				offset += descriptionOffset * descriptionOffsetFactor
			}
		}
	}

	var sb strings.Builder
	encoder := xml.NewEncoder(&sb)
	encoder.Indent("", "  ")
	if err := encoder.Encode(root); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// getHeight returns the height of the step including the text description offset
func (s *Sequence) getHeight(st *Step) int {
	height := s.stepHeight
	incr := len(strings.Split(st.Description, "\n")) - 1
	height += int((descriptionOffset * descriptionOffsetFactor) * incr)
	return height
}

// ensureActor ensures that an actor exists
// if it does not, the actor is appended (thus appears the last)
func (s *Sequence) ensureActor(a string) {
	if !slices.Contains(s.actors, a) {
		s.actors = append(s.actors, a)
		s.actorsMap[a] = &actor{}
	}
}

// setup initializes the sequence
func (s *Sequence) setup() error {
	// Check that all steps defined the actors
	for i, step := range s.steps {
		if step.SourceActor == "" || step.TargetActor == "" {
			return fmt.Errorf("step #%d defined an actor with an empty name", i+1)
		}
	}

	// Delete empty sections
	fullSections := []*section{}
	for _, sec := range s.sections {
		if sec.firstStepIndex != nil {
			fullSections = append(fullSections, sec)
		}
	}
	s.sections = fullSections

	// Check that all sections have been closed
	for _, sec := range s.sections {
		if sec.lastStepIndex == nil {
			return fmt.Errorf("found open section: %s", sec.name)
		}
	}

	return nil
}

// totalWidth returns the total width of the SVG
func (s *Sequence) totalWidth() int {
	width := margin * 2
	for range s.actorsMap {
		width += s.distance
	}
	return width
}

// totalHeight returns the total height of the SVG
func (s *Sequence) totalHeight() int {
	height := actorFontSize + 2
	for _, st := range s.steps {
		height += s.getHeight(st)
	}
	height += s.stepHeight / 2 // extra margin
	// ensure the height fits the dash-array so the sequence looks better
	for height%dashArraySize != 0 {
		height++
	}
	return height
}
