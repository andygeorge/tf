package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFilterOutput(t *testing.T) {
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
	t.Log(got)

	if !strings.Contains(got, "  # module.api.module.ami.aws_ebs_volume.arm64-ubuntu-22_04 must be replaced") {
		t.Error("header line missing")
	}
	if strings.Contains(got, `resource "aws_ebs_volume"`) {
		t.Error("block opener should be suppressed")
	}
	if strings.Contains(got, "arn =") {
		t.Error("block contents should be suppressed")
	}
	if !strings.Contains(got, "Plan: 1 to add") {
		t.Error("trailing line missing")
	}
}
