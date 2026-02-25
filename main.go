package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

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
	stateAfterHeader             // saw "  # ..." header, next line is block opener
	stateInBlock                 // inside a resource block, suppressing output
)

// filterOutput reads terraform plan/apply output from r, collapses resource
// change blocks down to their single-line header comment, and writes to w.
// All other output is passed through unchanged.
func filterOutput(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	state := stateNormal
	depth := 0

	for scanner.Scan() {
		line := scanner.Text()

		switch state {
		case stateNormal:
			// "  # <address> <action>" lines are resource-change headers.
			// Exactly 2 leading spaces so we don't match deeper indented comments.
			if strings.HasPrefix(line, "  # ") {
				fmt.Fprintln(w, line)
				state = stateAfterHeader
			} else {
				fmt.Fprintln(w, line)
			}

		case stateAfterHeader:
			bc := countBraces(line)
			if bc > 0 {
				// Block opener (e.g. `-/+ resource "..." {`): start suppressing.
				depth = bc
				state = stateInBlock
			} else {
				// Not a block opener; print and return to normal.
				fmt.Fprintln(w, line)
				state = stateNormal
			}

		case stateInBlock:
			depth += countBraces(line)
			if depth <= 0 {
				depth = 0
				state = stateNormal
			}
			// Suppress all lines inside the block.
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
