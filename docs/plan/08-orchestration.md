# 08 — Điều phối: công đoạn, agent, model

Dành cho **phiên chính của Claude Code (orchestrator)**. Orchestrator không tự viết mã sản phẩm: đọc bảng này, giao từng công đoạn cho đúng sub-agent `df-*` trong `.claude/agents/`, kiểm kết quả, cập nhật `docs/plan/PROGRESS.md`.

## 1. Agent và model

| Agent | Model | Effort | Làm gì | Ghi được |
|---|---|---|---|---|
| **orchestrator** (phiên chính) | `opus` | high | Đọc bảng, giao việc, kiểm tra, PROGRESS, BLOCKERS | `docs/plan/PROGRESS.md`, `docs/BLOCKERS.md`, `local/` |
| `df-architect` | `opus` | high | Kế hoạch chi tiết từng phase, đối chiếu API gDesk, không viết mã | `local/plans/` |
| `df-systems` | `opus` | high | Engine AVPlayer/MediaPlayer (chép từ jPlayer Mobile), objc/JNI, `internal/sys`, font, build/đóng gói, script | `internal/media/{avf,amp,enginekit}`, `internal/sys`, `internal/app`, `cmd/*spike`, `scripts/`, `gdesk.json` |
| `df-gdesk` | `opus` | high | Đề xuất G1–G7 (và làm khi chủ gDesk đồng ý) trong `../gdesk-dancefitvn-app` | `../gdesk-dancefitvn-app` (nhánh `dancefitvn-app-support`), mục "Đề xuất gDesk" của `docs/BLOCKERS.md` |
| `df-ui` | `sonnet` | high | Theme, icon, poster, kit, điều hướng, mọi màn mobile và TV | `internal/ui/`, `locales/` |
| `df-logic` | `sonnet` | medium | Lõi không UI: model, api, mock, auth, store, progress, download, pairing, format, qr, player, contractcheck | `internal/{model,api,auth,store,progress,download,pairing,format,qr,player}`, `cmd/contractcheck` |
| `df-tests` | `sonnet` | medium | Test từ đặc tả, random walk, benchmark, test simulator | `*_test.go`, `e2e/` |
| `df-matcher` | `sonnet` | medium | Chụp prototype và app, so layout/pixel, sửa hằng số lệch | hằng số trong `internal/ui/`, `local/shots/` |
| `df-reviewer` | `opus` | high | Duyệt độc lập, chỉ đọc | không ghi |
| `df-chores` | `haiku` | low | Việc cơ học: cấu trúc, sinh token, chuỗi, godoc, PROGRESS | tệp được chỉ định |
| `df-expert` | `fable` | high | **Chỉ khi leo thang** (§4) | theo việc được giao |

Khởi động: `claude --model opus --add-dir ../gdesk-dancefitvn-app`. Model của sub-agent lấy từ tệp agent; orchestrator không truyền `model` trừ khi leo thang.

## 2. Quy trình mỗi phase

1. Chọn các dòng `todo` của phase hiện tại có phụ thuộc đã `done` (PROGRESS).
2. Gọi `df-architect` cho dòng `Px.0` → `local/plans/Px.md` (kế hoạch, API gDesk dùng, rủi ro, câu hỏi).
3. **Cổng 1 (tự động):** orchestrator đọc kế hoạch; câu hỏi có mặc định ở PLAN §8 → dùng mặc định và ghi PROGRESS; câu hỏi chặn thật → BLOCKERS + dự phòng (§5).
4. Giao việc theo phụ thuộc; các dòng cùng nhóm song song và không chung tệp được gọi cùng lúc (tối đa 2).
5. Kiểm tra: `go test -count=1 ./...` và lệnh ở cột "Done khi". Lỗi → gọi lại cùng agent kèm lỗi (lần 2), rồi §4.
6. Phase có giao diện: `df-matcher` so các trạng thái của phase (07 §3.2) trên iPhone, Android (và Apple TV ở P7).
7. `df-reviewer` duyệt cả phase theo PLAN §5; FAIL → giao lại phần FAIL (tối đa 2 vòng), rồi §4.
8. **Cổng 2 (tự động):** reviewer PASS → `df-chores` ghi PROGRESS (phase `done`, ngày, số test, ảnh lệch còn lại, fallback đang dùng) → sang phase sau **không hỏi người dùng**.
9. Chạy tương tác thì gợi ý `/clear` giữa các phase; chạy tự động thì đi tiếp (PROGRESS là điểm nối lại).

Lệnh build/chạy chuẩn (ghi vào báo cáo):

```sh
gdesk build -os ios && xcrun simctl install booted dist/ios-simulator-arm64/DanceFITVN.app && xcrun simctl launch booted com.jivecode.dancefitvn
gdesk build -os android && adb install -r dist/android-arm64/DanceFITVN.apk && adb shell am start -n com.jivecode.dancefitvn/dev.gdesk.android.GDeskActivity
gdesk build -os tvos && xcrun simctl install "DanceFITVN Apple TV" dist/tvos-simulator-arm64/DanceFITVN.app && xcrun simctl launch "DanceFITVN Apple TV" com.jivecode.dancefitvn
```

## 3. Bảng công đoạn

Ký hiệu cột **Nhóm**: cùng chữ cái = chạy song song được.

### P0 — Khả thi & khung

| ID | Công đoạn | Agent | Phụ thuộc | Nhóm | Đầu vào | Done khi |
|---|---|---|---|---|---|---|
| P0.0 | Kế hoạch P0, xác nhận API gDesk theo 06 §1 | df-architect | — | — | PLAN, 04, 06, gDesk `kits/mobile`, `kits/cli`, `framework/ui/README.md` | `local/plans/P0.md` |
| P0.1 | Kiểm workspace (worktree, nhánh, `replace`; `$JPLAYER_MOBILE`, `$JPLAYER_TV` đọc được), cài `gdesk` từ worktree, múi giờ thật (`tz_*`), `.gitignore`, cây PLAN §4.3, `internal/archtest`, `main.go`, `gdesk.json` (ios, android, tvos; `localNetworking`, `cleartextHosts`) | df-chores | P0.0 | A | PLAN §4, 04 §5 | `go test ./...` xanh |
| P0.2 | App tối thiểu chạy iPhone simulator (393×852) — nền `--bg`, chữ "DANCEFIT" Barlow | df-systems | P0.1 | B | PLAN P0 việc 2 | ảnh `local/p0/ios.png` |
| P0.3 | App tối thiểu chạy emulator `dfv-412x892` (android-35) | df-systems | P0.2 | C | như trên | ảnh `local/p0/android.png` |
| P0.4 | App tối thiểu chạy simulator Apple TV, `ui.WithDirectionalFocus()`, 2 nút đổi focus bằng mũi tên | df-systems | P0.2 | C | `kits/mobile/README.md` tvOS | ảnh `local/p0/tv.png` |
| P0.5 | Font tiếng Việt (D5) trên Core Text và gotext | df-systems | P0.2 | C | D5 | `local/p0/fonts.md` + ảnh chuỗi "Đăng nhập huấn luyện viên · Khoá học · Đùi Mông" |
| P0.6 | Spike `cmd/avspike` iOS + tvOS (04 §5; tvOS dựa trên engine jPlayer TV + tua/tốc độ/lật) | df-systems | P0.4 | D | 04 §1, §5 | số đo trong `local/p0-results.md` |
| P0.7 | Spike `cmd/ampspike` Android (04 §5) | df-systems | P0.3 | D | 04 | số đo hoặc ghi R3 + F3 |
| P0.8 | Kiểm `secret` và `files` trên iOS/Android (06 §4) | df-systems | P0.3 | D | 06 | kết luận F1/F2 trong `local/p0-results.md` |
| P0.9 | Chụp prototype (07 §3.1), vectors, chuỗi | df-matcher | P0.0 | A | 07 §3 | `local/shots/prototype*/` đủ 43 ID, `local/vectors/`, `local/strings.json` |
| P0.10 | Duyệt P0, quyết định đi tiếp | df-reviewer | P0.2–P0.9 | — | PLAN P0 | PASS → P1; không khả thi → **dừng bắt buộc** (§5) |

### P1 — Lõi dữ liệu & mock

| ID | Công đoạn | Agent | Phụ thuộc | Nhóm | Đầu vào | Done khi |
|---|---|---|---|---|---|---|
| P1.0 | Kế hoạch P1 | df-architect | P0.10 | — | 05 | `local/plans/P1.md` |
| P1.1 | `model`, `format` (Clock, Size, ngày, Short) | df-logic | P1.0 | A | 05 §2, vectors | test vector xanh, `format.Clock` 0 alloc |
| P1.2 | `api` Client, `Transport`, `Error`, header, ETag, Live | df-logic | P1.1 | B | 05 §1, §3 | test xanh |
| P1.3 | `api/mock` 6 quy tắc + máy chủ media loopback | df-logic | P1.1 | B | 05 §4, 04 §5 | 24 route × kịch bản xanh |
| P1.4 | `internal/sys` secret (F1 nếu G1 chưa có), install id | df-systems | P1.0 | A | 05 §6, 06 G1 | test fake + chạy sim |
| P1.5 | `auth` (single-flight refresh), `store` (cache, prefs, lớp ghi đè mock) | df-logic | P1.2, P1.4 | C | 05 §3, §6 | test xanh |
| P1.6 | `progress`, `download`, `pairing` | df-logic | P1.5 | D | 03 §5, §7.4, 05 §6 | test xanh |
| P1.7 | `qr` | df-logic | P1.1 | C | 07 §2 | mã hoá/giải lại xanh |
| P1.8 | `cmd/contractcheck` + `-self` trong test | df-logic | P1.3 | D | 05 §5 | `go test ./cmd/contractcheck` xanh |
| P1.9 | Test bổ sung theo 07 §2 (refresh đồng thời, hàng chờ, Range) | df-tests | P1.6 | E | 07 §2 | xanh, `-race` xanh |
| P1.10 | Đề xuất G1, G3 (và G4) | df-gdesk | P1.0 | A | 06 | mục trong BLOCKERS |
| P1.11 | Duyệt P1 | df-reviewer | P1.1–P1.10 | — | PLAN P1 | PASS |

### P2 — Nền giao diện

| ID | Công đoạn | Agent | Phụ thuộc | Nhóm | Đầu vào | Done khi |
|---|---|---|---|---|---|---|
| P2.0 | Kế hoạch P2 | df-architect | P1.11 | — | 02, 03 §1 | `local/plans/P2.md` |
| P2.1 | Sinh `internal/tokens` từ `reference/design/tokens.json` (`go generate`), test màu = biến CSS | df-chores | P2.0 | A | D6, 02 §2 | test xanh |
| P2.2 | `theme` (mặt chữ theo PostScript name, D5), `icon` (bộ `IC`), chữ: giãn, chiều cao dòng, bóng, gradient tròn — chép từ jPlayer TV (06 §1b) | df-ui | P2.1 | B | 02 §2–§3, 06 §1b, `$JPLAYER_TV/internal/ui/{kit,theme}` | ảnh chữ và icon trong kitdemo khớp prototype |
| P2.3 | `poster` (công thức + bảng màu) | df-ui | P2.1 | B | 02 §5 | ảnh 4 cỡ (156×100, 353×232, 80×80, TV 540×304) khớp prototype |
| P2.4 | Kit A: Button, Chip, Input, Row, Switch, Segmented, Radio, Progress | df-ui | P2.2 | C | 02 §6 | test `ui.Tester` 2 máy |
| P2.5 | Kit B: Sheet + dim, Toast, TabBar, Top bar, Badge, Codebox, Avatar, LessonRow, CourseCard, OfflineBar | df-ui | P2.4 | D | 02 §6 | test 2 máy |
| P2.6 | Điều hướng mobile (stack, sheet, toast, Back/vuốt mép) | df-ui | P2.5 | E | 03 §1 | test |
| P2.7 | Trang `kitdemo` (`-tags kitdemo`) + so layout thành phần | df-matcher | P2.5 | E | 07 §3.3 | lệch ≤ 1 pt |
| P2.8 | Duyệt P2 | df-reviewer | P2.1–P2.7 | — | PLAN P2 | PASS |

### P3 — Mobile: vào app & duyệt khoá học

| ID | Công đoạn | Agent | Phụ thuộc | Nhóm | Đầu vào | Done khi |
|---|---|---|---|---|---|---|
| P3.0 | Kế hoạch P3 | df-architect | P2.8 | — | 03, 07 §3.2 | `local/plans/P3.md` |
| P3.1 | `scripts/shot.sh` + dựng trạng thái `DFV_STATE` (build dev) + dump `layout.json` | df-systems | P3.0 | A | 07 §3.3 | chụp được M01 trên 2 máy |
| P3.2 | Màn 01, 02, 02B, 03B (đăng nhập + mọi lỗi `auth.login`), nút sinh trắc học (K13) | df-ui | P3.0 | A | 03 §1, §3 `bioLogin`, K6, K13 | test |
| P3.2b | Sinh trắc học F6 (`internal/sys/biometric_*`) + lưu refresh token sau sinh trắc học | df-systems | P3.0 | A | 06 G3, K13 | Face ID trên iPhone sim (Features → Face ID), vân tay trên emulator |
| P3.3 | Màn 03 Trang chủ | df-ui | P3.2 | B | | test |
| P3.4 | Màn 04 Khoá học (lọc, tìm, rỗng) | df-ui | P3.2 | B | 03 §4 | test |
| P3.5 | Màn 05, 05B | df-ui | P3.3 | C | | test |
| P3.6 | Test luồng (đăng nhập → trang chủ → khoá học → chi tiết, ngoại tuyến) | df-tests | P3.5 | D | 07 §2 | xanh |
| P3.7 | So ảnh M01–M05B | df-matcher | P3.1, P3.5 | D | 07 §3 | đạt ngưỡng |
| P3.8 | Duyệt P3 | df-reviewer | P3.1–P3.7 | — | PLAN P3 | PASS |

### P4 — Mobile: phát bài

| ID | Công đoạn | Agent | Phụ thuộc | Nhóm | Đầu vào | Done khi |
|---|---|---|---|---|---|---|
| P4.0 | Kế hoạch P4 | df-architect | P3.8 | — | 04, 03 §5 | `local/plans/P4.md` |
| P4.1 | Chép engine `avf` iOS (jPlayer Mobile) và `avftv` (jPlayer TV + tua/tốc độ/lật) | df-systems | P4.0 | A | 04 §1–§2 | phát HLS + MP3 trên sim iOS, tvOS |
| P4.2 | Chép engine `amp` (Android) | df-systems | P4.0 | A | 04 §1, §3 | phát trên emulator |
| P4.3 | `player` (trạng thái phát, A–B, token, lưu tiến độ) | df-logic | P4.0 | A | 04 §4, 03 §5 | test với `media/fake` |
| P4.4 | Màn 06 Video (tự ẩn 03 §5a) + sheet 07 | df-ui | P4.3 | B | 03 §3, §5, §5a | test |
| P4.5 | Màn 08 Nghe, 10 Hết bài | df-ui | P4.3 | B | | test |
| P4.6 | Now Playing, nghe nền, `AVAudioSession`, dịch vụ media Android | df-systems | P4.1, P4.2 | C | 04 §2.4–§3.4 | nghe khi khoá màn hình 2 hệ |
| P4.7 | Test + benchmark khung video | df-tests | P4.4, P4.5 | D | 07 §2 | xanh, 0 alloc |
| P4.8 | So ảnh M06–M10 (phần P4) | df-matcher | P4.4, P4.5 | D | 07 §3 | đạt |
| P4.9 | Duyệt P4 | df-reviewer | P4.1–P4.8 | — | PLAN P4 | PASS |

### P5 — Mobile: tải xuống & ngoại tuyến

| ID | Công đoạn | Agent | Phụ thuộc | Nhóm | Đầu vào | Done khi |
|---|---|---|---|---|---|---|
| P5.0 | Kế hoạch P5 | df-architect | P4.9 | — | 03 §2, 05 §6, 06 G4/G5 | `local/plans/P5.md` |
| P5.1 | `internal/sys`: FreeSpace, mạng (F4 hoặc G4), thư mục tải | df-systems | P5.0 | A | 06 | test sim |
| P5.2 | Sheet 09, màn 11, 12, sheet 14, trạng thái nút tải | df-ui | P5.0 | A | 03 §3 | test |
| P5.3 | Màn 13 + thanh ngoại tuyến + quy tắc 03 §2 | df-ui | P5.1, P5.2 | B | 03 §2 | test |
| P5.4 | Đề xuất G4, G5 | df-gdesk | P5.0 | A | 06 | BLOCKERS |
| P5.5 | Test (máy bay, nối tiếp Range, sha256) | df-tests | P5.3 | C | 07 §2 | xanh |
| P5.6 | So ảnh M09, M11–M14 | df-matcher | P5.3 | C | 07 §3 | đạt |
| P5.7 | Duyệt P5 | df-reviewer | P5.1–P5.6 | — | PLAN P5 | PASS |

### P6 — Mobile: tài khoản & kết nối TV

| ID | Công đoạn | Agent | Phụ thuộc | Nhóm | Đầu vào | Done khi |
|---|---|---|---|---|---|---|
| P6.0 | Kế hoạch P6 | df-architect | P5.7 | — | 03 §3, §6 | `local/plans/P6.md` |
| P6.1 | Màn 15, 15B, sheet đổi mật khẩu, sheet đăng xuất | df-ui | P6.0 | A | | test |
| P6.2 | Màn 15C Hồ sơ HLV | df-ui | P6.1 | B | | test |
| P6.3 | Chọn ảnh (F2 hoặc G2) + sheet 15D cắt ảnh 512×512 | df-systems | P6.0 | A | 03 §6, 06 G2 | chọn ảnh trên 2 sim |
| P6.4 | Màn 16 Kết nối Apple TV + sheet chọn máy gỡ (`device_limit`) | df-ui | P6.1 | B | 03 §3, §9 | test |
| P6.5 | Đề xuất G2 | df-gdesk | P6.0 | A | 06 | BLOCKERS |
| P6.6 | Test | df-tests | P6.2–P6.4 | C | 07 §2 | xanh |
| P6.7 | So ảnh M15–M16-linked, M-toast | df-matcher | P6.2–P6.4 | C | 07 §3 | đạt |
| P6.8 | Duyệt P6 | df-reviewer | P6.1–P6.7 | — | PLAN P6 | PASS |

### P7 — Apple TV

| ID | Công đoạn | Agent | Phụ thuộc | Nhóm | Đầu vào | Done khi |
|---|---|---|---|---|---|---|
| P7.0 | Kế hoạch P7 | df-architect | P6.8 | — | 03 §7, 02 (TV) | `local/plans/P7.md` |
| P7.1 | Khung TV (Metal + nền trong suốt), nav, lớp `focus` chép từ jPlayer TV (D8), toast, phím | df-ui | P7.0 | A | 03 §7.1–§7.2, `$JPLAYER_TV/internal/ui/focus` | test |
| P7.2 | TV01, TV02 (QR, poll, hết hạn) | df-ui | P7.1 | B | 03 §7.4, K1, K8 | test |
| P7.3 | TV04, TV05, TV06 | df-ui | P7.1 | B | | test |
| P7.4 | TV07, TV08, TV09, TV10, TV11, TV12 | df-ui | P7.3 | C | 03 §7.3 | test |
| P7.5 | Build tvOS: `tvos` trong gdesk.json, brand assets tạm (placeholder của gDesk `kits/cli/testdata/tvos`), KeepAwake | df-systems | P7.0 | A | `kits/cli/README.md` | build sim chạy |
| P7.6 | Random walk 600 phím, vector focus, `local/focus-diff.md` | df-tests | P7.4 | D | 07 §2 | xanh; diff ghi K9 |
| P7.7 | So ảnh TV01–TV12, TV-toast | df-matcher | P7.4, P7.5 | D | 07 §3 | đạt |
| P7.8 | Đề xuất G6 nếu `focus-diff` có màn đi sai | df-gdesk | P7.6 | E | 06 G6 | BLOCKERS hoặc "không cần" |
| P7.9 | Duyệt P7 | df-reviewer | P7.1–P7.8 | — | PLAN P7 | PASS |

### P8 — Hoàn thiện & phát hành thử

| ID | Công đoạn | Agent | Phụ thuộc | Nhóm | Đầu vào | Done khi |
|---|---|---|---|---|---|---|
| P8.0 | Kế hoạch P8 | df-architect | P7.9 | — | PLAN P8, 07 §5–§6 | `local/plans/P8.md` |
| P8.1a | API: base live, `CheckLiveRoutes`, contractcheck qua HTTP (mockserve) | df-logic | P8.0 | A | 05 §3, §5 | `go test` xanh; `contractcheck -self` PASS |
| P8.1b | App: wire live (bản phát hành) + hybrid (bản dev), log chế độ | df-systems | P8.1a, P7.9 | B | 05 §4, `internal/app` | smoke sim qua mockserve; BLOCKERS máy chủ thật |
| P8.2a | Manifest quyền riêng tư iOS/tvOS, `gdesk.json`, `iosscan.py` | df-systems | P8.0 | C | `PrivacyInfo`, Apple required reasons | scan sạch; test `internal/release` |
| P8.2b | Build phát hành, `scripts/release.sh`, check-ios/android, `doctor` | df-systems | P8.2a | C | `gdesk build -device -release`, `gdesk package` | artefact + SHA256SUMS; ký → BLOCKERS |
| P8.2c | Nháp nội dung store, `docs/store`, `docs/release.md` | df-chores | P8.2b | C | §2 của P8 plan | nháp với TBD; BLOCKERS |
| P8.3a | Trợ năng mobile: `Hush`/`Decor`, nhãn, thứ tự đọc, video 06 | df-ui | P8.0 | A | 02, 03, `kit`, `mobile/*` | test `ui.Tester` 2 máy; ảnh không đổi |
| P8.3b | Trợ năng TV: nhãn tile/nav/cài đặt, panel TV08, toast, `ChangeScreen` | df-ui | P8.0 | B | `tv/*` | test TV 1080p; ảnh không đổi |
| P8.3c | Đề xuất G10 (cầu TalkBack Android) | df-gdesk | P8.0 | A | `kits/mobile/README.md`, jPlayer GM10 | BLOCKERS (đề xuất gDesk) |
| P8.3d | Dump trợ năng trên sim (`a11y-dump.sh`), VoiceOver | df-systems | P8.3a, P8.3b | C | `a11ydump*`, `devaccess.go` | `local/p8/a11y/` PASS |
| P8.4a | Điểm đo và kịch bản đo (`perf-*.sh`, mốc `home`, PERF-SCROLL, PERF-VIDEO10) | df-systems | P8.1b | D | 07 §5 | script chạy trên sim |
| P8.4b | Đo trên iPhone sim, Apple TV 4K sim, emulator (fallback) | df-tests | P8.4a, P8.3 | D | 07 §5 | `local/p8/perf/*.md` |
| P8.4c | Sweep điện thoại Android (chỉ khi kết nối, cài/test) | df-tests | P8.4b | D | 07 §6 | bằng chứng hoặc BLOCKERS |
| P8.5 | README (tiếng Anh), PROGRESS, BLOCKERS, sửa tài liệu §12, dọn tệp thừa | df-chores | P8.4 | E | | xong |
| P8.6 | Duyệt cuối (cả 9 phase, compliance scan) | df-reviewer | P8.1–P8.5 | — | PLAN | PASS |

## 4. Leo thang

1. Cùng agent, lần 2, kèm lỗi và đoạn đặc tả liên quan.
2. Lần 3: cùng việc giao `df-expert` (model `fable`), kèm lịch sử hai lần trước.
3. `df-expert` thất bại: ghi `docs/BLOCKERS.md` (việc, lỗi, đã thử gì), đánh dấu dòng `blocked`, làm tiếp các dòng không phụ thuộc. Không lặp vô hạn.

## 5. Chế độ tự động (mặc định)

Khi người dùng gửi "prompt một lần" của `docs/plan/README.vi.md`, orchestrator chạy **từ phase đang dở tới hết P8** không chờ người dùng:

- Mọi cổng duyệt được thay bằng: `df-reviewer` PASS + ghi PROGRESS. Câu hỏi mở dùng mặc định ở PLAN §8 và ghi "đã dùng mặc định Qx" vào PROGRESS.
- **Chỉ dừng bắt buộc khi:**
  1. P0.10 kết luận không khả thi (gDesk không build/chạy được một trong ba hệ, hoặc không phát được HLS mà không có dự phòng).
  2. Việc cần tài khoản, chứng chỉ, thiết bị thật, thanh toán, quyết định pháp lý/bản quyền, hoặc **sự đồng ý của chủ gDesk** cho một thay đổi gDesk → ghi `docs/BLOCKERS.md`, dùng dự phòng F đã ghi (06 §2), **làm tiếp** các việc độc lập. Chỉ dừng khi không còn việc nào làm được.
  3. Hành động không đảo ngược hoặc phá huỷ (xoá dữ liệu ngoài `local/` và `dist/`, ghi đè worktree khác, đổi nhánh).
  4. `git commit`, push, merge, tag — không bao giờ tự làm.
- `docs/plan/PROGRESS.md` là điểm nối lại: hết phiên hoặc hết ngữ cảnh, gửi lại đúng prompt, orchestrator đọc PROGRESS và đi tiếp từ dòng chưa `done`.
- Cuối cùng báo cáo: phase đã xong, phần còn dở, blockers, fallback đang dùng, lệnh để người dùng tự chạy trên simulator/máy thật.

## 6. Mẫu gọi sub-agent

```
Việc: <ID> — <công đoạn>
Đọc trước: <đầu vào ở bảng> + CLAUDE.md §4–§5
Phạm vi ghi: <cột "Ghi được" của agent>
Xong khi: <cột Done khi>
Báo lại: tệp đã đổi, lệnh test đã chạy và kết quả, chỗ lệch prototype còn lại, câu hỏi
```

## 7. Song song

- Tối đa 2 agent cùng lúc; không giao hai dòng chạm cùng tệp/gói.
- `df-systems` (engine, sim) và `df-logic` (core) thường song song được; `df-ui` chờ kit (P2) xong.
- Một simulator/emulator chỉ cho một agent tại một thời điểm (tên thiết bị trong lời giao việc).
