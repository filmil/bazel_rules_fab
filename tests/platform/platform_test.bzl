# SPDX-License-Identifier: Apache-2.0
"""Analysis test for spec P1: the fallback toolchain's message.

This checks the message on the fallback toolchain target itself. That
Bazel resolves to it on another host is checked by a CI step that builds
with --host_platform set to //tests/platform:macos_arm64: an analysis test
cannot, because under expect_failure the fallback's failure becomes a
target without ToolchainInfo, and toolchain resolution then aborts the
build.
"""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _fails_with_message_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "rules_fab v1 supports only Linux x86_64 hosts")
    return analysistest.end(env)

fallback_fails_test = analysistest.make(
    _fails_with_message_impl,
    expect_failure = True,
)
