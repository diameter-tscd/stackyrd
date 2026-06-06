package tui

import (
	"fmt"
	"math"
	"strings"
)

// BarChart represents a simple ASCII bar chart
type BarChart struct {
	Title      string
	Items      []ChartItem
	Width      int
	ShowValues bool
}

// ChartItem represents an item in a chart
type ChartItem struct {
	Label string
	Value float64
	Color string
}

// NewBarChart creates a new bar chart
func NewBarChart(title string, width int) *BarChart {
	if width <= 0 {
		width = 40
	}
	return &BarChart{
		Title:      title,
		Width:      width,
		ShowValues: true,
	}
}

// AddItem adds an item to the chart
func (bc *BarChart) AddItem(label string, value float64, color string) {
	bc.Items = append(bc.Items, ChartItem{
		Label: label,
		Value: value,
		Color: color,
	})
}

// Render renders the bar chart as a string
func (bc *BarChart) Render() string {
	if len(bc.Items) == 0 {
		return ""
	}

	var sb strings.Builder

	if bc.Title != "" {
		sb.WriteString(bc.Title + "\n")
		sb.WriteString(strings.Repeat("─", bc.Width) + "\n")
	}

	maxValue := 0.0
	maxLabelLen := 0

	for _, item := range bc.Items {
		if item.Value > maxValue {
			maxValue = item.Value
		}
		if len(item.Label) > maxLabelLen {
			maxLabelLen = len(item.Label)
		}
	}

	if maxValue == 0 {
		maxValue = 1
	}

	for _, item := range bc.Items {
		label := fmt.Sprintf("%-*s", maxLabelLen, item.Label)
		barLen := int((item.Value / maxValue) * float64(bc.Width-maxLabelLen-10))

		if barLen < 0 {
			barLen = 0
		}

		bar := strings.Repeat("█", barLen)

		if bc.ShowValues {
			fmt.Fprintf(&sb, "%s %s %.1f\n", label, bar, item.Value)
		} else {
			sb.WriteString(fmt.Sprintf("%s %s\n", label, bar))
		}
	}

	return sb.String()
}

// Sparkline represents a simple ASCII sparkline
type Sparkline struct {
	Title  string
	Values []float64
	Width  int
	Chars  []string
}

// NewSparkline creates a new sparkline
func NewSparkline(title string, width int) *Sparkline {
	if width <= 0 {
		width = 20
	}
	return &Sparkline{
		Title: title,
		Width: width,
		Chars: []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"},
	}
}

// AddValue adds a value to the sparkline
func (s *Sparkline) AddValue(value float64) {
	s.Values = append(s.Values, value)
}

// SetValues sets multiple values
func (s *Sparkline) SetValues(values []float64) {
	s.Values = values
}

// Render renders the sparkline as a string
func (s *Sparkline) Render() string {
	if len(s.Values) == 0 {
		return ""
	}

	var sb strings.Builder

	if s.Title != "" {
		sb.WriteString(s.Title + ": ")
	}

	values := s.Values
	if len(values) > s.Width {
		step := float64(len(values)) / float64(s.Width)
		sampled := make([]float64, s.Width)
		for i := 0; i < s.Width; i++ {
			idx := int(float64(i) * step)
			if idx >= len(values) {
				idx = len(values) - 1
			}
			sampled[i] = values[idx]
		}
		values = sampled
	}

	minVal, maxVal := math.Inf(1), math.Inf(-1)
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	rangeVal := maxVal - minVal
	if rangeVal == 0 {
		rangeVal = 1
	}

	for _, v := range values {
		normalized := (v - minVal) / rangeVal
		idx := int(normalized * float64(len(s.Chars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(s.Chars) {
			idx = len(s.Chars) - 1
		}
		sb.WriteString(s.Chars[idx])
	}

	return sb.String()
}

// Gauge represents a simple ASCII gauge
type Gauge struct {
	Title string
	Value float64
	Max   float64
	Width int
}

// NewGauge creates a new gauge
func NewGauge(title string, width int) *Gauge {
	if width <= 0 {
		width = 20
	}
	return &Gauge{
		Title: title,
		Width: width,
		Max:   100,
	}
}

// SetValue sets the gauge value
func (g *Gauge) SetValue(value float64) {
	g.Value = value
}

// SetMax sets the gauge max value
func (g *Gauge) SetMax(max float64) {
	g.Max = max
}

// Render renders the gauge as a string
func (g *Gauge) Render() string {
	var sb strings.Builder

	if g.Title != "" {
		sb.WriteString(g.Title + ": ")
	}

	percentage := g.Value / g.Max
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 1 {
		percentage = 1
	}

	filled := int(percentage * float64(g.Width))
	empty := g.Width - filled

	sb.WriteString("[")
	sb.WriteString(strings.Repeat("█", filled))
	sb.WriteString(strings.Repeat("░", empty))
	sb.WriteString("]")

	sb.WriteString(fmt.Sprintf(" %.1f%%", percentage*100))

	return sb.String()
}

// Table represents a simple ASCII table
type Table struct {
	Title   string
	Headers []string
	Rows    [][]string
	Width   int
}

// NewTable creates a new table
func NewTable(title string, width int) *Table {
	if width <= 0 {
		width = 80
	}
	return &Table{
		Title: title,
		Width: width,
	}
}

// SetHeaders sets the table headers
func (t *Table) SetHeaders(headers ...string) {
	t.Headers = headers
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) {
	t.Rows = append(t.Rows, cells)
}

// Render renders the table as a string
func (t *Table) Render() string {
	var sb strings.Builder

	if t.Title != "" {
		sb.WriteString(t.Title + "\n")
	}

	if len(t.Headers) == 0 {
		return sb.String()
	}

	colWidths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		colWidths[i] = len(h)
	}

	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	sb.WriteString("┌")
	for i, w := range colWidths {
		sb.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			sb.WriteString("┬")
		}
	}
	sb.WriteString("┐\n")

	sb.WriteString("│")
	for i, h := range t.Headers {
		sb.WriteString(fmt.Sprintf(" %-*s │", colWidths[i], h))
	}
	sb.WriteString("\n")

	sb.WriteString("├")
	for i, w := range colWidths {
		sb.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			sb.WriteString("┼")
		}
	}
	sb.WriteString("┤\n")

	for _, row := range t.Rows {
		sb.WriteString("│")
		for i, cell := range row {
			if i < len(colWidths) {
				sb.WriteString(fmt.Sprintf(" %-*s │", colWidths[i], cell))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("└")
	for i, w := range colWidths {
		sb.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			sb.WriteString("┴")
		}
	}
	sb.WriteString("┘\n")

	return sb.String()
}
