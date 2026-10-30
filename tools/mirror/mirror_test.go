// SPDX-License-Identifier: Apache-2.0

package mirror

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// readRunfile reads a file of this repository from the test's runfiles.
func readRunfile(path string) ([]byte, error) {
	p, err := runfiles.Rlocation("_main/" + path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

func readManifest(t *testing.T) *Manifest {
	t.Helper()
	b, err := readRunfile("tools/mirror/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// The manifest names what MODULE.bazel fetches: every asset of the
// manifest has its sha256 there, and is fetched from this repository's
// release and from its upstream (a mirrored asset's URL, a built asset's
// source); every URL there is one of those. Then the manifest and
// MODULE.bazel cannot drift apart.
func TestManifestMatchesModule(t *testing.T) {
	m := readManifest(t)
	mod, err := readRunfile("MODULE.bazel")
	if err != nil {
		t.Fatal(err)
	}
	src := string(mod)
	shas := map[string]bool{}
	for _, s := range regexp.MustCompile(`sha256 = "([0-9a-f]{64})"`).FindAllStringSubmatch(src, -1) {
		shas[s[1]] = true
	}
	urls := map[string]bool{}
	for _, u := range regexp.MustCompile(`"(https://[^"]+)"`).FindAllStringSubmatch(src, -1) {
		urls[u[1]] = true
	}
	prefix := "https://github.com/" + m.Repo + "/releases/download/"
	known := map[string]bool{}
	for _, r := range m.Releases {
		for _, a := range r.Assets {
			if !shas[a.SHA256] {
				t.Errorf("%s/%s: sha256 %s is not in MODULE.bazel", r.Tag, a.Name, a.SHA256)
			}
			mirror := prefix + r.Tag + "/" + a.Name
			known[mirror] = true
			if !urls[mirror] {
				t.Errorf("%s/%s: MODULE.bazel does not fetch it from %s", r.Tag, a.Name, mirror)
			}
			if a.URL != "" {
				known[a.URL] = true
				if !urls[a.URL] {
					t.Errorf("%s/%s: MODULE.bazel does not fetch it from its upstream %s", r.Tag, a.Name, a.URL)
				}
			}
		}
	}
	// The override of rules_kicad (issue #7) is the one other archive.
	for u := range urls {
		if !known[u] && !strings.HasPrefix(u, "https://github.com/filmil/bazel_rules_kicad/archive/") {
			t.Errorf("MODULE.bazel fetches %s, which is neither a mirrored asset nor its upstream", u)
		}
	}
}

func TestParseRejects(t *testing.T) {
	good := `{"repo": "o/r", "releases": [{"tag": "t", "title": "T", "notes": "n", "assets": [ASSET]}]}`
	sha := strings.Repeat("a", 64)
	for name, asset := range map[string]string{
		"no source":     `{"name": "a", "sha256": "` + sha + `"}`,
		"both sources":  `{"name": "a", "sha256": "` + sha + `", "url": "https://x/a", "built": {"source": "https://x/s", "source_sha256": "` + sha + `", "script": "s.sh"}}`,
		"short sha":     `{"name": "a", "sha256": "abc", "url": "https://x/a"}`,
		"upper sha":     `{"name": "a", "sha256": "` + strings.Repeat("A", 64) + `", "url": "https://x/a"}`,
		"slash in name": `{"name": "d/a", "sha256": "` + sha + `", "url": "https://x/a"}`,
		"no script":     `{"name": "a", "sha256": "` + sha + `", "built": {"source": "https://x/s", "source_sha256": "` + sha + `"}}`,
		"unknown field": `{"name": "a", "sha256": "` + sha + `", "url": "https://x/a", "sha1": "x"}`,
		"dot dot":       `{"name": "..", "sha256": "` + sha + `", "url": "https://x/a"}`,
		"dot":           `{"name": ".", "sha256": "` + sha + `", "url": "https://x/a"}`,
	} {
		if _, err := Parse([]byte(strings.Replace(good, "ASSET", asset, 1))); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	ok := `{"name": "a", "sha256": "` + sha + `", "url": "https://x/a"}`
	if _, err := Parse([]byte(strings.Replace(good, "ASSET", ok, 1))); err != nil {
		t.Errorf("a good manifest: %v", err)
	}
	if _, err := Parse([]byte(`{"repo": "o/r", "releases": [{"tag": "t", "title": "T", "assets": [` + ok + `]}, {"tag": "t", "title": "T", "assets": [` + ok + `]}]}`)); err == nil {
		t.Errorf("a repeated tag parsed")
	}
	for name, m := range map[string]string{
		"slash in tag":  strings.Replace(strings.Replace(good, "ASSET", ok, 1), `"tag": "t"`, `"tag": "a/b"`, 1),
		"trailing data": strings.Replace(good, "ASSET", ok, 1) + ` {}`,
		"no releases":   `{"repo": "o/r", "releases": []}`,
	} {
		if _, err := Parse([]byte(m)); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

// A dry run against a local server: no network.
func TestCheck(t *testing.T) {
	files := map[string]string{"/lib.tar.gz": "library", "/app.tar": "appimage"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	defer srv.Close()
	built := t.TempDir()
	if err := os.WriteFile(filepath.Join(built, "kicad.tar.xz"), []byte("prebuilt"), 0o644); err != nil {
		t.Fatal(err)
	}
	release := func(libSHA, srcSHA, builtSHA, libPath string) Release {
		return Release{Tag: "t", Title: "T", Assets: []Asset{
			{Name: "lib.tar.gz", SHA256: libSHA, URL: srv.URL + libPath},
			{Name: "kicad.tar.xz", SHA256: builtSHA, Built: &Built{Source: srv.URL + "/app.tar", SourceSHA256: srcSHA, Script: "make.sh"}},
		}}
	}
	opts := func(builtDir string) Options {
		return Options{Client: srv.Client(), WorkDir: t.TempDir(), BuiltDir: builtDir, Log: &bytes.Buffer{}}
	}
	good := release(sum("library"), sum("appimage"), sum("prebuilt"), "/lib.tar.gz")

	// Without --built_dir: the source of the built asset is checked, and
	// only the mirrored asset is ready.
	ready, err := Check(good, opts(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready["lib.tar.gz"] == "" {
		t.Errorf("ready %v, want lib.tar.gz only", ready)
	}
	if b, _ := os.ReadFile(ready["lib.tar.gz"]); string(b) != "library" {
		t.Errorf("downloaded %q", b)
	}

	// With it, both are ready.
	if ready, err := Check(good, opts(built)); err != nil || len(ready) != 2 {
		t.Errorf("ready %v, err %v; want both", ready, err)
	}

	// Every mismatch is reported, not only the first.
	for name, c := range map[string]struct {
		r    Release
		want []string
	}{
		"bad sha256s": {release(sum("other"), sum("other"), sum("other"), "/lib.tar.gz"),
			[]string{"lib.tar.gz: " + srv.URL + "/lib.tar.gz has sha256", "kicad.tar.xz: source", "kicad.tar.xz: " + built}},
		"missing": {release(sum("library"), sum("appimage"), sum("prebuilt"), "/gone"),
			[]string{"404 Not Found"}},
	} {
		_, err := Check(c.r, opts(built))
		if err == nil {
			t.Errorf("%s: no error", name)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: %v\nwant %q in it", name, err, w)
			}
		}
	}
}

// MODULE.bazel names a built asset, not its source. The release notes name
// the source, so the manifest's source must be the one there.
func TestBuiltSourceInNotes(t *testing.T) {
	for _, r := range readManifest(t).Releases {
		for _, a := range r.Assets {
			if a.Built == nil {
				continue
			}
			for _, want := range []string{a.Built.Source, a.Built.SourceSHA256, a.Built.Script} {
				if !strings.Contains(r.Notes, want) {
					t.Errorf("%s/%s: the notes do not name %s", r.Tag, a.Name, want)
				}
			}
		}
	}
}

// The commands the operator sees and runs.
func TestCommands(t *testing.T) {
	r := Release{Tag: "prebuilt-kicad-libs-9.0.9", Title: "KiCad 9.0.9 libraries", Notes: "As GitLab served them; it's CC-BY-SA."}
	if got, want := Quote(CreateCommand("filmil/bazel_rules_kicad", r)),
		`gh release create prebuilt-kicad-libs-9.0.9 --repo filmil/bazel_rules_kicad --title 'KiCad 9.0.9 libraries' --notes 'As GitLab served them; it'\''s CC-BY-SA.'`; got != want {
		t.Errorf("create\n got %s\nwant %s", got, want)
	}
	if got, want := Quote(UploadCommand("filmil/bazel_rules_kicad", r, []string{"/tmp/w/a.tar.gz", "/tmp/w/b c.tar.gz"})),
		`gh release upload prebuilt-kicad-libs-9.0.9 --repo filmil/bazel_rules_kicad /tmp/w/a.tar.gz '/tmp/w/b c.tar.gz'`; got != want {
		t.Errorf("upload\n got %s\nwant %s", got, want)
	}
	if got, want := Quote([]string{"--notes", ""}), `--notes ''`; got != want {
		t.Errorf("empty argument: got %s, want %s", got, want)
	}
}

// Mirror, with a runner that records the commands instead of running gh.
func TestMirror(t *testing.T) {
	files := map[string]string{"/a": "aaa", "/b": "bbb", "/src": "source"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := files[r.URL.Path]; ok {
			w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	built := t.TempDir()
	if err := os.WriteFile(filepath.Join(built, "k.tar.xz"), []byte("kicad"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := func(bSHA string) *Manifest {
		return &Manifest{Repo: "o/r", Releases: []Release{
			{Tag: "one", Title: "One", Assets: []Asset{{Name: "a", SHA256: sum("aaa"), URL: srv.URL + "/a"}}},
			{Tag: "two", Title: "Two", Assets: []Asset{
				{Name: "b", SHA256: bSHA, URL: srv.URL + "/b"},
				{Name: "k.tar.xz", SHA256: sum("kicad"), Built: &Built{Source: srv.URL + "/src", SourceSHA256: sum("source"), Script: "make.sh"}},
			}},
		}}
	}
	type result struct {
		ran []string
		log string
		err error
	}
	run := func(m *Manifest, dry bool, release, builtDir string, view func(tag string) (string, error)) result {
		var r result
		var log bytes.Buffer
		r.err = Mirror(m, MirrorOptions{
			Options: Options{Client: srv.Client(), WorkDir: t.TempDir(), BuiltDir: builtDir, Log: &log},
			DryRun:  dry, Release: release,
			Run: func(argv []string) (string, error) {
				r.ran = append(r.ran, strings.Join(argv[:4], " "))
				if argv[2] == "view" {
					return view(argv[3])
				}
				return "", nil
			},
		})
		r.log = log.String()
		return r
	}
	exists := func(string) (string, error) { return "", nil }
	missing := func(tag string) (string, error) {
		if tag == "two" {
			return "release not found", errors.New("exit status 1")
		}
		return "", nil
	}

	// A mismatch in the second release: no command runs, not even for the
	// first, which matches.
	if r := run(manifest(sum("other")), false, "", built, exists); r.err == nil || len(r.ran) != 0 {
		t.Errorf("mismatch: err %v, ran %v; want an error and no command", r.err, r.ran)
	}
	// An upload without --built_dir: no command runs.
	if r := run(manifest(sum("bbb")), false, "", "", exists); r.err == nil || !strings.Contains(r.err.Error(), "give --built_dir") || len(r.ran) != 0 {
		t.Errorf("no built_dir: err %v, ran %v", r.err, r.ran)
	}
	// A dry run runs nothing, and prints both commands of each release.
	if r := run(manifest(sum("bbb")), true, "", "", exists); r.err != nil || len(r.ran) != 0 || strings.Count(r.log, "would run") != 4 || !strings.Contains(r.log, "BUILT_DIR/k.tar.xz") {
		t.Errorf("dry run: err %v, ran %v, log\n%s", r.err, r.ran, r.log)
	}
	// An upload: a missing release is created first.
	r := run(manifest(sum("bbb")), false, "", built, missing)
	if want := "gh release view one|gh release upload one|gh release view two|gh release create two|gh release upload two"; r.err != nil || strings.Join(r.ran, "|") != want {
		t.Errorf("upload: err %v\n ran %v\nwant %s", r.err, r.ran, want)
	}
	// Any other failure of gh release view stops the upload.
	r = run(manifest(sum("bbb")), false, "", built, func(string) (string, error) { return "HTTP 401: Bad credentials", errors.New("exit status 1") })
	if r.err == nil || len(r.ran) != 1 {
		t.Errorf("view fails: err %v, ran %v; want an error after the view", r.err, r.ran)
	}
	// --release picks one release; an unknown one is an error.
	if r := run(manifest(sum("other")), true, "one", "", exists); r.err != nil || strings.Contains(r.log, " two") {
		t.Errorf("--release=one: err %v, log\n%s", r.err, r.log)
	}
	if r := run(manifest(sum("bbb")), true, "three", "", exists); r.err == nil {
		t.Errorf("--release=three: no error")
	}
}
