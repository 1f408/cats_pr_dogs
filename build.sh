#!/bin/bash
set -eu
export GO111MODULE=on
OUT_DIR=$(pwd)/bin

function build () {
    local cmd="$1"
    echo "build ${cmd}: "
    (
        cd "src/${cmd}" || exit 1
        #go build -o "${OUT_DIR}" -ldflags '-s -w' || exit 2
        go build -o "${OUT_DIR}" || exit 2
    )
}

echo "build bin: ${OUT_DIR}"
mkdir -p "${OUT_DIR}"

echo "go generate"
./generate.sh

if [ $# -eq 0 ]; then
    ls -1 src | while read cmd ; do
        build "${cmd}"
    done
else
    for cmd in "$@"; do
        build "${cmd}"
    done
fi

exit 0
