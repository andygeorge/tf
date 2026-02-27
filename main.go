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

	filterOutput(stdout, os.Stdout)

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "tf: %v\n", err)
		os.Exit(1)
	}
}
