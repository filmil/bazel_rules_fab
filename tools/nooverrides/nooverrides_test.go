// SPDX-License-Identifier: Apache-2.0

package nooverrides

import (
	"os"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func read(t *testing.T, name string) string {
	t.Helper()
	p, err := runfiles.Rlocation("_main/tools/nooverrides/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCheck(t *testing.T) {
	for _, c := range []struct {
		file, allow, want string // want: "" for none, else a problem's text
	}{
		{"root_clean.txt", "", ""},
		{"root_archive.txt", "", `:1: archive_override of "hdlf_rules_kicad"`},
		{"root_local.txt", "", `:1: local_path_override of "y"`},
		{"integration_clean.txt", "hdlf_rules_fab", ""},
		{"integration_comment.txt", "hdlf_rules_fab", ""},
		{"integration_archive.txt", "hdlf_rules_fab", `:6: archive_override of "hdlf_rules_kicad"`},
		{"integration_git.txt", "hdlf_rules_fab", `:6: git_override of "z"`},
		{"integration_other.txt", "hdlf_rules_fab", `local_path_override of "other"`},
		{"integration_two.txt", "hdlf_rules_fab", `:6: local_path_override of "hdlf_rules_fab"`},
		{"integration_none.txt", "hdlf_rules_fab", `no local_path_override of "hdlf_rules_fab"`},
		// The integration module's override fails the root's check.
		{"integration_clean.txt", "", `local_path_override of "hdlf_rules_fab"`},
	} {
		got := strings.Join(Check(c.file, read(t, c.file), c.allow), "\n")
		switch {
		case c.want == "" && got != "":
			t.Errorf("%s (allow %q): want none, got:\n%s", c.file, c.allow, got)
		case c.want != "" && !strings.Contains(got, c.want):
			t.Errorf("%s (allow %q): want %q, got:\n%s", c.file, c.allow, c.want, got)
		}
	}
}

func TestFindSkipsCommentsAndNests(t *testing.T) {
	src := `# archive_override(module_name = "commented")
archive_override(
    module_name = "a",  # module_name = "not this"
    patches = ["x(1).patch"],
)
bazel_dep(name = "b", version = "1")
`
	got := Find(src)
	if len(got) != 1 || got[0].Module != "a" || got[0].Line != 2 {
		t.Errorf("Find = %+v", got)
	}
}
