#!/bin/bash
set -eu
export GO111MODULE=on
export GOOS=${GOOS:-windows}
export GOARCH=${GOARCH:-$(go env GOARCH)}

EXE_DIR=$(pwd)/exe

SUFFIX=""
MD_CMD="cat_pr_md.exe"
TMPL_CMD="cat_pr_tmpl.exe"


if [[ $# -gt 1 ]]; then
    echo "Usage: $0 [<appname suffix>]"
    exit 1
fi

if [[ $# -eq 1 ]]; then
    SUFFIX="$1"
fi

if [[ $# -gt 1 ]]; then
    echo "Usage: $0 [<appname suffix>]"
    exit 1
fi

if [[ ! -z $SUFFIX ]]; then
    MD_CMD="cat_pr_md-${SUFFIX}.exe"
    TMPL_CMD="cat_pr_tmpl-${SUFFIX}.exe"
fi

declare -a CMD_LIST=(
    "cat_pr_md ${MD_CMD}"
    "cat_pr_tmpl ${TMPL_CMD}"
)

type -P go >/dev/null || {
    echo "go is not installed"
    exit 2
}

type -P go-winres >/dev/null || {
    echo "go-winres is not installed"
    echo "install command: go install github.com/tc-hib/go-winres@latest"
    exit 2
}

function build_exe () {
    set -eu
    local cmd="$1"
    local cmdname="$2"
    mkdir -p "${EXE_DIR}"
    echo "build exe: ${cmd}"
    (
        cd "src/${cmd}"
        go-winres make
        echo go build -o "${EXE_DIR}/${cmdname}" -ldflags="-H windowsgui"
        go build -o "${EXE_DIR}/${cmdname}" -ldflags="-H windowsgui"
        rm -f *.syso
    )
}

echo "build: ${EXE_DIR}"
if [ -d "${EXE_DIR}" ]; then
    rm -rf "${EXE_DIR}"
fi
mkdir -p "${EXE_DIR}"

echo "go generate"
./generate.sh

for param in "${CMD_LIST[@]}"; do
    build_exe ${param}
done

exit 0
