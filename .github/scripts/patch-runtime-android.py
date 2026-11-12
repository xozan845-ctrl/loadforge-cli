#!/usr/bin/env python3
"""Patch the Go runtime so android/arm64 binaries accept shell arguments.

When an android/arm64 binary is launched from a shell (as Termux does), the
Android linker inserts the executable's absolute path as argv[1]. The Go
toolchain shipped by Termux patches the runtime to drop that extra element,
but the official toolchain used on CI does not: it is designed for binaries
launched through app_process.

This script produces an overlay file that replaces the runtime's goargs()
with the Termux-compatible version. It only builds with CGO_ENABLED=0, which
is the configuration used for the Termux release asset.
"""

import json
import os
import subprocess

OLD = """\targslice = make([]string, argc)
\tfor i := int32(0); i < argc; i++ {
\t\targslice[i] = gostringnocopy(argv_index(argv, i))
\t}"""

NEW = """\t// Android's linker inserts the executable path as argv[1] when the
\t// binary is launched from a shell. Drop it.
\tvar adjust int32
\tif GOOS == "android" {
\t\tadjust = 1
\t}
\targslice = make([]string, argc-adjust)
\tfor i := int32(0); i < argc-adjust; i++ {
\t\tidx := i
\t\tif i > 0 {
\t\t\tidx += adjust
\t\t}
\t\targslice[i] = gostringnocopy(argv_index(argv, idx))
\t}"""


def main() -> None:
    goroot = os.environ.get("GOROOT") or subprocess.check_output(
        ["go", "env", "GOROOT"], text=True
    ).strip()
    source_path = os.path.join(goroot, "src", "runtime", "runtime1.go")
    with open(source_path, encoding="utf-8") as f:
        source = f.read()

    if OLD not in source:
        raise SystemExit(f"goargs block not found in {source_path}")

    patched = "/tmp/runtime1_android.go"
    with open(patched, "w", encoding="utf-8") as f:
        f.write(source.replace(OLD, NEW, 1))

    overlay = {"Replace": {source_path: patched}}
    with open("/tmp/overlay.json", "w", encoding="utf-8") as f:
        json.dump(overlay, f)
        f.write("\n")


if __name__ == "__main__":
    main()
