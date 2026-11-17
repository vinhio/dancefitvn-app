#!/usr/bin/env bash
# Adapted from jplayer-mobile/scripts/dev-build.sh at 49de3c2.
#
# Builds a developer's DanceFITVN: the app with the dancefitvndev tag, which
# reads the variables of the shot script (DFV_STATE, DFV_SETTLE,
# DFV_OFFLINE, DFV_SHOT_LOG; internal/app/devenv*.go): from the environment
# on iOS (simctl launch passes SIMCTL_CHILD_*), from the launch intent's
# string extras on Android (am start --es DFV_STATE M02). The build that
# ships, plain gdesk build, reads none of them.
#
#   scripts/dev-build.sh ios|android|tvos [gdesk build flags...]
#
# gdesk build takes -tags itself (kits/cli README: a -tags in GOFLAGS would
# be replaced). An iOS build then gets NSFaceIDUsageDescription
# (scripts/ios-faceid-plist.sh, until gDesk's G3a), so that Face ID works in
# it. Extra tags: DFV_TAGS=a,b.
set -euo pipefail
cd "$(dirname "$0")/.."

sys="${1:?usage: scripts/dev-build.sh ios|android|tvos [flags...]}"
shift
tags="dancefitvndev${DFV_TAGS:+,$DFV_TAGS}"
gdesk build -os "$sys" -tags "$tags" "$@"

if [[ "$sys" == ios ]]; then
	app=dist/ios-simulator-arm64/DanceFITVN.app
	for a in "$@"; do
		[[ "$a" == -device ]] && app=dist/ios-arm64/DanceFITVN.app
	done
	scripts/ios-faceid-plist.sh "$app"
fi
