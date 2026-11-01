<!-- SPDX-License-Identifier: Apache-2.0 -->
# Instructions

Read `README.md` first.

The coding standard is the `ai-coding-sop` repository at
`https://github.com/filmil/ai-coding-sop`.
Its `AGENTS.md`, its `prose-readability` skill, its `git-commit-rules`
skill and its `hermetic`, `bazel` and `bazel-bzl-files` fragments apply
here in full.
Where this file and ai-coding-sop differ, ai-coding-sop wins; ask the
operator about a conflict that matters.

# Agent operating rules

A change is complete only when `bazel test //...` passes, the integration
module passes (`cd integration && bazel test //...`), and new tests cover
the change.

1. Edit text sources only: `BUILD.bazel`, `MODULE.bazel`, `.bzl`, Go
   sources, vendor tables. Do not commit generated outputs, except the
   docs that `bazel run //docs:update_docs` writes.
2. Keep every action hermetic. No host tools, no host files outside the
   workspace, no host environment variables, no network in actions. Add
   each external file with `http_archive` or `http_file` and a pinned
   `sha256`. `--action_env=PATH`, `use_default_shell_env = True` and the
   tags `local` and `no-sandbox` are forbidden. If a tool cannot run
   hermetically, stop and ask; do not make it work with a host install.
3. Find targets with `bazel query` and `bazel cquery`. Run
   `bazel run //tools:buildifier` on every change.
4. Once there are golden files, update them only with
   `bazel run //:update_goldens`, and give the reason in the commit
   message.
5. A change to a package layout lists the files and columns of the
   package before and after in the pull request. A change to a swatch
   board (`fab_swatch`, `fab_assembly_swatch` or their generators)
   attaches the `kicad_render` PNGs of both sides of each board it
   changes: push them to a branch `pr-assets/<the PR's branch>` of this
   repository, link them from the pull request by their
   `raw.githubusercontent.com` URLs at that branch's commit, and ask the
   operator to delete the branch after the merge.
6. One logical change per pull request: one rule, one feature, or one
   design change.
7. Stop and ask the operator before you:
   * change or remove a DRC or fab rule file;
   * delete a test, or make a tolerance larger;
   * change a toolchain version or a library version (the pull request
     also updates the version wherever `README.md` names it: under
     Licenses, and in its examples, such as the `hdlf_rules_kicad` pin);
   * run a release or publish workflow, or set a secret.
8. When a test fails and the cause is not clear after three attempts, stop.
   Write the failure and your hypotheses in the pull request.

# Rules particular to this repository

* **Tools come from toolchains only.** Every action runs its tool through
  `run_tool` in `//fab/private` (issue #4), with the tool from
  `ctx.toolchains`.
  No private attribute names an external tool.
* **Public packages name no dev dependency.** A user of the module loads
  `//fab` and `//`, where `@stardoc`, `@bazel_lib`, `@buildifier_prebuilt`
  and `@hermetic_cc_toolchain` are not visible. Documentation targets live in
  `//docs` and development tools in `//tools`.
* **Documentation is generated.** After changing a `.bzl` file, run
  `bazel run //docs:update_docs`; `bazel test //...` fails until then.
* **Linux x86_64 only** in v1.
* **Commits for a BCR pull request carry no `Co-Authored-By` trailer**
  (the `bazel-bcr-publish` skill says why).

`CLAUDE.md` and `GEMINI.md` are symlinks to this file.
