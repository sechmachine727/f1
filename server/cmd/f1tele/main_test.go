package main

import (
	"strings"
	"testing"
)

func TestReadNewPassword(t *testing.T) {
	got, err := readNewPassword(true, strings.NewReader("new-password-123\n"))
	if err != nil || got != "new-password-123" {
		t.Fatalf("stdin password %q err %v", got, err)
	}
	got, err = readNewPassword(false, strings.NewReader("one-two-three\none-two-three\n"))
	if err != nil || got != "one-two-three" {
		t.Fatalf("interactive password %q err %v", got, err)
	}
	if _, err := readNewPassword(false, strings.NewReader("one-two-three\nother-password\n")); err == nil {
		t.Fatal("mismatched confirmation accepted")
	}
	if _, err := readNewPassword(true, strings.NewReader("short\n")); err == nil {
		t.Fatal("short password accepted")
	}
}
