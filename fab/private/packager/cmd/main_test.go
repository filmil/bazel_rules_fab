// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runWith(t *testing.T, args ...string) error {
	t.Helper()
	flag.CommandLine = flag.NewFlagSet("packager", flag.ContinueOnError)
	os.Args = append([]string{"packager"}, args...)
	return run()
}

func TestRun(t *testing.T) {
	d := t.TempDir()
	gtl := filepath.Join(d, "main-F_Cu.gtl")
	csv := filepath.Join(d, "x.csv")
	for _, f := range []string{gtl, csv} {
		if err := os.WriteFile(f, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(d, "out.zip")
	if err := runWith(t, "--vendor", "jlcpcb", "--board", "main", "--fab", gtl, "--out", out); err != nil {
		t.Fatalf("a bare board: %v", err)
	}
	if b, err := os.ReadFile(out); err != nil || len(b) == 0 {
		t.Errorf("no ZIP: %v", err)
	}
	base := []string{"--vendor", "jlcpcb", "--board", "main", "--fab", gtl, "--out", out}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--bom", csv}, "--bom goes with --pos"},
		{[]string{"--pos", csv}, "--bom goes with --pos"},
		{[]string{"--rotation", ".*=90"}, "a bare board has none"},
	} {
		if err := runWith(t, append(append([]string{}, base...), c.args...)...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: got %v, want %q", c.args, err, c.want)
		}
	}
	if err := runWith(t, "--vendor", "jlcpcb", "--board", "main", "--out", out); err == nil || !strings.Contains(err.Error(), "--board, --fab and --out are needed") {
		t.Errorf("no --fab: %v", err)
	}
}
