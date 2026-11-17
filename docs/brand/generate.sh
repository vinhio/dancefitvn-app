#!/bin/sh
# Adapted from jplayer-mobile/docs/brand/generate.sh at 49de3c2.
#
# Make every raster file of DanceFITVN's brand from the SVG sources in docs/brand (docs/brand/BRAND.md): the
# traced wordmark, the iOS and Android icons, the Play Store pictures and the tvOS brand assets. Needs the
# Playwright venv local/.venv (Chromium, Pillow: BRAND.md section 1) and Go; the output is committed, so a build
# needs neither.
#
#   docs/brand/generate.sh          make everything, then run cmd/brandcheck
#
# Same sources, same Chromium, same bytes: render.py re-encodes every picture with fixed settings.
set -eu
cd "$(dirname "$0")/../.."
PY=local/.venv/bin/python
SRC=docs/brand/mobile
OUT=$SRC/out
TV=docs/brand/tv
BA=assets/tvos/DanceFITVN.brandassets
R="$PY -I scripts/brand/render.py"

mkdir -p "$OUT/android" "$OUT/play"

# The wordmark of the site logo, traced (with and without the registered sign).
$PY -I scripts/brand/trace.py docs/brand/source/dancefitvn.png docs/brand/wordmark.svg
$PY -I scripts/brand/trace.py docs/brand/source/dancefitvn.png docs/brand/wordmark-plain.svg --no-reg

# iOS (gdesk.json icon, ios.iconDark, ios.iconTinted): 1024 px; the plain one has no alpha.
$R $SRC/ios-icon.svg        1024 1024 icon.png                   --opaque
$R $SRC/ios-icon-dark.svg   1024 1024 "$OUT/ios-icon-dark.png"   --alpha
$R $SRC/ios-icon-tinted.svg 1024 1024 "$OUT/ios-icon-tinted.png" --opaque

# Android (gdesk.json android.icon, android.splash.icon, android.notificationIcon): 432, 1152 and 96 px.
$R $SRC/android-fg.svg      432  432  "$OUT/android/ic_foreground.png" --alpha
$R $SRC/android-bg.svg      432  432  "$OUT/android/ic_background.png" --opaque
$R $SRC/android-mono.svg    432  432  "$OUT/android/ic_monochrome.png" --alpha
$R $SRC/android-splash.svg  1152 1152 "$OUT/android/splash.png"        --alpha
$R $SRC/notification.svg    96   96   assets/brand/notification.png    --alpha

# Google Play: the 512 icon (square, Play rounds it) and the 1024x500 feature graphic.
$R $SRC/ios-icon.svg        512  512  "$OUT/play/icon-512.png"         --alpha
$R $SRC/feature-graphic.svg 1024 500  "$OUT/play/feature-1024x500.png" --opaque

# tvOS (gdesk.json tvos.brandAssets): the App Icon's three layers at 400x240 @1x and @2x and the App Store's
# 1280x768, the back layer opaque; the Top Shelf images at 1x and 2x, opaque.
for layer in Back Middle Front; do
	src=$TV/icon-$(echo $layer | tr '[:upper:]' '[:lower:]').svg
	mode=--alpha
	[ $layer = Back ] && mode=--opaque
	set="$BA/App Icon.imagestack/$layer.imagestacklayer/Content.imageset"
	$R "$src" 400  240 "$set/$layer.png"    $mode
	$R "$src" 800  480 "$set/$layer@2x.png" $mode
	$R "$src" 1280 768 "$BA/App Icon - App Store.imagestack/$layer.imagestacklayer/Content.imageset/$layer.png" $mode
done
$R $TV/top-shelf.svg      1920 720  "$BA/Top Shelf Image.imageset/shelf.png"         --opaque
$R $TV/top-shelf.svg      3840 1440 "$BA/Top Shelf Image.imageset/shelf@2x.png"      --opaque
$R $TV/top-shelf-wide.svg 2320 720  "$BA/Top Shelf Image Wide.imageset/shelf.png"    --opaque
$R $TV/top-shelf-wide.svg 4640 1440 "$BA/Top Shelf Image Wide.imageset/shelf@2x.png" --opaque

go run ./cmd/brandcheck
