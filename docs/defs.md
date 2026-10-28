<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Public API of rules_fab.

Load every rule from this file. For now it holds only the API version:

```starlark
load("@rules_fab//fab:defs.bzl", "RULES_FAB_API_VERSION")
```

The rules arrive in milestone M3 (see the issues of
https://github.com/filmil/bazel_rules_fab). This file is the only public
entry point; everything under `//fab/private` may change without notice.

