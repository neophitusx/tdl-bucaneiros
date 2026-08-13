package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestUploadPathPreservesCommas(t *testing.T) {
	var got []string

	cmd := &cobra.Command{Run: func(_ *cobra.Command, _ []string) {}}
	cmd.Flags().StringArrayVarP(&got, "path", "p", []string{}, "dirs or files")
	cmd.SetArgs([]string{"--path", `C:\videos\episode, part 1.mkv`, "-p", `C:\videos\episode, part 2.mkv`})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	want := []string{`C:\videos\episode, part 1.mkv`, `C:\videos\episode, part 2.mkv`}
	if len(got) != len(want) {
		t.Fatalf("path count = %d, want %d (%q)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("path[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
