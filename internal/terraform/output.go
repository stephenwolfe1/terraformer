package terraform

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

type palette struct {
	accent, yellow, green, red, reset string
}

var colors = palette{
	accent: "\x1b[94m",
	yellow: "\x1b[33m",
	green:  "\x1b[32m",
	red:    "\x1b[31m",
	reset:  "\x1b[0m",
}

// SetColor enables or disables ANSI colors in wrapper status lines. Colors are
// on by default unless NO_COLOR is set (https://no-color.org).
func SetColor(enabled bool) {
	if enabled {
		colors = palette{accent: "\x1b[94m", yellow: "\x1b[33m", green: "\x1b[32m", red: "\x1b[31m", reset: "\x1b[0m"}
		return
	}
	colors = palette{}
}

func init() {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		SetColor(false)
	}
}

const marker = "==>"

func prefix() string { return colors.accent + marker + colors.reset + " " }

// PrintCmdStart prints the banner shown before a terraform command runs.
func PrintCmdStart(w io.Writer, cmdName, service, fullCmd string) {
	fmt.Fprintf(w, "\n%s%sSTART  [%s%s%s]  service=%s%s\n", prefix(), colors.yellow, colors.reset, cmdName, colors.yellow, colors.reset, service)
	fmt.Fprintf(w, "%s%s%s%s\n\n", prefix(), colors.yellow, fullCmd, colors.reset)
}

// PrintCmdFinish prints the banner shown after a terraform command exits.
func PrintCmdFinish(w io.Writer, cmdName, service string, err error) {
	if err != nil {
		fmt.Fprintf(w, "\n%s%sFINISH [%s%s%s]  service=%s%s  %s(failed: %v)%s\n\n", prefix(), colors.yellow, colors.reset, cmdName, colors.yellow, colors.reset, service, colors.red, err, colors.reset)
		return
	}
	fmt.Fprintf(w, "\n%s%sFINISH [%s%s%s]  service=%s%s  %s(ok)%s\n\n", prefix(), colors.yellow, colors.reset, cmdName, colors.yellow, colors.reset, service, colors.green, colors.reset)
}

// PrintStatus prints a single wrapper status line.
func PrintStatus(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s%s%s\n", prefix(), colors.yellow, msg, colors.reset)
}

// OutputMode controls how stdout from terraform is handled.
type OutputMode int

const (
	OutputNormal     OutputMode = iota // pass stdout through as-is
	OutputSilent                       // discard stdout
	OutputPlanFilter                   // hide refresh noise, show plan output
)

// PlanFilter buffers terraform stdout and only passes it through once actual
// plan output begins (after the state-refresh noise).
type PlanFilter struct {
	Out     io.Writer
	buf     bytes.Buffer
	passing bool
}

// planMarker is a string that makes the filter start passing output through.
// contextLines is how many preceding lines to include as well.
type planMarker struct {
	text         string
	contextLines int
}

var planMarkers = []planMarker{
	{"Terraform used the selected providers", 0},
	{"No changes.", 0},
	{"Changes to Outputs:", 0},
	{"Planning failed.", 0},
	{"Enter a value:", 3}, // include var name, description, and blank line
	{"Do you want to perform these actions", 0},
	{"Do you really want to destroy", 0},
}

func (pf *PlanFilter) Write(p []byte) (int, error) {
	if pf.passing {
		_, err := pf.Out.Write(p)
		return len(p), err
	}
	pf.buf.Write(p)

	content := pf.buf.String()
	best := -1
	var bestMarker planMarker
	for _, m := range planMarkers {
		if idx := strings.Index(content, m.text); idx >= 0 && (best < 0 || idx < best) {
			best, bestMarker = idx, m
		}
	}
	if best < 0 {
		return len(p), nil
	}

	start := 0
	before := content[:best]
	for i := 0; i <= bestMarker.contextLines; i++ {
		pos := strings.LastIndex(before, "\n")
		if pos < 0 {
			start = 0
			break
		}
		start = pos + 1
		before = before[:pos]
	}
	pf.passing = true
	_, err := pf.Out.Write([]byte(content[start:]))
	pf.buf.Reset()
	return len(p), err
}

// Flush writes any remaining buffered content (safety net for unexpected output formats).
func (pf *PlanFilter) Flush() {
	if !pf.passing && pf.buf.Len() > 0 {
		_, _ = pf.Out.Write(pf.buf.Bytes())
		pf.buf.Reset()
	}
}
