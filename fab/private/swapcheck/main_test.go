// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) string {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := write("a.pdf", Marker+"\n[\"x\"]\n")
	write("tree/stub_record", Marker+"\n[]\n")
	host := write("tree/F_Cu.gbr", "%TF.GenerationSoftware,KiCad*%\n")
	near := write("b.txt", Marker+"X\n")

	bad, n, err := Check([]string{good, filepath.Join(dir, "tree"), near})
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 || !reflect.DeepEqual(bad, []string{host, near}) {
		t.Errorf("got bad=%v n=%d", bad, n)
	}
}
