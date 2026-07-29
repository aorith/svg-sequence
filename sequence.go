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
	defaultDistance   = 160 // default horizontal distance between actors
	defaultStepHeight = 50  // default vertical space allotted to one step

	leftMargin = 20 // outer canvas margin and the left x of section boxes
	topMargin  = 28 // space reserved between actor text and first step

	actorFontSize   = 20
	descFontSize    = 12
	sectionFontSize = 14

	headerGap    = 10                        // gap between the actor label baseline and the lifeline start
	headerHeight = actorFontSize + headerGap // y where lifelines/steps begin

	lineHeight = descFontSize + 4 // vertical space per description text line
	textGap    = 8                // gap between a step's arrow/mark and its nearest text baseline

	selfLoopRX = 9 // self-loop arc horizontal radius
	selfLoopRY = 6 // self-loop arc vertical radius

	arrowMargin = 5 // arrow line margin towards the actor lifeline

	sectionFirstStepMargin = 14 // margin between the section's top and the first step
	sectionBottomMargin    = 10 // margin reserved below a section's last step
	sectionLabelMargin     = 4  // margin between a section box's border and its label text

	charWidthFactor = 0.6 // to adjust text against a fraction of descFontSize
)

type actor struct {
	x float64
}

type section struct {
	name     string
	color    string
	bordered bool
	link     string
	hasSteps bool
	closed   bool

	x, x2, y, yBottom float64
	lastMemberY       float64 // y of the last step associated with this section
}

type Step struct {
	// Text: Optional text displayed above the arrow or mark.
	Text string

	// Source: Name of the actor that initiates the action.
	//
	// At least one of Source or Target is required. If only one is set, the
	// step is placed on that actor's lifeline with no arrow drawn, as if
	// Source and Target were the same actor.
	Source string

	// Target: Name of the actor that receives the action.
	//
	// It can be the same as Source. At least one of Source or Target is
	// required. If only one is set, the step is placed on that actor's
	// lifeline with no arrow drawn, as if Source and Target were the same actor.
	Target string

	// Color: Optional CSS color value (e.g., "#ff0000", "red").
	//
	// Pass an empty string to use the default color.
	Color string

	x1       float64 // Source Actor x
	x2       float64 // Target Actor x
	y        float64
	height   int
	sections []*section
	noArrow  bool // true when Source or Target (not both) was left empty
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

// SetStepHeight sets the height for steps added after this call.
//
// It does not retroactively affect steps already added.
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

		s.ensureActor(a)

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
		if s.ensureActor(a) {
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

	// If only one of Source/Target is given, treat the step as belonging to
	// that single actor: no arrow is drawn, only the text.
	switch {
	case step.Source == "" && step.Target != "":
		step.Source = step.Target
		step.noArrow = true
	case step.Target == "" && step.Source != "":
		step.Target = step.Source
		step.noArrow = true
	default:
	}

	step.height = s.stepHeight
	if step.noArrow {
		step.height /= 2
	}

	// associate step with all currently open sections
	for _, sec := range s.sections {
		if sec.closed {
			continue
		}

		sec.hasSteps = true
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
	Link          string // Optional URL/fragment for the section label
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
	}

	if cfg != nil {
		if cfg.Color != "" {
			sec.color = cfg.Color
		}

		sec.bordered = !cfg.WithoutBorder
		sec.link = cfg.Link
	}

	s.sections = append(s.sections, sec)
}

// CloseSection closes the last open section, whether or not it has any step.
func (s *Sequence) CloseSection() {
	for _, sec := range slices.Backward(s.sections) {
		if !sec.closed {
			sec.closed = true

			return
		}
	}
}

// CloseAllSections closes all the sections.
// Use only if you cannot guarantee an open/close sequence for the sections.
func (s *Sequence) CloseAllSections() {
	for _, sec := range slices.Backward(s.sections) {
		if sec.hasSteps && !sec.closed {
			sec.closed = true
		}
	}
	// Delete incomplete sections
	complete := []*section{}

	for _, sec := range s.sections {
		if sec.hasSteps && sec.closed {
			complete = append(complete, sec)
		}
	}

	s.sections = complete
}

// Generate generates a new SVG sequence.
func (s *Sequence) Generate() (string, error) {
	err := s.validate()
	if err != nil {
		return "", err
	}

	totalWidth, totalHeight := s.layout()

	elements := make([]any, 0, 2+len(s.actors)*2+len(s.sections)*2+len(s.steps)*2)
	elements = append(elements, buildDefs())
	elements = append(elements, rect{X: 0, Y: 0, Width: totalWidth, Height: totalHeight, Fill: "#FFFFFF"})
	elements = append(elements, s.buildActors(totalHeight)...)
	elements = append(elements, s.buildSections()...)
	elements = append(elements, s.buildSteps()...)

	root := svg{
		Xmlns:               "http://www.w3.org/2000/svg", // nolint:revive
		Width:               s.width,
		Height:              s.height,
		ViewBox:             fmt.Sprintf("0 0 %s %s", fmtNum(totalWidth), fmtNum(totalHeight)),
		PreserveAspectRatio: "xMinYMin meet",
		Elements:            elements,
	}

	data, err := xml.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// validate drops sections that never received a step, checks that every
// remaining section was closed, and ensures there's at least one actor and
// one step to draw.
func (s *Sequence) validate() error {
	complete := make([]*section, 0, len(s.sections))

	for _, sec := range s.sections {
		if !sec.hasSteps {
			continue
		}

		if !sec.closed {
			return fmt.Errorf("section %q was not closed", sec.name)
		}

		complete = append(complete, sec)
	}

	s.sections = complete

	if len(s.actors) == 0 {
		return errors.New("sequence has no actors")
	}

	if len(s.steps) == 0 {
		return errors.New("sequence has no steps")
	}

	if s.stepHeight < 10 {
		return errors.New("min step height is 10")
	}

	if s.distance < 50 {
		return errors.New("min actor distance is 50")
	}

	return nil
}

// layout assigns every position for the drawn objects and returns
// the total canvas size (width, height).
func (s *Sequence) layout() (float64, float64) {
	distance := float64(s.distance)
	totalWidth := 2*leftMargin + float64(len(s.actors))*distance

	for i, name := range s.actors {
		s.actorsMap[name].x = leftMargin + distance/2 + float64(i)*distance
	}

	for _, step := range s.steps {
		step.x1 = s.actorsMap[step.Source].x
		step.x2 = s.actorsMap[step.Target].x
	}

	cursor := float64(headerHeight + topMargin)
	started := make(map[*section]bool, len(s.sections))

	var prevSections []*section

	for _, step := range s.steps {
		for _, sec := range prevSections {
			if !slices.Contains(step.sections, sec) {
				cursor += sectionBottomMargin

				break
			}
		}

		prevSections = step.sections

		lines := strings.Split(step.Text, "\n")
		textSpan := textGap + float64(len(lines)-1)*lineHeight
		minHalf := textSpan + textGap

		opensSection := false

		for _, sec := range step.sections {
			lo, hi := stepXBounds(step.x1, step.x2, distance, totalWidth)

			if started[sec] {
				sec.x = math.Min(sec.x, lo)
				sec.x2 = math.Max(sec.x2, hi)

				continue
			}

			started[sec] = true
			sec.x, sec.x2 = lo, hi
			sec.y = cursor
			opensSection = true
		}

		if opensSection {
			cursor += sectionFirstStepMargin
		}

		effectiveHeight := math.Max(float64(step.height), 2*minHalf)
		step.y = cursor + effectiveHeight/2

		for _, sec := range step.sections {
			sec.yBottom = cursor + effectiveHeight
			sec.lastMemberY = step.y
		}

		cursor += effectiveHeight
	}

	return totalWidth, cursor
}

// stepXBounds returns a section's horizontal extent.
func stepXBounds(x1, x2, distance, totalWidth float64) (float64, float64) {
	lo := math.Min(x1, x2) - distance/2
	hi := math.Max(x1, x2) + distance/2

	if lo < leftMargin {
		lo = leftMargin
	}

	if hi > totalWidth-leftMargin {
		hi = totalWidth - leftMargin
	}

	return lo, hi
}

// buildDefs returns the <defs> element: the embedded stylesheet and the
// dot/arrowhead markers reused by every step.
func buildDefs() svgDefs {
	arrowPath := path{D: "M 0 0 L 10 5 L 0 10 z", Fill: "context-stroke"}

	return svgDefs{
		Elements: []any{
			svgStyle{Content: defaultCSS},
			marker{
				ID: "seq-dot", ViewBox: "0 0 10 10", MarkerWidth: 5, MarkerHeight: 5, RefX: 5, RefY: 5,
				Elements: []any{circle{CX: 5, CY: 5, R: 3, Fill: "context-stroke"}},
			},
			marker{
				ID: "seq-arrow", ViewBox: "0 0 10 10", MarkerWidth: 5, MarkerHeight: 5, RefX: 5, RefY: 5,
				Orient: "auto-start-reverse", Elements: []any{arrowPath},
			},
			marker{
				ID: "seq-arrow-sm", ViewBox: "0 0 10 10", MarkerWidth: 2.5, MarkerHeight: 2.5, RefX: 5, RefY: 5,
				Orient: "auto-start-reverse", Elements: []any{arrowPath},
			},
		},
	}
}

// buildActors draws every actor's dashed lifeline and its centered label.
func (s *Sequence) buildActors(totalHeight float64) []any {
	elements := make([]any, 0, len(s.actors)*2)

	for _, name := range s.actors {
		x := s.actorsMap[name].x

		elements = append(elements,
			line{X1: x, Y1: headerHeight, X2: x, Y2: totalHeight, Stroke: "#CCCCCC", StrokeWidth: 1, StrokeDasharray: "4 4"},
			text{
				X: x, Y: headerHeight - 4, Fill: "#000000",
				FontSize: strconv.Itoa(actorFontSize), TextAnchor: "middle", Content: name,
			},
		)
	}

	return elements
}

// buildSections draws every section's box and its label.
func (s *Sequence) buildSections() []any {
	elements := make([]any, 0, len(s.sections)*2)

	for _, sec := range s.sections {
		top := sec.y

		bottom := math.Max(sec.yBottom-sectionBottomMargin, sec.lastMemberY)

		r := rect{X: sec.x, Y: top, Width: sec.x2 - sec.x, Height: bottom - top, Fill: sec.color, FillOpacity: 0.1}
		if sec.bordered {
			r.Stroke = sec.color
			r.StrokeWidth = 1
		}

		elements = append(elements, r)

		label := text{Fill: sec.color, Stroke: "none", FontSize: strconv.Itoa(sectionFontSize), Content: sec.name}

		if s.verticalSectionText {
			label.X = sec.x - sectionLabelMargin
			label.Y = sec.y + sectionLabelMargin
			label.TextAnchor = "end"
			label.Transform = fmt.Sprintf("rotate(-90 %s %s)", fmtNum(label.X), fmtNum(label.Y))
		} else {
			label.X = sec.x
			label.Y = sec.y - sectionLabelMargin
			label.TextAnchor = "start"
		}

		if sec.link != "" {
			elements = append(elements, anchor{Href: sec.link, Elements: []any{label}})
		} else {
			elements = append(elements, label)
		}
	}

	return elements
}

// buildSteps draws every step's arrow/mark (if any) and its description text.
func (s *Sequence) buildSteps() []any {
	distance := float64(s.distance)
	elements := make([]any, 0, len(s.steps)*2)

	for _, step := range s.steps {
		switch {
		case step.noArrow:
			elements = append(elements, stepText(step, distance-2*textGap)...)

		case step.Source == step.Target:
			d := fmt.Sprintf("M %s %s A %d %d 0 1 1 %s %s",
				fmtNum(step.x1), fmtNum(step.y), selfLoopRX, selfLoopRY, fmtNum(step.x1), fmtNum(step.y+2*selfLoopRY))

			elements = append(elements,
				path{D: d, Fill: "none", Stroke: step.Color, StrokeWidth: 1.5, MarkerEnd: "url(#seq-arrow-sm)"},
			)
			elements = append(elements, stepText(step, distance-2*textGap)...)

		default:
			x2 := step.x2
			if step.x2 > step.x1 {
				x2 -= arrowMargin
			} else {
				x2 += arrowMargin
			}

			elements = append(elements,
				line{
					X1: step.x1, Y1: step.y, X2: x2, Y2: step.y,
					Stroke: step.Color, StrokeWidth: 2,
					MarkerStart: "url(#seq-dot)", MarkerEnd: "url(#seq-arrow)",
				},
			)
			elements = append(elements, stepText(step, math.Abs(step.x2-step.x1)-4*textGap)...)
		}
	}

	return elements
}

// stepText renders a step's description.
func stepText(step *Step, availWidth float64) []any {
	class := "seq-desc"
	if step.noArrow {
		class = "seq-desc seq-desc-no-arrow"
	}

	lines := strings.Split(step.Text, "\n")
	elements := make([]any, 0, len(lines))

	for i, line := range lines {
		rendered, cut := truncateLine(line, availWidth)

		t := text{
			Class: class, X: (step.x1 + step.x2) / 2, Y: step.y - textGap - float64(len(lines)-1-i)*lineHeight,
			Fill: step.Color, Stroke: "none", FontSize: strconv.Itoa(descFontSize), TextAnchor: "middle",
			Content: rendered,
		}

		if cut {
			t.Title = &title{Content: step.Text}
		}

		elements = append(elements, t)
	}

	return elements
}

// truncateLine shortens line to fit availWidth.
func truncateLine(line string, availWidth float64) (string, bool) {
	charWidth := descFontSize * charWidthFactor
	maxChars := max(int(availWidth/charWidth), 1)

	runes := []rune(line)
	if len(runes) <= maxChars {
		return line, false
	}

	if maxChars == 1 {
		return "…", true
	}

	return string(runes[:maxChars-1]) + "…", true
}

// fmtNum formats a coordinate without a trailing ".0" for whole numbers.
func fmtNum(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ensureActor registers name in actorsMap if it isn't already there and
// reports whether it was newly added.
func (s *Sequence) ensureActor(name string) bool {
	if _, ok := s.actorsMap[name]; ok {
		return false
	}

	s.actorsMap[name] = &actor{}

	return true
}
