// SPDX-License-Identifier: MIT

package svgsequence

import (
	_ "embed"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

//go:embed default.css
var defaultCSS string

const (
	margin                  = 20                  // left and right margins
	defaultDistance         = 180                 // default distance between actors
	defaultStepHeight       = 50                  // default height for each step
	actorFontSize           = 16                  // actor font size
	dashArraySize           = 4                   // actor line stroke dash-array size
	lifelineStrokeWidth     = 1                   // actor line stroke width
	descriptionOffset       = 7                   // text description offset against the step line
	descriptionOffsetFactor = 2                   // how much is increased the offset for each line in a multiline description
	descriptionFontSize     = 10                  // step description font size
	sectionFontSize         = descriptionFontSize // section label font size
	monoCharWidthRatio      = 0.6                 // approx glyph width as a fraction of font-size for the monospace description font
	descriptionPadding      = 10                  // horizontal padding kept clear on each side of a step description
	ellipsis                = "…"                 // appended to a step description line truncated to fit
)

type actor struct {
	x float64
}

type section struct {
	name           string
	color          string
	bordered       bool
	firstStepIndex *int
	lastStepIndex  *int

	x, x2, y float64
	width    float64
	height   int
}

type Step struct {
	// Text: Optional text displayed above the arrow or mark.
	Text string

	// Source: Required name of the actor that initiates the action.
	Source string

	// Target: Required name of the actor that receives the action.
	//
	// It can be the same as sourceActor.
	Target string

	// Color: Optional CSS color value (e.g., "#ff0000", "red").
	//
	// Pass an empty string to use the default color.
	Color string

	x1       float64 // Source Actor x
	x2       float64 // Target Actor x
	y        float64
	sections []*section
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

// SetDistance sets the distance between actors.
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

// SetVerticalSectionText sets the section text vertically on the left.
func (s *Sequence) SetVerticalSectionText(b bool) {
	s.verticalSectionText = b
}

// AddActors adds the given actors to the sequence, in order.
//
// Use this to ensure the order of the actors in the sequence.
func (s *Sequence) AddActors(actors ...string) {
	seen := make(map[string]struct{}, len(actors))

	newActors := make([]string, 0, len(actors))
	for _, a := range actors {
		if a == "" {
			continue
		}

		if _, ok := s.actorsMap[a]; !ok {
			s.actorsMap[a] = &actor{}
		}

		if _, dup := seen[a]; !dup {
			seen[a] = struct{}{}
			newActors = append(newActors, a)
		}
	}

	remaining := make([]string, 0, len(s.actors))
	for _, a := range s.actors {
		if _, inNew := seen[a]; !inNew {
			remaining = append(remaining, a)
		}
	}

	s.actors = append(newActors, remaining...) // nolint: gocritic
}

// AppendActors ensures that an actor exists
// if it does not, the actor is appended (thus appears the last).
func (s *Sequence) AppendActors(actors ...string) {
	for _, a := range actors {
		if _, ok := s.actorsMap[a]; !ok {
			s.actorsMap[a] = &actor{}
			s.actors = append(s.actors, a)
		}
	}
}

// Actors returns the current list of actors.
func (s *Sequence) Actors() []string {
	return s.actors
}

// AddStep adds a new step to the sequence diagram.
func (s *Sequence) AddStep(step Step) {
	if step.Color == "" {
		step.Color = "#000000"
	}

	// step.y is computed later in Generate(), once the final stepHeight is
	// known; computing it here would go stale if SetStepHeight is called
	// again before Generate().

	// associate step with all currently open sections; closed sections must
	// not be touched, or a section closed with no steps of its own would get
	// a firstStepIndex stamped in from a later, unrelated step.
	for _, sec := range s.sections {
		if sec.lastStepIndex != nil {
			continue
		}

		if sec.firstStepIndex == nil {
			idx := len(s.steps)
			sec.firstStepIndex = &idx
		}

		step.sections = append(step.sections, sec)
	}

	if step.Source != "" {
		s.AppendActors(step.Source)
	}

	if step.Target != "" {
		s.AppendActors(step.Target)
	}

	s.steps = append(s.steps, &step)
}

// SectionConfig holds optional configuration for a section.
type SectionConfig struct {
	Color         string // Optional CSS color value (e.g., " #ff0000", "red").
	WithoutBorder bool   // Section is drawn without a border.
}

// OpenSection opens a new section to the sequence diagram.
// An open section must be closed after adding the steps which should be placed in it.
//
// Parameters:
//   - name:   Required name of the section.
//   - config: Optional 'SectionConfig' configuration. Pass nil to use defaults.
func (s *Sequence) OpenSection(name string, cfg *SectionConfig) {
	if name == "" {
		return
	}

	sec := &section{
		name:     name,
		color:    "#000000",
		bordered: true,
		height:   -10, // negative margin between steps so sections dont overlap
	}

	if cfg != nil {
		if cfg.Color != "" {
			sec.color = cfg.Color
		}

		sec.bordered = !cfg.WithoutBorder
	}

	s.sections = append(s.sections, sec)
}

// CloseSection closes the last open section, whether or not it has any step.
func (s *Sequence) CloseSection() {
	for _, sec := range slices.Backward(s.sections) {
		if sec.lastStepIndex == nil {
			idx := len(s.steps) - 1
			sec.lastStepIndex = &idx

			return
		}
	}
}

// CloseAllSections closes all the sections.
// Use only if you cannot guarantee an open/close sequence for the sections.
func (s *Sequence) CloseAllSections() {
	for _, sec := range slices.Backward(s.sections) {
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

// Generate generates a new SVG sequence.
func (s *Sequence) Generate() (string, error) {
	if len(s.actors) == 0 {
		return "", errors.New("sequence has no actors")
	}

	if len(s.steps) == 0 {
		return "", errors.New("sequence has no steps")
	}

	err := s.setup()
	if err != nil {
		return "", err
	}

	totalWidth := s.totalWidth()
	totalHeight := s.totalHeight()

	root := svg{
		Xmlns:               "http://www.w3.org/2000/svg", // nolint: revive
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
			line{X1: float64(x), Y1: float64(y + dashArraySize), X2: float64(x), Y2: float64(totalHeight), Stroke: "#CCCCCC", StrokeDasharray: fmt.Sprintf("%[1]d %[1]d", dashArraySize), StrokeWidth: lifelineStrokeWidth},
			// Actor text
			text{X: float64(x), Y: float64(y), FontSize: strconv.Itoa(actorFontSize), Stroke: "none", Fill: "#000000", TextAnchor: "middle", Content: name},
		)

		a.x = float64(x)
		x += s.distance
	}

	// Compute steps and section values
	stepY := float64(actorFontSize + 2)

	for _, st := range s.steps {
		srcAct := s.actorsMap[st.Source]
		tgtAct := s.actorsMap[st.Target]
		st.x1 = srcAct.x
		st.x2 = tgtAct.x

		stHeight := s.getHeight(st)
		stepY += float64(stHeight)
		st.y = stepY

		minSecY := max(0, st.y-float64(stHeight)+float64(s.stepHeight)/2.0)
		minSecX := max(1.0, min(st.x1, st.x2)-float64(s.distance)/2.0)
		maxSecX := max(st.x1, st.x2) + float64(s.distance)/2.0

		for _, sec := range st.sections {
			sec.height += stHeight

			if sec.y == 0 || sec.y > minSecY {
				sec.y = minSecY
			}

			if sec.x == 0 || sec.x > minSecX {
				sec.x = minSecX
			}

			if sec.x2 == 0 || sec.x2 < maxSecX {
				sec.x2 = maxSecX
			}

			sw := max(0, math.Abs(sec.x-sec.x2))
			if sec.width < sw {
				sec.width = sw
			}
		}
	}

	// Draw sections
	for _, sec := range s.sections {
		if !s.verticalSectionText {
			// Offset the sections to make space for horizontal labels
			sec.height -= 4
			sec.y += 2
			sec.width -= 2
		}

		var secText *text
		if s.verticalSectionText {
			secText = &text{X: sec.x, Y: sec.y - (float64(sec.height / 2.0)), Transform: fmt.Sprintf("rotate(180,%d,%d)", int(sec.x-4), int(sec.y)), Fill: sec.color, Stroke: "none", FontSize: strconv.Itoa(sectionFontSize), TextAnchor: "middle", WritingMode: "tb", Content: sec.name}
		} else {
			secText = &text{X: sec.x, Y: sec.y - 2, Fill: sec.color, Stroke: "none", FontSize: strconv.Itoa(sectionFontSize), TextAnchor: "start", Content: sec.name}
		}

		secElem := rect{X: sec.x, Y: sec.y, Height: float64(sec.height), Width: float64(sec.width), Fill: sec.color, FillOpacity: 0.1}
		if sec.bordered {
			secElem.Stroke = sec.color
			secElem.StrokeWidth = 1
		}

		root.Elements = append(root.Elements, secElem, *secText)
	}

	// Draw steps
	var x2 float64
	for _, st := range s.steps {
		if st.x1 == st.x2 {
			// dot
			root.Elements = append(root.Elements,
				circle{CX: st.x1, CY: st.y, R: 3, Fill: st.Color},
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
		if st.Text != "" {
			parts := strings.Split(st.Text, "\n")
			offset := float64(descriptionOffset)

			// available horizontal space before a line risks overlapping neighboring lanes
			maxWidth := max(float64(s.distance), math.Abs(st.x2-st.x1)) - 2*descriptionPadding

			for _, p := range slices.Backward(parts) {
				line, truncated := truncateLine(p, maxWidth)

				t := text{Class: "seq-desc", X: float64(st.x1+st.x2) / 2, Y: st.y - offset, Fill: st.Color, Stroke: "none", FontSize: strconv.Itoa(descriptionFontSize), TextAnchor: "middle", Content: line}
				if truncated {
					// full, untruncated text (newlines included) shown as a native tooltip on mouse over
					t.Title = &title{Content: st.Text}
				}

				root.Elements = append(root.Elements, t)
				offset += descriptionOffset * descriptionOffsetFactor
			}
		}
	}

	var sb strings.Builder

	encoder := xml.NewEncoder(&sb)
	encoder.Indent("", "  ")

	err = encoder.Encode(root)
	if err != nil {
		return "", err
	}

	return sb.String(), nil
}

// truncateLine shortens a single line of step-description text so that it
// roughly fits within maxWidth, appending an ellipsis when it does not fit.
// It reports whether the line was truncated.
func truncateLine(line string, maxWidth float64) (string, bool) {
	maxChars := max(int(maxWidth/(descriptionFontSize*monoCharWidthRatio)), 1)

	runes := []rune(line)
	if len(runes) <= maxChars {
		return line, false
	}

	if maxChars == 1 {
		return ellipsis, true
	}

	return string(runes[:maxChars-1]) + ellipsis, true
}

// getHeight returns the height of the step including the text description offset.
func (s *Sequence) getHeight(st *Step) int {
	height := s.stepHeight
	incr := len(strings.Split(st.Text, "\n")) - 1
	height += (descriptionOffset * descriptionOffsetFactor) * incr

	return height
}

// setup initializes the sequence.
func (s *Sequence) setup() error {
	// Check that all steps defined the actors
	for i, step := range s.steps {
		if step.Source == "" || step.Target == "" {
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

// totalWidth returns the total width of the SVG.
func (s *Sequence) totalWidth() int {
	return margin*2 + s.distance*len(s.actorsMap)
}

// totalHeight returns the total height of the SVG.
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
