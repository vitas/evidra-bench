package main

import "testing"

func TestMainHelp(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}
}
