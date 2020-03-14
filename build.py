#!/usr/bin/env python3

import subprocess
import sys

GO_OS_ARCH_LIST = [
    ["darwin", "amd64"],
    ["linux", "amd64"],
    ["linux", "arm"],
    ["linux", "arm64"],
    ["linux", "mips", "softfloat"],
    ["linux", "mips", "hardfloat"],
    ["linux", "mipsle", "softfloat"],
    ["linux", "mipsle", "hardfloat"],
    ["windows", "amd64"]
]


def go_build():
    version = subprocess.check_output(
        "git describe --tags", shell=True).decode()
    for o, a, *p in GO_OS_ARCH_LIST:
        zip_name = "doublebarrel-" + o + "-" + a + \
            ("-" + (p[0] if p else "") if p else "")
        binary_name = zip_name + (".exe" if o == "windows" else "")
        mipsflag = (" GOMIPS=" + (p[0] if p else "") if p else "")
        command = "GOOS=" + o + " GOARCH=" + a + mipsflag + " CGO_ENABLED=0" + \
            " go build -ldflags \"-s -w " + "-X main.version=" + \
            version + "\" -o " + binary_name + " main.go"
        print(command)
        subprocess.check_call(command, shell=True)

        subprocess.check_call("zip " +"output/" +zip_name + ".zip " +
                              binary_name + " " + "config.json cidrlist", shell=True)


if __name__ == "__main__":
    if "-build" in sys.argv:
        subprocess.check_call("mkdir output", shell=True)
        go_build()
