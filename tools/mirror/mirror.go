// SPDX-License-Identifier: Apache-2.0

// Package mirror checks the archives that MODULE.bazel fetches, and
// uploads them as release assets of this repository (spec "Prebuilts").
// It is rules_kicad's tools/mirror, ported (issue #8).
//
// The manifest (manifest.json) lists each release and its assets. An asset
// is either:
//
//   - mirrored: the same bytes as an upstream URL, such as a GitLab
//     archive; or
//   - built: made from an upstream source by a script outside this tool
//     (in rules_kicad, the KiCad prebuilt; rules_fab has none yet).
//
// A dry run downloads every upstream file (each mirrored asset, and the
// source of each built asset) and checks its sha256. An upload does the
// same checks, then runs `gh release create` when the release is missing
// and `gh release upload` for its assets.
package mirror

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Manifest is the content of manifest.json.
type Manifest struct {
	// Repo is the GitHub repository of the releases, as OWNER/NAME.
	Repo     string    `json:"repo"`
	Releases []Release `json:"releases"`
}

// Release is one GitHub release and its assets.
type Release struct {
	Tag    string  `json:"tag"`
	Title  string  `json:"title"`
	Notes  string  `json:"notes"`
	Assets []Asset `json:"assets"`
}

// Asset is one file of a release. Exactly one of URL and Built is set.
type Asset struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	// URL is the upstream file a mirrored asset copies.
	URL   string `json:"url,omitempty"`
	Built *Built `json:"built,omitempty"`
}

// Built says how a built asset is made.
type Built struct {
	Source       string `json:"source"`
	SourceSHA256 string `json:"source_sha256"`
	Script       string `json:"script"`
}

var hexSHA = regexp.MustCompile(`^[0-9a-f]{64}$`)

// safeName tells whether n is a plain file name: the tool joins tags and
// asset names to directories.
func safeName(n string) bool {
	return n != "" && n != "." && n != ".." && !strings.ContainsAny(n, "/\\\x00")
}

// Parse reads and checks a manifest.
func Parse(b []byte) (*Manifest, error) {
	var m Manifest
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("manifest: more data after the manifest")
	}
	if len(m.Releases) == 0 {
		return nil, fmt.Errorf("manifest: no releases")
	}
	if !regexp.MustCompile(`^[\w.-]+/[\w.-]+$`).MatchString(m.Repo) {
		return nil, fmt.Errorf("manifest: repo %q: want OWNER/NAME", m.Repo)
	}
	tags := map[string]bool{}
	for _, r := range m.Releases {
		if !safeName(r.Tag) || tags[r.Tag] {
			return nil, fmt.Errorf("manifest: tag %q is repeated, or not a plain file name", r.Tag)
		}
		tags[r.Tag] = true
		if r.Title == "" || len(r.Assets) == 0 {
			return nil, fmt.Errorf("manifest: release %s needs a title and assets", r.Tag)
		}
		names := map[string]bool{}
		for _, a := range r.Assets {
			where := fmt.Sprintf("manifest: %s/%s", r.Tag, a.Name)
			if !safeName(a.Name) || names[a.Name] {
				return nil, fmt.Errorf("%s: the name is repeated, or not a plain file name", where)
			}
			names[a.Name] = true
			if !hexSHA.MatchString(a.SHA256) {
				return nil, fmt.Errorf("%s: sha256 %q: want 64 lowercase hex digits", where, a.SHA256)
			}
			switch {
			case (a.URL == "") == (a.Built == nil):
				return nil, fmt.Errorf("%s: give either url (mirrored) or built", where)
			case a.Built != nil && (a.Built.Source == "" || a.Built.Script == "" || !hexSHA.MatchString(a.Built.SourceSHA256)):
				return nil, fmt.Errorf("%s: built needs source, a source_sha256 of 64 hex digits, and script", where)
			}
		}
	}
	return &m, nil
}

// Download fetches url into dir/name and returns its sha256, in hex.
func Download(client *http.Client, url, dir, name string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FileSHA256 returns the sha256 of the file at p, in hex.
func FileSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Options says what Check does.
type Options struct {
	Client *http.Client
	// WorkDir gets the downloads.
	WorkDir string
	// BuiltDir holds the built assets, by name. Empty: built assets are
	// not checked, only their sources.
	BuiltDir string
	// Log gets one line per file checked.
	Log io.Writer
}

// Check downloads and checks every upstream file of release r, and the
// built assets in o.BuiltDir. It returns the path of each asset that is
// ready to upload, by name, and an error that lists every mismatch.
func Check(r Release, o Options) (map[string]string, error) {
	ready := map[string]string{}
	var problems []string
	for _, a := range r.Assets {
		if a.URL != "" {
			got, err := Download(o.Client, a.URL, o.WorkDir, a.Name)
			switch {
			case err != nil:
				problems = append(problems, fmt.Sprintf("%s: %v", a.Name, err))
			case got != a.SHA256:
				problems = append(problems, fmt.Sprintf("%s: %s has sha256 %s, the manifest says %s", a.Name, a.URL, got, a.SHA256))
			default:
				fmt.Fprintf(o.Log, "ok %s/%s %s (from %s)\n", r.Tag, a.Name, got, a.URL)
				ready[a.Name] = filepath.Join(o.WorkDir, a.Name)
			}
			continue
		}
		src := a.Name + ".source"
		got, err := Download(o.Client, a.Built.Source, o.WorkDir, src)
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s: source: %v", a.Name, err))
		case got != a.Built.SourceSHA256:
			problems = append(problems, fmt.Sprintf("%s: source %s has sha256 %s, the manifest says %s", a.Name, a.Built.Source, got, a.Built.SourceSHA256))
		default:
			fmt.Fprintf(o.Log, "ok %s/%s: source %s (from %s)\n", r.Tag, a.Name, got, a.Built.Source)
		}
		os.Remove(filepath.Join(o.WorkDir, src))
		if o.BuiltDir == "" {
			fmt.Fprintf(o.Log, "-- %s/%s: built by %s; give --built_dir to check and upload it\n", r.Tag, a.Name, a.Built.Script)
			continue
		}
		p := filepath.Join(o.BuiltDir, a.Name)
		got, err = FileSHA256(p)
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s: %v", a.Name, err))
		case got != a.SHA256:
			problems = append(problems, fmt.Sprintf("%s: %s has sha256 %s, the manifest says %s", a.Name, p, got, a.SHA256))
		default:
			fmt.Fprintf(o.Log, "ok %s/%s %s (built, from %s)\n", r.Tag, a.Name, got, p)
			ready[a.Name] = p
		}
	}
	if len(problems) > 0 {
		return ready, fmt.Errorf("%s:\n  %s", r.Tag, strings.Join(problems, "\n  "))
	}
	return ready, nil
}

// CreateCommand returns the gh command that creates release r.
func CreateCommand(repo string, r Release) []string {
	return []string{"gh", "release", "create", r.Tag, "--repo", repo, "--title", r.Title, "--notes", r.Notes}
}

// UploadCommand returns the gh command that uploads the given files to
// release r. Without --clobber, gh refuses to replace an asset that is
// already there.
func UploadCommand(repo string, r Release, files []string) []string {
	return append([]string{"gh", "release", "upload", r.Tag, "--repo", repo}, files...)
}

// Quote returns argv as a line a POSIX shell reads back as argv.
func Quote(argv []string) string {
	var out []string
	for _, a := range argv {
		if regexp.MustCompile(`^[\w@%+=:,./-]+$`).MatchString(a) {
			out = append(out, a)
		} else {
			out = append(out, "'"+strings.ReplaceAll(a, "'", `'\''`)+"'")
		}
	}
	return strings.Join(out, " ")
}

// Runner runs a command and returns what it wrote to stderr.
type Runner func(argv []string) (stderr string, err error)

// MirrorOptions says what Mirror does.
type MirrorOptions struct {
	Options
	// DryRun checks and prints the commands, and runs none.
	DryRun bool
	// Release limits the run to one tag; empty is every release.
	Release string
	// Run runs a gh command. Mirror calls it only when DryRun is false.
	Run Runner
}

// Mirror checks every selected release first, each in its own directory
// under o.WorkDir. Only when all of them match does it print, or run, the
// gh commands: `gh release create` for a release that is missing, then
// `gh release upload`.
func Mirror(m *Manifest, o MirrorOptions) error {
	type plan struct {
		r     Release
		files []string
	}
	var plans []plan
	var problems []string
	for _, r := range m.Releases {
		if o.Release != "" && r.Tag != o.Release {
			continue
		}
		ro := o.Options
		ro.WorkDir = filepath.Join(o.WorkDir, r.Tag)
		if err := os.MkdirAll(ro.WorkDir, 0o755); err != nil {
			return err
		}
		ready, err := Check(r, ro)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		p := plan{r: r}
		for _, a := range r.Assets {
			switch f, ok := ready[a.Name]; {
			case ok:
				p.files = append(p.files, f)
			case o.DryRun:
				// Where the upload will look for the built file.
				p.files = append(p.files, filepath.Join("BUILT_DIR", a.Name))
			default:
				problems = append(problems, fmt.Sprintf("%s/%s: built by %s; give --built_dir to upload it", r.Tag, a.Name, a.Built.Script))
			}
		}
		plans = append(plans, p)
	}
	if o.Release != "" && len(plans) == 0 && len(problems) == 0 {
		return fmt.Errorf("no release %q in the manifest", o.Release)
	}
	if len(problems) > 0 {
		return fmt.Errorf("nothing uploaded:\n%s", strings.Join(problems, "\n"))
	}
	for _, p := range plans {
		create := CreateCommand(m.Repo, p.r)
		upload := UploadCommand(m.Repo, p.r, p.files)
		if o.DryRun {
			fmt.Fprintln(o.Log, "would run, if the release is missing:", Quote(create))
			fmt.Fprintln(o.Log, "would run:", Quote(upload))
			continue
		}
		stderr, err := o.Run([]string{"gh", "release", "view", p.r.Tag, "--repo", m.Repo})
		switch {
		case err == nil:
		// gh says "release not found"; some versions report the API's 404.
		case strings.Contains(stderr, "release not found") || strings.Contains(stderr, "HTTP 404"):
			if _, err := o.Run(create); err != nil {
				return fmt.Errorf("%s: %w", Quote(create), err)
			}
		default:
			return fmt.Errorf("gh release view %s: %w", p.r.Tag, err)
		}
		if _, err := o.Run(upload); err != nil {
			return fmt.Errorf("%s: %w", Quote(upload), err)
		}
	}
	if o.DryRun {
		fmt.Fprintln(o.Log, "dry run: every file checked matches the manifest; nothing uploaded")
	}
	return nil
}
