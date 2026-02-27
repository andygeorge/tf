package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime/debug"
	"strings"

	"golang.org/x/term"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
// Falls back to the module version embedded by go install.
var version = ""

func getVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// isPreamble returns true for terraform state-refresh and data-read progress
// lines that precede the actual plan output.
func isPreamble(plain string) bool {
	return strings.Contains(plain, ": Refreshing state...") ||
		strings.Contains(plain, ": Still refreshing...") ||
		strings.Contains(plain, ": Reading...") ||
		strings.Contains(plain, ": Still reading...") ||
		strings.Contains(plain, ": Read complete after")
}

// formatResourceHeader strips the "  # " prefix from a terraform resource
// header line and prepends bold ANSI formatting for the resource address.
func formatResourceHeader(line string) string {
	idx := strings.Index(line, "  # ")
	if idx >= 0 {
		return "\x1b[1m" + line[idx+4:]
	}
	return "\x1b[1m" + line
}

// isSeparator returns true for the box-drawing horizontal rule terraform prints
// before the "Note: You didn't use -out..." advisory.
func isSeparator(plain string) bool {
	trimmed := strings.TrimSpace(plain)
	if trimmed == "" {
		return false
	}
	for _, ch := range trimmed {
		if ch != '─' {
			return false
		}
	}
	return true
}

// countBraces returns the net number of open braces on a line ({=+1, }=-1).
func countBraces(s string) int {
	n := 0
	for _, ch := range s {
		if ch == '{' {
			n++
		} else if ch == '}' {
			n--
		}
	}
	return n
}

type filterState int

const (
	stateNormal      filterState = iota
	stateAfterHeader             // inside the resource-change list
	stateInBlock                 // suppressing a resource block's body
)

// ResourceBlock holds a single terraform resource change as parsed from
// plan/apply output. Summary is the formatted single-line bold header.
// Body contains the raw diff lines of the block (may be nil for piped/no-body format).
type ResourceBlock struct {
	Summary string
	Body    []string
}

// parseResult holds the structured output of parseBlocks.
type parseResult struct {
	Blocks []ResourceBlock
	Footer []string
}

// parseBlocks reads terraform plan/apply output from r and returns parsed
// resource blocks (each with summary and body) and any trailing footer lines.
// It applies the same suppression rules as filterOutput.
func parseBlocks(r io.Reader) parseResult {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	var state filterState
	depth := 0
	inNote := false
	inIntroBlock := false
	pendingBlank := false
	hasEmitted := false

	var result parseResult
	var currentSummary string
	var bodyLines []string

	saveBlock := func() {
		if currentSummary == "" {
			return
		}
		result.Blocks = append(result.Blocks, ResourceBlock{
			Summary: currentSummary,
			Body:    bodyLines,
		})
		currentSummary = ""
		bodyLines = nil
	}

	addFooter := func(line string) {
		if pendingBlank && hasEmitted {
			result.Footer = append(result.Footer, "")
		}
		pendingBlank = false
		result.Footer = append(result.Footer, line)
		hasEmitted = true
	}

	for scanner.Scan() {
		line := scanner.Text()
		plain := stripANSI(line)

		// Suppress invisible ANSI-only lines (e.g. colour-reset before separator).
		if plain == "" && line != "" {
			continue
		}
		// Suppress state-refresh preamble.
		if isPreamble(plain) {
			continue
		}
		// Suppress box-drawing separator line.
		if isSeparator(plain) {
			continue
		}
		// Suppress "Note:" advisory and its continuation paragraph.
		if strings.HasPrefix(plain, "Note: ") {
			inNote = true
			continue
		}
		if inNote {
			if plain == "" {
				inNote = false
			}
			continue
		}
		// Suppress "Terraform used the selected providers" block (includes symbol legend).
		if strings.HasPrefix(plain, "Terraform used the selected providers") {
			inIntroBlock = true
			continue
		}
		if inIntroBlock {
			if plain == "" {
				inIntroBlock = false
			}
			continue
		}
		// Suppress "Terraform will perform the following actions:" header.
		if plain == "Terraform will perform the following actions:" {
			continue
		}
		// Suppress "Plan: X to add, Y to change, Z to destroy." summary.
		if strings.HasPrefix(plain, "Plan:") {
			continue
		}

		// Blank lines: record as pending rather than processing immediately.
		if plain == "" {
			pendingBlank = true
			continue
		}

		switch state {
		case stateNormal:
			if strings.HasPrefix(plain, "  # ") {
				state = stateAfterHeader
				trimmed := plain[4:]
				if strings.Contains(trimmed, "module") {
					currentSummary = formatResourceHeader(line)
					pendingBlank = false
					hasEmitted = true
				} else {
					pendingBlank = false
				}
			} else {
				addFooter(line)
			}

		case stateAfterHeader:
			bc := countBraces(plain)
			switch {
			case strings.HasPrefix(plain, "  # ("):
				// Secondary parenthetical comment (e.g. "# (because ...)"); suppress.
				pendingBlank = false
			case strings.HasPrefix(plain, "  # "):
				// Another primary resource header (piped format — no body for previous).
				saveBlock()
				pendingBlank = false
				trimmed := plain[4:]
				if strings.Contains(trimmed, "module") {
					currentSummary = formatResourceHeader(line)
					hasEmitted = true
				}
			case bc > 0:
				// Block opener (e.g. `resource "..." {`); suppress blank before block.
				pendingBlank = false
				depth = bc
				bodyLines = []string{line} // include opener in body
				state = stateInBlock
			default:
				// First non-header line after the resource list; save pending block
				// and emit the line as footer.
				saveBlock()
				addFooter(line)
				state = stateNormal
			}

		case stateInBlock:
			bodyLines = append(bodyLines, line)
			depth += countBraces(plain)
			if depth <= 0 {
				depth = 0
				saveBlock()
				state = stateAfterHeader // next resource's header may follow
			}
		}
	}

	// Save any remaining pending block (e.g. last resource in piped format).
	saveBlock()

	return result
}

// filterOutput reads terraform plan/apply output from r and writes a collapsed
// version to w. It wraps parseBlocks and emits block summaries followed by
// any footer lines.
func filterOutput(r io.Reader, w io.Writer) {
	result := parseBlocks(r)
	for _, b := range result.Blocks {
		fmt.Fprintln(w, b.Summary)
	}
	for _, line := range result.Footer {
		fmt.Fprintln(w, line)
	}
}

// targetOutput reads terraform plan output from r, extracts resource addresses
// from "  # <address> <action>" header lines, and writes them to w as
// -target= flags suitable for pasting into a shell command. Each line ends
// with " \" except the last.
func targetOutput(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	var targets []string
	for scanner.Scan() {
		plain := stripANSI(scanner.Text())
		if strings.HasPrefix(plain, "  # ") && !strings.HasPrefix(plain, "  # (") {
			trimmed := plain[4:]
			if idx := strings.Index(trimmed, " "); idx > 0 {
				targets = append(targets, trimmed[:idx])
			}
		}
	}

	for i, t := range targets {
		if i < len(targets)-1 {
			fmt.Fprintf(w, "-target=%s \\\n", t)
		} else {
			fmt.Fprintf(w, "-target=%s\n", t)
		}
	}
}

// --- Interactive viewer ---

// ANSI escape sequences used by the interactive viewer.
const (
	viewerClearScreen   = "\x1b[2J"
	viewerCursorHome    = "\x1b[H"
	viewerCursorHide    = "\x1b[?25l"
	viewerCursorShow    = "\x1b[?25h"
	viewerAltEnter      = "\x1b[?1049h" // switch to alternate screen buffer
	viewerAltLeave      = "\x1b[?1049l" // switch back to main screen buffer
	viewerInvert        = "\x1b[7m"
	viewerBoldReset     = "\x1b[0m"
)

// Viewer holds the interactive diff viewer state.
type Viewer struct {
	blocks   []ResourceBlock
	cursor   int
	expanded []bool
}

func newViewer(blocks []ResourceBlock) *Viewer {
	return &Viewer{
		blocks:   blocks,
		cursor:   0,
		expanded: make([]bool, len(blocks)),
	}
}

// navigate moves the cursor by delta, clamped to valid range.
func (v *Viewer) navigate(delta int) {
	if len(v.blocks) == 0 {
		return
	}
	v.cursor += delta
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= len(v.blocks) {
		v.cursor = len(v.blocks) - 1
	}
}

// toggle flips the expanded state for the block at cursor.
func (v *Viewer) toggle() {
	if len(v.blocks) == 0 {
		return
	}
	v.expanded[v.cursor] = !v.expanded[v.cursor]
}

// render writes the full viewer UI to w. Uses \r\n for correct rendering in
// raw terminal mode.
func render(v *Viewer, w io.Writer) {
	fmt.Fprint(w, viewerClearScreen+viewerCursorHome)
	fmt.Fprintf(w, "\x1b[1m tf interactive diff — %d resource(s) \x1b[0m\r\n", len(v.blocks))
	fmt.Fprint(w, "  j/k or \u2191\u2193 to navigate  Enter/Space to expand  q to quit\r\n\r\n")

	for i, b := range v.blocks {
		if i == v.cursor {
			fmt.Fprintf(w, "%s\u25b6 %s%s\r\n", viewerInvert, b.Summary, viewerBoldReset)
		} else {
			fmt.Fprintf(w, "  %s%s\r\n", b.Summary, viewerBoldReset)
		}
		if v.expanded[i] {
			if len(b.Body) == 0 {
				fmt.Fprint(w, "    (no diff body captured)\r\n")
			} else {
				for _, line := range b.Body {
					fmt.Fprintf(w, "    %s\r\n", line)
				}
			}
		}
	}
}

// runViewer launches the interactive terminal viewer. It uses the alternate
// screen buffer so the original terminal content is restored on exit.
// After exiting, collapsed summaries are printed to stdout for scrollback.
func runViewer(blocks []ResourceBlock) error {
	// Open /dev/tty explicitly — os.Stdin may be connected to the terraform pipe.
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return fmt.Errorf("open /dev/tty: %w", err)
	}
	defer tty.Close()

	oldState, err := term.MakeRaw(int(tty.Fd()))
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}

	v := newViewer(blocks)
	fmt.Fprint(os.Stdout, viewerAltEnter+viewerCursorHide)
	render(v, os.Stdout)

	buf := make([]byte, 4)
	quit := false
	for !quit {
		n, err := tty.Read(buf)
		if err != nil || n == 0 {
			break
		}
		switch {
		case buf[0] == 'q' || buf[0] == 3: // q or Ctrl-C
			quit = true
		case buf[0] == 'j' || (n >= 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'B'): // j or ↓
			v.navigate(+1)
		case buf[0] == 'k' || (n >= 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'A'): // k or ↑
			v.navigate(-1)
		case buf[0] == 13 || buf[0] == 32: // Enter or Space
			v.toggle()
		}
		if !quit {
			render(v, os.Stdout)
		}
	}

	// Restore terminal before printing final output.
	term.Restore(int(tty.Fd()), oldState) //nolint:errcheck
	fmt.Fprint(os.Stdout, viewerCursorShow+viewerAltLeave)

	// Print collapsed summaries to the main screen for terminal scrollback.
	for _, b := range v.blocks {
		fmt.Fprintln(os.Stdout, b.Summary)
	}
	return nil
}

// isPlanApply reports whether the first argument is "plan" or "apply".
func isPlanApply(args []string) bool {
	return len(args) > 0 && (args[0] == "plan" || args[0] == "apply")
}

func main() {
	args := os.Args[1:]

	if len(args) == 1 && args[0] == "ver" {
		fmt.Printf("tf %s\n", getVersion())
		os.Exit(0)
	}

	if len(args) >= 1 && args[0] == "target" {
		planArgs := append([]string{"plan"}, args[1:]...)
		cmd := exec.Command("terraform", planArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "tf: %v\n", err)
			os.Exit(1)
		}
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "tf: %v\n", err)
			os.Exit(1)
		}

		targetOutput(stdout, os.Stdout)

		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Fprintf(os.Stderr, "tf: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	cmd := exec.Command("terraform", args...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tf: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "tf: %v\n", err)
		os.Exit(1)
	}

	result := parseBlocks(stdout)

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "tf: %v\n", err)
		os.Exit(1)
	}

	// Launch interactive viewer for plan/apply when stdout is a TTY and there
	// are resource blocks to display; otherwise fall back to plain output.
	if isPlanApply(args) && term.IsTerminal(int(os.Stdout.Fd())) && len(result.Blocks) > 0 {
		if err := runViewer(result.Blocks); err != nil {
			fmt.Fprintf(os.Stderr, "tf: viewer: %v\n", err)
		}
	} else {
		for _, b := range result.Blocks {
			fmt.Fprintln(os.Stdout, b.Summary)
		}
		for _, line := range result.Footer {
			fmt.Fprintln(os.Stdout, line)
		}
	}
}
