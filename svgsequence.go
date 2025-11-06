// SPDX-License-Identifier: MIT

package svgsequence

import (
	_ "embed"
	"fmt"
	"math"
	"slices"
	"strings"
)

//go:embed defs.xml
var defs string

//go:embed default.css
var defaultCSS string

const (
	margin                  = 20                // left and right margins
	defaultDistance         = 180               // default distance between actors
	stepHeight              = 50                // height for each step
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

// getHeight returns the height of the step including the text description offset
func (st Step) getHeight() int {
	height := stepHeight
	incr := len(strings.Split(st.Description, "\n")) - 1
	height += int((descriptionOffset * descriptionOffsetFactor) * incr)
	return height
}

type Sequence struct {
	actors    []string
	actorsMap map[string]*actor   // map[actorName]actor
	sections  map[string]*section // map[sectionName]section
	steps     []*Step

	width, height string // SVG width and height (not the viewport)
	distance      int    // distance between actors
}

func NewSequence() *Sequence {
	return &Sequence{
		actorsMap: make(map[string]*actor),
		sections:  make(map[string]*section),
		width:     "100%",
		height:    "100%",
		distance:  defaultDistance,
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
	y += stepHeight + float64((descriptionOffset*descriptionOffsetFactor)*incr)
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
	name = escapeXML(name)
	s.sections[name] = &section{
		name:   name,
		color:  color,
		height: -10, // negative margin between steps so sections dont overlap
	}
}

// CloseSection closes the open section
func (s *Sequence) CloseSection() {
	for name, sec := range s.sections {
		// when closing a section, check if it had any step in it (it does
		// if firstStep != nil) and add the lastStep
		// otherwise delete the section as it is empty
		if sec.firstStepIndex == nil {
			delete(s.sections, name)
		} else {
			idx := len(s.steps) - 1
			sec.lastStepIndex = &idx
		}
	}
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

	var sb strings.Builder

	// SVG header
	sb.WriteString(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %d %d" preserveAspectRatio="xMinYMin meet">`,
		s.width, s.height, totalWidth, totalHeight,
	))
	sb.WriteString("\n")

	// Definitions
	sb.WriteString("<defs>\n<style>\n" + defaultCSS + "\n</style>\n" + defs + "\n</defs>\n")

	// Draw actors
	x := margin + s.distance/2
	y := actorFontSize + 2
	for _, name := range s.actors {
		a := s.actorsMap[name]

		// Actor line
		sb.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#CCCCCC" stroke-dasharray="%d %d" stroke-width="2" />`,
			x, y+dashArraySize, x, totalHeight, dashArraySize, dashArraySize))
		sb.WriteString("\n")

		// Actor text
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="%d"`, x, y, actorFontSize))
		sb.WriteString(fmt.Sprintf(` stroke="none" fill="#000000" text-anchor="middle">%s</text>`, name))
		sb.WriteString("\n")

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
			stHeight := st.getHeight()
			st.section.height += stHeight

			minSecY := max(0, st.y-float64(stHeight)+stepHeight/2)
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
		sb.WriteString(fmt.Sprintf(`<rect fill="%s" fill-opacity="0.1" height="%d" width="%f" x="%f" y="%f" />`,
			sec.color, sec.height, sec.width, sec.x, sec.y))
		sb.WriteString(fmt.Sprintf(`<rect fill="none"  stroke="%s" stroke-width="1" height="%d" width="%f" x="%f" y="%f" />`,
			sec.color, sec.height, sec.width, sec.x, sec.y))
		sb.WriteString(fmt.Sprintf(`<text fill="%s" stroke="none" font-size="10" text-anchor="middle" writing-mode="tb" transform="rotate(180,%d,%d)" x="%f" y="%f">%s</text>`,
			sec.color, int(sec.x)-4, int(sec.y), sec.x, sec.y-(float64(sec.height/2.0)), sec.name))
	}

	// Draw steps
	var x2 float64
	for _, st := range s.steps {
		if st.x1 == st.x2 {
			// dot
			sb.WriteString(fmt.Sprintf(`<circle fill="%s" cx="%f" cy="%f" r="4" />`, st.Color, st.x1, st.y))
		} else {
			if st.x1 < st.x2 {
				x2 = st.x2 - 5
			} else {
				x2 = st.x2 + 5
			}
			// arrow
			sb.WriteString(fmt.Sprintf(`<line marker-start="url(#seq-dot)" marker-end="url(#seq-arrow)" fill="%s" stroke="%s" stroke-width="2" x1="%f" x2="%f" y1="%f" y2="%f" />`,
				st.Color, st.Color, st.x1, x2, st.y, st.y))
		}
		sb.WriteString("\n")

		// description
		if st.Description != "" {
			parts := strings.Split(st.Description, "\n")
			offset := float64(descriptionOffset)
			for i := len(parts) - 1; i >= 0; i-- {
				p := parts[i]
				sb.WriteString(fmt.Sprintf(`<text class="seq-desc" fill="%s" stroke="none" font-size="10" text-anchor="middle" x="%f" y="%f">%s</text>`,
					st.Color, float64(st.x1+st.x2)/2, st.y-offset, escapeXML(p)))
				sb.WriteString("\n")
				offset += descriptionOffset * descriptionOffsetFactor
			}
		}
	}

	sb.WriteString("</svg>\n")
	return sb.String(), nil
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

	// Check that all sections have been closed
	for name, sec := range s.sections {
		if sec.lastStepIndex == nil {
			return fmt.Errorf("found open section: %s", name)
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
		height += st.getHeight()
	}
	height += stepHeight / 2 // extra margin
	// ensure the height fits the dash-array so the sequence looks better
	for height%dashArraySize != 0 {
		height++
	}
	return height
}

// escapeXML escapes special XML characters
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
