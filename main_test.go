package main

import "testing"

func TestIsInstanceArm(t *testing.T) {
	result := isInstanceArm("t4g.micro")
	if result != true {
		t.Error("t4g.micro should be detected as ARM")
	}
	result = isInstanceArm("m5.2xlarge")
	if result != false {
		t.Error("m5.2xlarge should not be detected as ARM")
	}
	result = isInstanceArm("r6gd.16xlarge")
	if result != true {
		t.Error("r6gd.16xlarge should be detected as ARM")
	}
}
