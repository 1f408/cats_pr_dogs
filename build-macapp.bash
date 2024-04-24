#!/bin/bash
set -eu
export GO111MODULE=on

APP_DIR=$(pwd)/app
ICON_DIR=$(pwd)/icons

SUFFIX=""
MD_APP="CatPrMd.app"
TMPL_APP="CatPrTmpl.app"

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
    MD_APP="CatPrMd-${SUFFIX}.app"
    TMPL_APP="CatPrTmpl-${SUFFIX}.app"
fi

declare -a APP_LIST=(
    "cat_pr_md ${MD_APP} cat_pr_md.icns cat_pr_md_24x24.png"
    "cat_pr_tmpl ${TMPL_APP} cat_pr_tmpl.icns cat_pr_tmpl_24x24.png"
)

function build_app () {
    set -eu
    local cmd="$1"
    local app="$2"
    local bin_dir=${APP_DIR}/${app}/Contents/MacOS
    mkdir -p "${bin_dir}"
    echo "build ${cmd}"
    (
        cd "src/${cmd}"
        go build -o "${bin_dir}"
    )
}

function mk_app () {
    set -eu
    local CMD="$1"
    local APP="$2"
    local ICNS="$3"
    local PNG="$4"

    local APP_ID="com.github.1f408.cats_pr_dogs.${APP}"

    echo "build app: ${CMD} ${APP} ${ICNS}"
    test -d "${APP_DIR}/${APP}" && rm -r "${APP_DIR}/${APP}"
    mkdir -p "${APP_DIR}/${APP}"/Contents/{MacOS,Resources}
    cp ${ICON_DIR}/${ICNS} "${APP_DIR}/${APP}"/Contents/Resources/${CMD}.icns
    cp ${ICON_DIR}/${PNG} "${APP_DIR}/${APP}"/Contents/Resources/${CMD}.png

    build_app ${CMD} "${APP}" || { rm -fr "${APP_DIR}/${APP}"; return; }

    cat > "${APP_DIR}/${APP}"/Contents/Info.plist << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>${APP}</string>
	<key>CFBundleDisplayName</key>
	<string>${APP}</string>
	<key>CFBundleIdentifier</key>
	<string>${APP_ID}</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleExecutable</key>
	<string>${CMD}</string>
	<key>CFBundleIconFile</key>
	<string>${ICNS}</string>
</dict>
</plist>
EOF

}

echo "build: ${APP_DIR}"
if [ -d "${APP_DIR}" ]; then
    rm -rf "${APP_DIR}"
fi
mkdir -p "${APP_DIR}"

echo "go generate"
./generate.sh

for param in "${APP_LIST[@]}"; do
    mk_app ${param}
done
