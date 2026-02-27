package main

import (
	"bytes"
	"fmt"
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

// --- parseBlocks tests ---

func TestParseBlocks_AddOperation(t *testing.T) {
	input := `Terraform will perform the following actions:

  # module.api.aws_instance.web will be created
+ resource "aws_instance" "web" {
      ami = "ami-12345"
    }

Plan: 1 to add, 0 to change, 0 to destroy.`

	result := parseBlocks(strings.NewReader(input))

	if len(result.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result.Blocks))
	}
	b := result.Blocks[0]
	if !strings.Contains(b.Summary, "module.api.aws_instance.web will be created") {
		t.Errorf("summary missing resource address: %q", b.Summary)
	}
	if !strings.HasPrefix(b.Summary, "\x1b[1m") {
		t.Error("summary should start with bold ANSI code")
	}
	if len(b.Body) == 0 {
		t.Error("body should not be empty for add operation")
	}
	bodyText := strings.Join(b.Body, "\n")
	if !strings.Contains(bodyText, `resource "aws_instance"`) {
		t.Error("body should contain block opener")
	}
}

func TestParseBlocks_DestroyOperation(t *testing.T) {
	input := `Terraform will perform the following actions:

  # module.api.aws_instance.web will be destroyed
- resource "aws_instance" "web" {
      id = "i-12345"
    }

Plan: 0 to add, 0 to change, 1 to destroy.`

	result := parseBlocks(strings.NewReader(input))

	if len(result.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result.Blocks))
	}
	if !strings.Contains(result.Blocks[0].Summary, "will be destroyed") {
		t.Error("summary should indicate destroy operation")
	}
	if len(result.Blocks[0].Body) == 0 {
		t.Error("body should not be empty for destroy operation")
	}
}

func TestParseBlocks_ChangeOperation(t *testing.T) {
	input := `Terraform will perform the following actions:

  # module.api.aws_instance.web will be updated in-place
~ resource "aws_instance" "web" {
      ~ instance_type = "t2.micro" -> "t3.micro"
        id            = "i-12345"
    }

Plan: 0 to add, 1 to change, 0 to destroy.`

	result := parseBlocks(strings.NewReader(input))

	if len(result.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result.Blocks))
	}
	if !strings.Contains(result.Blocks[0].Summary, "will be updated in-place") {
		t.Error("summary should indicate update operation")
	}
	bodyText := strings.Join(result.Blocks[0].Body, "\n")
	if !strings.Contains(bodyText, "instance_type") {
		t.Error("body should contain changed attribute")
	}
}

func TestParseBlocks_ReplaceOperation(t *testing.T) {
	input := `Terraform will perform the following actions:

  # module.api.aws_ebs_volume.data must be replaced
-/+ resource "aws_ebs_volume" "data" {
      ~ size = 10 -> 20 # forces replacement
        type = "gp2"
    }

Plan: 1 to add, 0 to change, 1 to destroy.`

	result := parseBlocks(strings.NewReader(input))

	if len(result.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result.Blocks))
	}
	if !strings.Contains(result.Blocks[0].Summary, "must be replaced") {
		t.Error("summary should indicate replace operation")
	}
}

func TestParseBlocks_BodyCaptured(t *testing.T) {
	input := `Terraform will perform the following actions:

  # module.api.aws_instance.web must be replaced
-/+ resource "aws_instance" "web" {
      ~ ami  = "old-ami" -> "new-ami" # forces replacement
        id   = "i-12345"
        tags = {
            "Name" = "web"
        }
    }

Plan: 1 to add, 0 to change, 1 to destroy.`

	result := parseBlocks(strings.NewReader(input))

	if len(result.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result.Blocks))
	}
	body := result.Blocks[0].Body
	if len(body) == 0 {
		t.Fatal("body should not be empty")
	}
	bodyText := strings.Join(body, "\n")
	if !strings.Contains(bodyText, "ami") {
		t.Error("body should contain changed attribute")
	}
	if !strings.Contains(bodyText, "tags") {
		t.Error("body should contain nested block")
	}
}

func TestParseBlocks_EmptyPlan(t *testing.T) {
	input := `No changes. Your infrastructure matches the configuration.`

	result := parseBlocks(strings.NewReader(input))

	if len(result.Blocks) != 0 {
		t.Errorf("expected 0 blocks for empty plan, got %d", len(result.Blocks))
	}
}

func TestParseBlocks_MultipleBlocks(t *testing.T) {
	input := `Terraform will perform the following actions:

  # module.api.aws_instance.web will be created
+ resource "aws_instance" "web" {
      ami = "ami-12345"
    }

  # module.api.aws_s3_bucket.data will be destroyed
- resource "aws_s3_bucket" "data" {
      id = "my-bucket"
    }

Plan: 1 to add, 0 to change, 1 to destroy.`

	result := parseBlocks(strings.NewReader(input))

	if len(result.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result.Blocks))
	}
	if !strings.Contains(result.Blocks[0].Summary, "will be created") {
		t.Error("first block should be the create")
	}
	if !strings.Contains(result.Blocks[1].Summary, "will be destroyed") {
		t.Error("second block should be the destroy")
	}
}

func TestParseBlocks_PipedFormat(t *testing.T) {
	// Piped format: headers only, no block bodies.
	input := `Terraform will perform the following actions:

  # module.api.aws_autoscaling_group.myapp will be destroyed

  # module.api.aws_cloudwatch_log_group.logs will be destroyed

Plan: 0 to add, 0 to change, 2 to destroy.`

	result := parseBlocks(strings.NewReader(input))

	if len(result.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result.Blocks))
	}
	if result.Blocks[0].Body != nil {
		t.Error("piped format block should have nil body")
	}
	if result.Blocks[1].Body != nil {
		t.Error("piped format block should have nil body")
	}
	if !strings.Contains(result.Blocks[0].Summary, "aws_autoscaling_group.myapp") {
		t.Error("first block summary missing")
	}
	if !strings.Contains(result.Blocks[1].Summary, "aws_cloudwatch_log_group.logs") {
		t.Error("second block summary missing")
	}
}

// --- Viewer state transition tests ---

func TestViewer_Navigate(t *testing.T) {
	blocks := []ResourceBlock{
		{Summary: "a"},
		{Summary: "b"},
		{Summary: "c"},
	}
	v := newViewer(blocks)

	if v.cursor != 0 {
		t.Errorf("initial cursor should be 0, got %d", v.cursor)
	}

	v.navigate(+1)
	if v.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", v.cursor)
	}

	v.navigate(+1)
	if v.cursor != 2 {
		t.Errorf("expected cursor 2, got %d", v.cursor)
	}

	// Clamp at end.
	v.navigate(+10)
	if v.cursor != 2 {
		t.Errorf("cursor should clamp at 2, got %d", v.cursor)
	}

	v.navigate(-1)
	if v.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", v.cursor)
	}

	// Clamp at start.
	v.navigate(-100)
	if v.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", v.cursor)
	}
}

func TestViewer_Toggle(t *testing.T) {
	blocks := []ResourceBlock{{Summary: "a"}}
	v := newViewer(blocks)

	if v.expanded[0] {
		t.Error("block should start collapsed")
	}

	v.toggle()
	if !v.expanded[0] {
		t.Error("block should be expanded after toggle")
	}

	v.toggle()
	if v.expanded[0] {
		t.Error("block should be collapsed after second toggle")
	}
}

func TestViewer_EmptyBlocks(t *testing.T) {
	v := newViewer(nil)
	// Should not panic.
	v.navigate(+1)
	v.navigate(-1)
	v.toggle()
	if v.cursor != 0 {
		t.Errorf("cursor should remain 0 for empty viewer, got %d", v.cursor)
	}
}

func TestViewer_ToggleAtCursor(t *testing.T) {
	blocks := []ResourceBlock{
		{Summary: "a"},
		{Summary: "b"},
		{Summary: "c"},
	}
	v := newViewer(blocks)
	v.navigate(+1) // cursor at 1
	v.toggle()

	if v.expanded[0] {
		t.Error("block 0 should not be expanded")
	}
	if !v.expanded[1] {
		t.Error("block 1 (at cursor) should be expanded")
	}
	if v.expanded[2] {
		t.Error("block 2 should not be expanded")
	}
}

// --- Viewer render tests ---

func TestRender_Header(t *testing.T) {
	blocks := []ResourceBlock{{Summary: "module.api.aws_instance.web will be created"}}
	v := newViewer(blocks)
	var buf bytes.Buffer
	render(v, &buf, 0)
	got := buf.String()

	if !strings.Contains(got, "tf interactive diff") {
		t.Error("render should contain viewer header")
	}
	if !strings.Contains(got, "1 resource") {
		t.Error("render should show resource count")
	}
}

func TestRender_CursorHighlight(t *testing.T) {
	blocks := []ResourceBlock{
		{Summary: "res.a"},
		{Summary: "res.b"},
	}
	v := newViewer(blocks) // cursor at 0
	var buf bytes.Buffer
	render(v, &buf, 0)
	got := buf.String()

	// The cursor indicator should appear before the first block.
	if !strings.Contains(got, viewerInvert+"▶ ") {
		t.Error("cursor indicator (invert+arrow) should appear for selected block")
	}
	// The second block should not be highlighted.
	lines := strings.Split(got, "\n")
	for _, l := range lines {
		if strings.Contains(l, "res.b") && strings.Contains(l, viewerInvert) {
			t.Error("non-selected block should not have cursor highlight")
		}
	}
}

func TestRender_ExpandedBlock(t *testing.T) {
	blocks := []ResourceBlock{
		{
			Summary: "module.api.aws_instance.web must be replaced",
			Body:    []string{`+ resource "aws_instance" "web" {`, "    ami = \"new\"", "  }"},
		},
	}
	v := newViewer(blocks)
	v.toggle() // expand block 0

	var buf bytes.Buffer
	render(v, &buf, 0)
	got := buf.String()

	if !strings.Contains(got, `resource "aws_instance"`) {
		t.Error("expanded block should show body content")
	}
}

func TestRender_CollapsedBlock(t *testing.T) {
	blocks := []ResourceBlock{
		{
			Summary: "module.api.aws_instance.web must be replaced",
			Body:    []string{`+ resource "aws_instance" "web" {`, "    ami = \"new\"", "  }"},
		},
	}
	v := newViewer(blocks)
	// Block is collapsed by default.

	var buf bytes.Buffer
	render(v, &buf, 0)
	got := buf.String()

	if strings.Contains(got, `resource "aws_instance"`) {
		t.Error("collapsed block should not show body content")
	}
}

func TestRender_EmptyBody(t *testing.T) {
	blocks := []ResourceBlock{
		{Summary: "module.api.aws_instance.web will be destroyed", Body: nil},
	}
	v := newViewer(blocks)
	v.toggle() // expand

	var buf bytes.Buffer
	render(v, &buf, 0)
	got := buf.String()

	if !strings.Contains(got, "no diff body captured") {
		t.Error("nil body should show 'no diff body captured' message")
	}
}

// --- Viewport / scrolling tests ---

func TestViewer_ScrollToCursor(t *testing.T) {
	blocks := make([]ResourceBlock, 10)
	for i := range blocks {
		blocks[i] = ResourceBlock{Summary: fmt.Sprintf("resource.%d", i)}
	}
	v := newViewer(blocks)

	// Move cursor to block 7 with viewport of 5.
	v.cursor = 7
	v.scrollToCursor(5)
	// Offset should be at most cursor - viewportSize + 1 = 3.
	if v.offset > 3 {
		t.Errorf("offset should be ≤ 3 to keep cursor in view, got %d", v.offset)
	}

	// Move cursor to block 0 — offset should scroll back to 0.
	v.cursor = 0
	v.scrollToCursor(5)
	if v.offset != 0 {
		t.Errorf("offset should be 0 when cursor at top, got %d", v.offset)
	}
}

func TestViewer_ScrollIndicators(t *testing.T) {
	blocks := make([]ResourceBlock, 10)
	for i := range blocks {
		blocks[i] = ResourceBlock{Summary: fmt.Sprintf("module.api.resource.res%d will be created", i)}
	}
	v := newViewer(blocks)
	v.offset = 3 // simulate having scrolled down

	var buf bytes.Buffer
	render(v, &buf, 5) // viewport of 5 blocks
	got := buf.String()

	// Should show "more above" since offset > 0.
	if !strings.Contains(got, "more above") {
		t.Error("should show scroll indicator for hidden blocks above")
	}
	// Should show "more below" since 3+5=8 < 10.
	if !strings.Contains(got, "more below") {
		t.Error("should show scroll indicator for hidden blocks below")
	}
	// Should not show blocks 0-2 (above offset).
	if strings.Contains(got, "res0") || strings.Contains(got, "res1") || strings.Contains(got, "res2") {
		t.Error("blocks above offset should not be rendered")
	}
}
