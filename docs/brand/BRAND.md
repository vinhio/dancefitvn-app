<!-- Adapted from jplayer-mobile/docs/brand/BRAND.md at 49de3c2. -->
# DanceFITVN app brand

The app's brand is DanceFITVN's own logo, as dancefitvn.com shows it, laid out for the pictures iOS, Android, tvOS and
the stores ask for, on the Night Stage palette of the prototype (`reference/design/tokens.json`). The owner asked for
this (2026-11-17); the logo may be used for the app's icon and store pictures. Course videos, music and photos stay out
of the repository (`docs/plan/05-data.md` section 7).

## 1. Sources and outputs

`docs/brand/source/` holds verbatim copies of the site's files, never edited (`SHA256SUMS` guards them): from
`dancefitvn/public/assets/` at commit `428ec34`, `logo/dancefitvn.png` (the logo: mark and black wordmark),
`logo/dancefitvn-white.png` (white wordmark) and `favicon/apple-touch-icon.png` (the site's square mark).

SVG sources are in `docs/brand/`; `docs/brand/generate.sh` traces the wordmark (`scripts/brand/trace.py`), renders every
picture with Playwright's Chromium (`scripts/brand/render.py`) and runs `go run ./cmd/brandcheck`. Both scripts use the
venv `local/.venv` (Playwright with Chromium, as `docs/plan/07-testing.md` section 3.1 makes it, plus Pillow:
`local/.venv/bin/pip install pillow`). The pictures are committed, so a build needs neither Chromium nor Python. The
same sources give the same bytes: `render.py` re-encodes each picture with fixed settings, the tracer is deterministic.

| Source | Output | Size | Goes to |
|---|---|---|---|
| `mark.svg` | (drawn into every picture below) | | |
| `source/dancefitvn.png` | `wordmark.svg`, `wordmark-plain.svg` (no ®) | vector | feature graphic, Top Shelf |
| `mobile/ios-icon.svg` | `icon.png` (repository root), RGB, no alpha | 1024 | `gdesk.json` `icon` |
| `mobile/ios-icon-dark.svg` | `mobile/out/ios-icon-dark.png`, transparent ground | 1024 | `ios.iconDark` |
| `mobile/ios-icon-tinted.svg` | `mobile/out/ios-icon-tinted.png`, greyscale, black ground | 1024 | `ios.iconTinted` |
| `mobile/android-fg.svg` | `mobile/out/android/ic_foreground.png` | 432 (108 dp) | `android.icon.foreground` |
| `mobile/android-bg.svg` | `mobile/out/android/ic_background.png`, opaque | 432 (108 dp) | `android.icon.background` |
| `mobile/android-mono.svg` | `mobile/out/android/ic_monochrome.png`, alpha only | 432 (108 dp) | `android.icon.monochrome` |
| `mobile/android-splash.svg` | `mobile/out/android/splash.png` | 1152 (288 dp) | `android.splash.icon` |
| `mobile/notification.svg` | `assets/brand/notification.png`, white | 96 (24 dp) | `android.notificationIcon` |
| `mobile/ios-icon.svg` | `mobile/out/play/icon-512.png` | 512 | Play Console |
| `mobile/feature-graphic.svg` | `mobile/out/play/feature-1024x500.png`, RGB | 1024x500 | Play Console |
| `tv/icon-{back,middle,front}.svg` | `App Icon.imagestack` layers, Back RGB | 400x240 @1x, @2x | `tvos.brandAssets` |
| `tv/icon-{back,middle,front}.svg` | `App Icon - App Store.imagestack` layers, Back RGB | 1280x768 | `tvos.brandAssets` |
| `tv/top-shelf.svg` | `Top Shelf Image.imageset/shelf{,@2x}.png`, RGB | 1920x720 @1x, @2x | `tvos.brandAssets` |
| `tv/top-shelf-wide.svg` | `Top Shelf Image Wide.imageset/shelf{,@2x}.png`, RGB | 2320x720 @1x, @2x | `tvos.brandAssets` |
| `mobile/store-frame.html` | store screenshots (made by hand from raw shots) | per store | App Store Connect, Play Console |

The tvOS outputs are under `assets/tvos/DanceFITVN.brandassets/`. Also in `gdesk.json`: `ios.launchBackground`,
`tvos.launchBackground` and `android.splash.background` are the Night Stage ground, `#141518`.

## 2. The mark and the wordmark

- **Mark** (`mark.svg`): a vector redraw of the pill mark of the site logo, fitted to the logo's pixels (no vector
  original was available). It is two equal pills and a circle, point-symmetric about the circle's centre: pills of
  radius 47.2 (source pixels) with axes 47.45 degrees above the horizontal, 24.4 either side of the centre; the circle
  of radius 72.7. Yellow `#FFDE59` at the back, red `#FF3232`, cyan `#74E5E9` in front; where cyan covers red it is
  `#74C7CB`, the site's own color there. Each pill's hidden end lies inside the circle. The colors are the logo's
  pixels exactly; the redraw differs from the logo on 0.11 % of its fully colored pixels, all at anti-aliased edges
  (`local/brand/evidence/mark-compare.png`). The site art's uneven white halo around the mark (visible only on dark
  grounds, e.g. the 2026-kinetic theme) is left out.
- **Wordmark** (`wordmark.svg`): "DanceFIT" and the registered sign, traced from the logo's black wordmark
  (marching squares at 4x, 50 % contour, Douglas-Peucker 0.15 px; the logo's drop shadow removed) and filled white, as
  the site's white logo. It is not re-typeset in another face. `wordmark-plain.svg` is the same without the ®.
- The in-app logo (the prototype's "DANCEFIT" in Barlow Condensed) is not changed by this brand.

## 3. Variants

- **iOS icon**: the mark on the Night Stage ground `#141518`, a full opaque square (iOS applies its own mask). The
  mark is 340 of 512 wide, the width of jPlayer's record; it reads at 60 and 29 px.
- **iOS dark / tinted** (iOS 18): the same mark on a transparent ground; the tinted one is greyscale on black, each
  color its Rec. 709 luminance (yellow 219, red 94, cyan 205, cyan over red 182), which the system tints.
- **Android adaptive icon** (108 dp layers, 72 dp shown, 66 dp safe circle): the background is the ground; the
  foreground is the mark as the iOS icon places it in the 72 dp (47.8 dp wide, at most 26.9 dp from the centre); the
  monochrome layer is the mark as one shape, the circle cut from the yellow pill and the cyan pill from both by a
  1.46 dp gap.
- **Android splash** (Android 12+): the mark, at most 90 dp from the centre, inside the 96 dp circle of the 288 dp icon,
  over the ground.
- **Notification icon**: the monochrome shape in white, 19.5 dp wide, gaps of 0.89 dp, inside the 20 dp live area.
- **iOS launch screen**: the plain ground `#141518`, no logo.
- **Feature graphic**: the ground of the prototype's splash (`#141518` and its red glow, `MS.splash`,
  `reference/prototype/index.html:328`), the mark, the wordmark, the splash's motto "Dễ · Vui · Khoẻ" (upper case, as
  the prototype's `.eb` style, in the accent `#FFD028`) and its tagline "Ứng dụng học tập dành cho huấn luyện viên"
  (`locales/vi.json` `splash.motto`, `splash.tagline`) in Inter. It names no other platform.

## 4. tvOS

`gdesk.json` `tvos.brandAssets` names `assets/tvos/DanceFITVN.brandassets`, which `gdesk build -os tvos` compiles with
actool (gDesk `kits/cli/config.go`, `IOS.BrandAssets`). Sizes are those actool of Xcode 26.6 takes for tvOS 17 and
jPlayer TV ships: the App Icon a stack of 400x240 pt at 1x and 2x, the App Store icon a stack of 1280x768 at 1x, the
Top Shelf image 1920x720 and the wide one 2320x720 at 1x and 2x.

- **App Icon** (three layers): Back, opaque, the ground; Middle, the splash's red glow on transparency; Front, the whole
  mark (144 of 240 pt high). The mark is not split across layers: cyan over red has its own color, which parallax
  would shift off the circle. The front layer stays inside the middle 80 % of each side (a margin chosen as jPlayer TV's,
  not Apple's figure).
- **Top Shelf** (static, no extension): the feature graphic's lockup (mark, wordmark, motto) centred on the splash
  ground; nothing in the bottom 160 pt, where the Home screen's row of apps lies over it on tvOS 27.
- Seen on the tvOS 27 simulator: the layered icon in the app grid and, with the app in the top row, the Top Shelf
  (`local/brand/evidence/home-focused.png`, `shelf-topshelf.png`). Apple TV hardware: not checked (BLOCKERS U17).

## 5. Store screenshots

`mobile/store-frame.html` puts a raw simulator or emulator shot (mock data only) on the splash ground with a Vietnamese
title in Barlow Condensed ExtraBold, upper case. Sizes `cmd/brandcheck -shots DIR` accepts: iPhone 6.9" 1320x2868,
iPad 13" 2064x2752, Android phone 1080x1920, Android tablet 2560x1600, Apple TV 1920x1080 and 3840x2160.

## 6. Rules

- Keep the mark's colors, overlap and proportions: no recoloring, no stretching, no extra shadows or outlines. The one
  added treatment is the gap cut of the one-color shapes (monochrome, notification), which keeps the three parts
  readable without color.
- No alpha where a store or `gdesk build` refuses it: `icon.png`, `ic_background.png`, `ios-icon-tinted.png`,
  `feature-1024x500.png`, the tvOS back layers and Top Shelf images, every store shot. `cmd/brandcheck` checks sizes,
  alpha, the safe circles and boxes, the tvOS catalog (roles, sizes, scales, layer count, no stray picture), the
  `gdesk.json` wiring and the source hashes (`go run ./cmd/brandcheck [-shots DIR]`).
- Fonts are those of `assets/fonts` (OFL): the feature graphic and the Top Shelf load Inter by file, the store frame
  Barlow Condensed ExtraBold.

## 7. Owner decisions (BLOCKERS D1, approved 2026-11-17: all kept as below)

1. The motto and tagline of the feature graphic and Top Shelf (taken from the prototype's splash).
2. Fidelity of the redrawn mark and the traced wordmark; a vector original from the designer would replace both.
3. Whether the ® appears (now: yes, as in the site logo; `wordmark-plain.svg` is ready).
4. Whether the white halo of the site art belongs to the mark on dark grounds (now: left out).
