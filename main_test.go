package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFilterOutput_BlockCollapse(t *testing.T) {
	input := `Terraform will perform the following actions:

  # module.api.module.ami.aws_ebs_volume.arm64-ubuntu-22_04 must be replaced
-/+ resource "aws_ebs_volume" "arm64-ubuntu-22_04" {
      ~ arn                  = "arn:aws:ec2:us-east-2:123456789012:volume/vol-00000000000000000" -> (known after apply)
      ~ create_time          = "2025-11-13T20:45:01Z" -> (known after apply)
        tags                 = {
            "Name" = "arm64-ubuntu-22.04"
        }
      ~ type                 = "gp2" -> (known after apply)
        # (4 unchanged attributes hidden)
    }

Plan: 1 to add, 0 to change, 1 to destroy.`

	var out bytes.Buffer
	filterOutput(strings.NewReader(input), &out)
	got := out.String()

	if !strings.Contains(got, "\x1b[1mmodule.api.module.ami.aws_ebs_volume.arm64-ubuntu-22_04 must be replaced") {
		t.Error("header line missing")
	}
	if strings.Contains(got, `resource "aws_ebs_volume"`) {
		t.Error("block opener should be suppressed")
	}
	if strings.Contains(got, "arn =") {
		t.Error("block contents should be suppressed")
	}
	if strings.Contains(got, "Plan:") {
		t.Error("plan summary should be suppressed")
	}
}

func TestFilterOutput_Preamble(t *testing.T) {
	input := `module.api.random_password.my-db: Refreshing state... [id=none]
module.api.data.aws_kms_key.api: Reading...
module.api.data.aws_kms_key.api: Read complete after 0s [id=mrk-abc]
module.api.data.aws_kms_key.api: Still reading... [id=mrk-abc, 10s elapsed]

Terraform will perform the following actions:

Plan: 0 to add, 0 to change, 0 to destroy.`

	var out bytes.Buffer
	filterOutput(strings.NewReader(input), &out)
	got := out.String()

	if strings.Contains(got, "Refreshing state") {
		t.Error("Refreshing state lines should be suppressed")
	}
	if strings.Contains(got, ": Reading...") {
		t.Error("Reading lines should be suppressed")
	}
	if strings.Contains(got, "Read complete after") {
		t.Error("Read complete lines should be suppressed")
	}
	if strings.Contains(got, "Still reading") {
		t.Error("Still reading lines should be suppressed")
	}
	if strings.Contains(got, "Terraform will perform") {
		t.Error("plan header should be suppressed")
	}
	if strings.Contains(got, "Plan:") {
		t.Error("plan summary should be suppressed")
	}
}

func TestFilterOutput_SecondaryComment(t *testing.T) {
	input := `Terraform will perform the following actions:

  # module.api.aws_autoscaling_group.myapp will be destroyed
  # (because aws_autoscaling_group.myapp is not in configuration)
  resource "aws_autoscaling_group" "myapp" {
      id = "myapp"
    }

Plan: 0 to add, 0 to change, 1 to destroy.`

	var out bytes.Buffer
	filterOutput(strings.NewReader(input), &out)
	got := out.String()

	if !strings.Contains(got, "\x1b[1mmodule.api.aws_autoscaling_group.myapp will be destroyed") {
		t.Error("primary header should be preserved")
	}
	if strings.Contains(got, "(because") {
		t.Error("secondary parenthetical comment should be suppressed")
	}
	if strings.Contains(got, `resource "aws_autoscaling_group"`) {
		t.Error("block should be suppressed")
	}
	if strings.Contains(got, "Plan:") {
		t.Error("plan summary should be suppressed")
	}
}

// TestFilterOutput_PipedFormat covers the compact format terraform uses when
// its stdout is piped: one "# resource" header per resource with a blank line
// between each, no full block bodies, followed by a separator and Note advisory.
func TestFilterOutput_PipedFormat(t *testing.T) {
	sep := strings.Repeat("─", 40)
	input := "Terraform will perform the following actions:\n" +
		"\n" +
		"  # module.api.aws_autoscaling_group.myapp will be destroyed\n" +
		"\n" +
		"  # module.api.aws_cloudwatch_log_group.myapp-cache will be destroyed\n" +
		"\n" +
		"  # module.api.aws_launch_template.api will be updated in-place\n" +
		"\n" +
		"Plan: 0 to add, 1 to change, 2 to destroy.\n" +
		"\x1b[0m\x1b[90m\n" + // ANSI-only line before separator
		sep + "\n" +
		"\n" +
		"Note: You didn't use the -out option to save this plan, so Terraform can't\n" +
		"guarantee to take exactly these actions if you run \"terraform apply\" now.\n" +
		"\n"

	var out bytes.Buffer
	filterOutput(strings.NewReader(input), &out)
	got := out.String()

	// All three headers must appear.
	for _, hdr := range []string{
		"\x1b[1mmodule.api.aws_autoscaling_group.myapp will be destroyed",
		"\x1b[1mmodule.api.aws_cloudwatch_log_group.myapp-cache will be destroyed",
		"\x1b[1mmodule.api.aws_launch_template.api will be updated in-place",
	} {
		if !strings.Contains(got, hdr) {
			t.Errorf("header missing: %s", hdr)
		}
	}

	// Headers must be consecutive (no blank lines between them).
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	inHeaders := false
	for _, l := range lines {
		if strings.HasPrefix(stripANSI(l), "module.") {
			inHeaders = true
			continue
		}
		if inHeaders {
			if l == "" {
				// Blank line is OK only after ALL headers are done.
				inHeaders = false
			} else {
				t.Errorf("unexpected line between/after headers: %q", l)
			}
			break
		}
	}

	if strings.Contains(got, "Plan:") {
		t.Error("plan summary should be suppressed")
	}
	if strings.Contains(got, "─") {
		t.Error("separator line should be suppressed")
	}
	if strings.Contains(got, "Note:") {
		t.Error("Note advisory should be suppressed")
	}
	if strings.Contains(got, "guarantee to") {
		t.Error("Note continuation should be suppressed")
	}
}

func TestTargetOutput_Basic(t *testing.T) {
	input := `Terraform will perform the following actions:

  # module.api.aws_autoscaling_group.myapp will be destroyed
  # (because aws_autoscaling_group.myapp is not in configuration)
  # module.api.aws_cloudwatch_log_group.myapp-cache will be destroyed
  # module.api.aws_launch_template.api will be updated in-place

Plan: 0 to add, 1 to change, 2 to destroy.`

	var out bytes.Buffer
	targetOutput(strings.NewReader(input), &out)
	got := out.String()

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	if lines[0] != `-target=module.api.aws_autoscaling_group.myapp \` {
		t.Errorf("line 0: %q", lines[0])
	}
	if lines[1] != `-target=module.api.aws_cloudwatch_log_group.myapp-cache \` {
		t.Errorf("line 1: %q", lines[1])
	}
	if lines[2] != `-target=module.api.aws_launch_template.api` {
		t.Errorf("line 2 (last, no backslash): %q", lines[2])
	}
}

func TestTargetOutput_SingleResource(t *testing.T) {
	input := `  # module.api.aws_instance.web must be replaced
Plan: 1 to add, 0 to change, 1 to destroy.`

	var out bytes.Buffer
	targetOutput(strings.NewReader(input), &out)
	got := strings.TrimRight(out.String(), "\n")

	if got != `-target=module.api.aws_instance.web` {
		t.Errorf("single resource should have no backslash: %q", got)
	}
}

func TestTargetOutput_NoResources(t *testing.T) {
	input := `No changes. Your infrastructure matches the configuration.`

	var out bytes.Buffer
	targetOutput(strings.NewReader(input), &out)
	if out.Len() != 0 {
		t.Errorf("expected empty output, got %q", out.String())
	}
}

func TestTargetOutput_ANSICodes(t *testing.T) {
	// Headers with ANSI codes should still be extracted correctly.
	input := "\x1b[1m  # module.api.aws_autoscaling_group.myapp\x1b[0m will be \x1b[31mdestroyed\x1b[0m\n" +
		"\x1b[1m  # module.api.aws_instance.web\x1b[0m must be \x1b[31mreplaced\x1b[0m\n"

	var out bytes.Buffer
	targetOutput(strings.NewReader(input), &out)
	got := out.String()

	if !strings.Contains(got, "-target=module.api.aws_autoscaling_group.myapp") {
		t.Error("first target missing")
	}
	if !strings.Contains(got, "-target=module.api.aws_instance.web") {
		t.Error("second target missing")
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if strings.HasSuffix(lines[len(lines)-1], `\`) {
		t.Error("last line should not end with backslash")
	}
}

func TestFilterOutput_ANSICodes(t *testing.T) {
	// Preamble line with ANSI codes should be suppressed.
	preambleLine := "\x1b[0m\x1b[1mmodule.api.random_password: Refreshing state... [id=none]\x1b[0m"
	// Input header line as terraform emits it (with "  # " prefix, bold wrapping all).
	inputHeader := "\x1b[1m  # module.api.aws_autoscaling_group.myapp\x1b[0m will be \x1b[1m\x1b[31mdestroyed\x1b[0m"
	// Expected output: "  # " stripped, \x1b[1m prepended for bold resource address.
	wantHeader := "\x1b[1mmodule.api.aws_autoscaling_group.myapp\x1b[0m will be \x1b[1m\x1b[31mdestroyed\x1b[0m"
	// Block opener with ANSI codes should be suppressed (after header).
	blockLine := "\x1b[0m  \x1b[31m-\x1b[0m resource \"aws_autoscaling_group\" \"myapp\" {\n      id = \"myapp\"\n    }"

	input := preambleLine + "\n" + inputHeader + "\n" + blockLine + "\nPlan: 0 to add, 0 to change, 1 to destroy."

	var out bytes.Buffer
	filterOutput(strings.NewReader(input), &out)
	got := out.String()

	if strings.Contains(got, "Refreshing state") {
		t.Error("ANSI-coded preamble line should be suppressed")
	}
	if !strings.Contains(got, wantHeader) {
		t.Error("ANSI-coded header line should be emitted without '  # ' prefix")
	}
	if strings.Contains(got, `resource "aws_autoscaling_group"`) {
		t.Error("ANSI-coded block should be suppressed")
	}
}

func TestFilterOutput_MultipleResources(t *testing.T) {
	input := `Terraform will perform the following actions:

  # module.api.resource1 will be destroyed
  # (because resource1 is not in configuration)
  resource "type1" "resource1" {
      id = "r1"
    }

  # module.api.resource2 must be replaced
-/+ resource "type2" "resource2" {
      ~ id = "old" -> (known after apply)
    }

Plan: 1 to add, 0 to change, 2 to destroy.`

	var out bytes.Buffer
	filterOutput(strings.NewReader(input), &out)
	got := out.String()

	if !strings.Contains(got, "\x1b[1mmodule.api.resource1 will be destroyed") {
		t.Error("resource1 header missing")
	}
	if !strings.Contains(got, "\x1b[1mmodule.api.resource2 must be replaced") {
		t.Error("resource2 header missing")
	}
	if strings.Contains(got, `resource "type1"`) || strings.Contains(got, `resource "type2"`) {
		t.Error("resource blocks should be suppressed")
	}
	if strings.Contains(got, "Plan:") {
		t.Error("plan summary should be suppressed")
	}
}
