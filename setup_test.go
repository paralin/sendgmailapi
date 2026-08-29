package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAskYesNo(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		{name: "default yes", input: "\n", defaultYes: true, want: true},
		{name: "default no", input: "\n", defaultYes: false, want: false},
		{name: "yes", input: "yes\n", want: true},
		{name: "no", input: "no\n", defaultYes: true, want: false},
		{name: "retry invalid", input: "maybe\nyes\n", want: true},
		{name: "EOF is not consent", input: "", defaultYes: true, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(test.input))
			if got := askYesNo(reader, "Continue?", test.defaultYes); got != test.want {
				t.Fatalf("askYesNo() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestWritePrivateFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "token.json")
	if err := writePrivateFile(path, []byte("token")); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "token" {
		t.Fatalf("unexpected contents: %q", contents)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if permissions := info.Mode().Perm(); permissions != 0600 {
		t.Fatalf("permissions = %o, want 600", permissions)
	}
}
