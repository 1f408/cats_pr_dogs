#!/bin/bash
set -eu
export GO111MODULE=on

SRC_DIR=$(pwd)/src
DEB_DIR=$(pwd)/deb
ICON_DIR=$(pwd)/icons

SUFFIX=""

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

MD_APP="CatPrMd"
TMPL_APP="CatPrTmpl"
MD_CMD="cat_pr_md"
TMPL_CMD="cat_pr_tmpl"
MD_PKG="cat-pr-md"
TMPL_PKG="cat-pr-tmpl"
if [[ ! -z $SUFFIX ]]; then
    MD_APP="${MD_APP}-${SUFFIX}"
    TMPL_APP="${TMPL_APP}-${SUFFIX}"
    MD_CMD="${MD_CMD}-${SUFFIX}"
    TMPL_CMD="${TMPL_CMD}-${SUFFIX}"
    MD_PKG="${MD_PKG}-${SUFFIX}"
    TMPL_PKG="${TMPL_PKG}-${SUFFIX}"
fi

declare -a DEB_LIST=(
    "cat_pr_md ${MD_CMD} ${MD_PKG} ${MD_APP}"
    "cat_pr_tmpl ${TMPL_CMD} ${TMPL_PKG} ${TMPL_APP}"
)

for cmd in go desktop-file-validate dpkg-deb fakeroot; do
	type -P ${cmd} >/dev/null || {
	    echo "${cmd} is not installed"
	    exit 2
	}
done

case $(uname -m) in
    "x86_64")
	ARCH=amd64;;
    "aarch64")
	ARCH=arm64;;
    *)
	echo "unknown machine architecture."
	exit 2
esac

function build_cmd () {
    set -eu
    local srcname="$1"
    local cmd="$2"
    local dir="$3"

    local src="${SRC_DIR}/${srcname}"
    if [ ! -d "${dir}" ]; then
        return 1
    fi
    echo "build cmd: ${dir}/${cmd}"
    (
        cd "${src}"
        go build -o "${dir}/${cmd}"
    )
}

function mk_deb () {
    set -eu
    local SRC="$1"
    local CMD="$2"
    local PKG="$3"
    local APP="$4"
    TMP_ROOT="${DEB_DIR}/${PKG}_1.0.0"

    echo "build deb: ${PKG} ${APP}"
    if [ -d "${TMP_ROOT}" ]; then
       rm -r "${TMP_ROOT}"
    fi
    mkdir -p ${TMP_ROOT}/usr/bin
    mkdir -p ${TMP_ROOT}/usr/share/applications
    mkdir -p ${TMP_ROOT}/DEBIAN
    build_cmd ${SRC} "${CMD}" ${TMP_ROOT}/usr/bin

    mkdir -p ${TMP_ROOT}/usr/share/icons/hicolor/scalable/apps
    cp ${ICON_DIR}/${SRC}.svg ${TMP_ROOT}/usr/share/icons/hicolor/scalable/apps/${PKG}.svg

    cat > ${TMP_ROOT}/usr/share/applications/${APP}.desktop << EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=${APP}
Exec=${CMD}
Icon=${PKG}
Terminal=false
StartupWMClass=${APP}
EOF

    cat > ${TMP_ROOT}/DEBIAN/control << EOF
Package: ${PKG}
Version: 1.0-0
Section: base
Priority: optional
Architecture: ${ARCH}
Maintainer: 1f408 GitHub Organization https://github.com/1f408 
Description: ${APP} for previewing cats_dogs Markdown
EOF

    fakeroot dpkg-deb --build ${TMP_ROOT}
    rm -rf ${TMP_ROOT}
}

echo "build: ${DEB_DIR}"
if [ -d "${DEB_DIR}" ]; then
    rm -rf "${DEB_DIR}"
fi
mkdir -p "${DEB_DIR}"

echo "go generate"
./generate.sh

for param in "${DEB_LIST[@]}"; do
    mk_deb ${param}
done
