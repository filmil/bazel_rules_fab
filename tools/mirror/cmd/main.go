// SPDX-License-Identifier: Apache-2.0

// Command mirror checks the prebuilt archives of tools/mirror/manifest.json
// and, for the operator, uploads them as release assets.
//
// Check every upstream file (what agents run; this is the default):
//
//	bazel run //tools:mirror
//
// Upload (the operator only; it needs gh, logged in with write access):
//
//	bazel run //tools:mirror -- --dry_run=false --built_dir=/path/to/built
//
// --built_dir holds the built assets by name, made by a script outside
// this tool (rules_fab has none yet; kicad-druid is mirrored as it is). Each command is printed before it
// runs. An upload stops before running any command if a file does not
// match the manifest, in any release. gh refuses to replace an asset that
// is already in the release.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/filmil/bazel_rules_fab/tools/mirror"
)

func run() error {
	root := flag.String("root", os.Getenv("BUILD_WORKSPACE_DIRECTORY"),
		"repository root (default: the workspace of `bazel run`)")
	manifestPath := flag.String("manifest", "tools/mirror/manifest.json", "the manifest, absolute or relative to --root")
	dryRun := flag.Bool("dry_run", true, "check the files only; --dry_run=false uploads them")
	builtDir := flag.String("built_dir", "", "a directory with the built assets, by name")
	release := flag.String("release", "", "only this release tag (default: all)")
	workDir := flag.String("work_dir", "", "where to download (default: a new temporary directory)")
	flag.Parse()
	path := *manifestPath
	if !filepath.IsAbs(path) {
		if *root == "" {
			return fmt.Errorf("no root; run it with `bazel run //tools:mirror` or pass --root")
		}
		path = filepath.Join(*root, path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m, err := mirror.Parse(b)
	if err != nil {
		return err
	}
	if *workDir == "" {
		d, err := os.MkdirTemp("", "mirror-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(d)
		*workDir = d
	}
	return mirror.Mirror(m, mirror.MirrorOptions{
		Options: mirror.Options{
			Client:   &http.Client{Timeout: 30 * time.Minute},
			WorkDir:  *workDir,
			BuiltDir: *builtDir,
			Log:      os.Stdout,
		},
		DryRun:  *dryRun,
		Release: *release,
		Run:     runCommand,
	})
}

// runCommand prints argv, runs it, and returns its stderr as well as
// showing it.
func runCommand(argv []string) (string, error) {
	fmt.Println("running:", mirror.Quote(argv))
	var stderr bytes.Buffer
	c := exec.Command(argv[0], argv[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = io.MultiWriter(os.Stderr, &stderr)
	err := c.Run()
	return stderr.String(), err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mirror:", err)
		os.Exit(1)
	}
}
