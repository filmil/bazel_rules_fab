# SPDX-License-Identifier: Apache-2.0
"""An analysis test that a target fails analysis with a given message."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _fails_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, ctx.attr.message)
    return analysistest.end(env)

fails_test = analysistest.make(
    _fails_impl,
    expect_failure = True,
    attrs = {"message": attr.string(mandatory = True)},
)
