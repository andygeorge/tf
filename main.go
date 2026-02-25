package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

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

// filterOutput reads terraform plan/apply output from r and writes a collapsed
// version to w. It suppresses:
//   - state-refresh preamble lines (Refreshing state, Reading, etc.)
//   - invisible ANSI-only lines
//   - box-drawing separator and the "Note: You didn't use -out" advisory
//   - resource block bodies (keeping only the "  # resource" header line)
//   - blank lines between consecutive resource headers
//
// A single blank line is preserved as a separator before the "Plan:" summary.
func filterOutput(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	var state filterState
	depth := 0
	inNote := false        // suppress multi-line "Note:" advisory
	inIntroBlock := false  // suppress "Terraform used the selected providers" block
	pendingBlank := false  // defer blank lines so inter-header blanks can be dropped
	hasEmitted := false    // suppress leading blank before first real content

	emit := func(line string) {
		if pendingBlank && hasEmitted {
			fmt.Fprintln(w, "")
		}
		pendingBlank = false
		fmt.Fprintln(w, line)
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

		// Blank lines: record as pending rather than emitting immediately, so we
		// can drop blanks that turn out to be between two resource headers.
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
					emit(formatResourceHeader(line))
				}
			} else {
				emit(line)
			}

		case stateAfterHeader:
			bc := countBraces(plain)
			switch {
			case strings.HasPrefix(plain, "  # ("):
				// Secondary parenthetical comment (e.g. "# (because ...)"); suppress.
				pendingBlank = false
			case strings.HasPrefix(plain, "  # "):
				// Another primary resource header; drop inter-header blank, print it.
				pendingBlank = false
				trimmed := plain[4:]
				if strings.Contains(trimmed, "module") {
					emit(formatResourceHeader(line))
				}
				// stay in stateAfterHeader
			case bc > 0:
				// Block opener (e.g. `resource "..." {`); suppress blank before block.
				pendingBlank = false
				depth = bc
				state = stateInBlock
			default:
				// First non-header line after the list (e.g. "Plan:"); re-emit the
				// pending blank as a visual separator, then print the line.
				emit(line)
				state = stateNormal
			}

		case stateInBlock:
			depth += countBraces(plain)
			if depth <= 0 {
				depth = 0
				state = stateAfterHeader // next resource's header may follow
			}
		}
	}
}

func main() {
	args := os.Args[1:]

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
