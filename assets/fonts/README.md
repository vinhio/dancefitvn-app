# Fonts

The faces DanceFITVN draws in (PLAN D5), embedded by `assets.Fonts` (`assets/assets.go`) and
registered at launch by `internal/app/fonts.go`: written once to the user's cache folder, then given to
`auto.RegisterFontFile` (Core Text on iOS and tvOS, gotext on Android), Inter first so that gotext's
default family is the body font. `internal/ui/theme/faces.go` names them: Core Text by PostScript name
at `text.WeightRegular` (its named families collapse every weight from 600 up to the bold trait),
gotext by typographic family and weight.

All are licensed under the SIL Open Font License 1.1; the licence text of each family is the
`OFL-<Family>.txt` file beside it.

- Sources: https://github.com/google/fonts, branch `main`, folder `ofl/`, fetched 2026-11-14 by
  `scripts/fonts/fetch.sh` into `local/fonts/source/` (not committed, Q-P0-8), which checks the sha256
  below.
- `scripts/fonts/make-static-fonts.sh` cuts the static faces into `local/fonts/static/`: Inter at
  wght 400, 500, 600, 700, 800 and opsz 14 (the variable font's default, and what the prototype's
  Google Fonts CSS serves: a woff2 with a `wght` axis alone, whose advances equal the opsz 14 cuts';
  `local/p0/fonts.md`), JetBrains Mono at wght 500 and 700, each renamed so that its PostScript name
  is its file's stem and its typographic family and style are `Inter` / `JetBrains Mono` and the
  style word (`static_fonts.py`); Barlow Condensed Bold and ExtraBold are upstream's static files,
  unchanged (600 is loaded by the prototype but never drawn, Q-P0-7).
- The `*.ttf` files here are those faces subset to Latin and Vietnamese by `scripts/fonts/subset.sh`
  (the characters of `scripts/fonts/unicodes.txt`, written by `scripts/fonts/charset.py` from
  `locales/vi.json`, the prototype's strings and fixed ranges), every layout feature and name kept.
  Rerun `scripts/fonts/subset.sh --charset` when `locales/vi.json` gains a character.
- `SHA256SUMS.txt` lists the sha256 of the embedded fonts: the app names its cached copies by them
  instead of hashing the fonts as it starts.

| Source (`local/fonts/source/`, not embedded) | URL | sha256 | Bytes |
|---|---|---|---|
| `BarlowCondensed-Bold.ttf` | https://raw.githubusercontent.com/google/fonts/main/ofl/barlowcondensed/BarlowCondensed-Bold.ttf | `e476562ec9c1e16cf16475895b511f08c804f438cc9a9f80a44ea50a0eeb5b65` | 109912 |
| `BarlowCondensed-ExtraBold.ttf` | https://raw.githubusercontent.com/google/fonts/main/ofl/barlowcondensed/BarlowCondensed-ExtraBold.ttf | `724c9c25952d5f4a2d87185d9767aa006144c5f0d944dc05bf7d5d603551c260` | 110296 |
| `OFL-BarlowCondensed.txt` | https://raw.githubusercontent.com/google/fonts/main/ofl/barlowcondensed/OFL.txt | `186d750eb496a4c17a76385f82be6aea2ac1cf2de074a811d63786cf374ea73f` | 4377 |
| `Inter[opsz,wght].ttf` | https://raw.githubusercontent.com/google/fonts/main/ofl/inter/Inter%5Bopsz,wght%5D.ttf | `29160a80ff49ddcab2c97711247e08b1fab27a484a329ce8b813d820dc559031` | 876576 |
| `OFL-Inter.txt` | https://raw.githubusercontent.com/google/fonts/main/ofl/inter/OFL.txt | `5b9321a4298cfeb6b34354164a1c3afc3db114569984c502b9b35d988fd58c57` | 4377 |
| `JetBrainsMono[wght].ttf` | https://raw.githubusercontent.com/google/fonts/main/ofl/jetbrainsmono/JetBrainsMono%5Bwght%5D.ttf | `48715a42ec242c21e9f02692891e147d022299a52e48d5e413e1a942193ffeda` | 187208 |
| `OFL-JetBrainsMono.txt` | https://raw.githubusercontent.com/google/fonts/main/ofl/jetbrainsmono/OFL.txt | `b2fe5e8987594e9ffd1d2ca52a2f5d73eb8335243893c5d6254b5ad69269591d` | 4399 |

| Face | Bytes, full | sha256, full (`local/fonts/static/`) | Bytes, subset | sha256, subset (embedded) |
|---|---|---|---|---|
| `BarlowCondensed-Bold.ttf` | 109912 | `e476562ec9c1e16cf16475895b511f08c804f438cc9a9f80a44ea50a0eeb5b65` | 70852 | `1073288adc50f53836403a49ec5c44ebec99e74c31333b0d5f63f1c654a97be6` |
| `BarlowCondensed-ExtraBold.ttf` | 110296 | `724c9c25952d5f4a2d87185d9767aa006144c5f0d944dc05bf7d5d603551c260` | 71464 | `42aa4f08b4acc1098f57fef9c62942d265972039dca76adb44c2fec7320e7a68` |
| `Inter-Bold.ttf` | 343072 | `2fb12f8177863403562a5fd38b8a0a4b6d557cabbb380be88e3fe2b73ee05231` | 114644 | `afab92cc9f40977e7c57865a3926dad10f4e63cbdbb371becd4655c474acba36` |
| `Inter-ExtraBold.ttf` | 343976 | `325490554f4b960206a63f5c8fe506cb47462b1d9f8a9c68106542e983919e3e` | 115004 | `65b9bb8fcf26a0d8ea01c40a71bd3bffb57b801be9e1f46cadab749678893d33` |
| `Inter-Medium.ttf` | 341972 | `954a7a85b466221d1e7f47e5d86ccac6e1206e6a8bd1a9d3c65d489ffc6526d7` | 114460 | `2f98f67ad2781484d9f9b5361727b01ff827a318b4f0c7be593764b00f7b65dd` |
| `Inter-Regular.ttf` | 341384 | `06a495a414879dd32f8d3ed30d80596365fcc94046441f600a6a5e108694ee30` | 114424 | `5903515b1ee14de60a916e8a94501d7ee5911cf31668ab24bb29f3a8c29f0f26` |
| `Inter-SemiBold.ttf` | 342664 | `74728409d92f47fe5e758517385ef7b18a4a1263693a5a7f068ed71cc9790fcb` | 114668 | `0dfd6505aac572a3d896077ef6495daa650bf63d657136056c54b95b84dc0e3c` |
| `JetBrainsMono-Bold.ttf` | 115104 | `fd8890de0ad7ccaf70fdfb197d8be4cd3f0de057bb3dd796e5f122c604ec2c85` | 62940 | `d2046783521058f468d9bdd62e2d38d0fa04988579ee3ba32c5fee3a74feb6ae` |
| `JetBrainsMono-Medium.ttf` | 115188 | `495fa506f5aa71907f52265f0645b7ae0b50fa56f934e1c8c3894ce8471be17b` | 62996 | `fe65f889f6b2e072e36e249f61dcc14fb0356f97ec48cf7c67a438f8585e01b2` |
| total | 2163568 | | 841452 | |

| Licence | sha256 |
|---|---|
| `OFL-BarlowCondensed.txt` | `186d750eb496a4c17a76385f82be6aea2ac1cf2de074a811d63786cf374ea73f` |
| `OFL-Inter.txt` | `5b9321a4298cfeb6b34354164a1c3afc3db114569984c502b9b35d988fd58c57` |
| `OFL-JetBrainsMono.txt` | `b2fe5e8987594e9ffd1d2ca52a2f5d73eb8335243893c5d6254b5ad69269591d` |
