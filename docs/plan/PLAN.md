# DanceFITVN App — Kế hoạch tổng

Ứng dụng học tập cho huấn luyện viên DanceFITVN trên **iPhone, Android và Apple TV**, viết bằng Go trên **gDesk**, trong một codebase `dancefitvn-app`. Đặc tả chuẩn là prototype HTML Night Stage ở `reference/prototype/index.html`. Backend gFly chưa có: app chạy trên bộ JSON mock `mock/dancefitvn-mock-api/` qua một lớp transport, rồi chuyển sang server thật mà không sửa giao diện.

## 1. Mục tiêu

1. App Mobile (iOS + Android, **một** giao diện, một hành vi) và app Apple TV khớp prototype 100% về bố cục, kích thước, màu, chữ, hiệu ứng, thao tác và chuỗi hiển thị, trừ các khác biệt bắt buộc K1–K16 (01 §3).
2. Toàn bộ dữ liệu đi qua `internal/api` với 3 chế độ `mock` / `live` / `hybrid` (D3). Mock đọc `mock/dancefitvn-mock-api/routes.json`, đổi kịch bản lỗi theo từng route.
3. Phát video HLS dọc 9:16 với tốc độ 0.5–1.5x, lật gương, lặp A–B, thanh đếm 8 nhịp; phát file âm thanh nền với Now Playing; tải file âm thanh để dùng ngoại tuyến (chỉ Mobile).
4. Apple TV điều khiển bằng Siri Remote (focus theo hướng), đăng nhập bằng mã 6 ký tự ghép từ điện thoại.
5. Có công cụ `cmd/contractcheck` để đội backend gFly kiểm server thật so với mock.

## 2. Ngoài phạm vi bản 1.0

- Giao diện sáng (bảng thiết kế màn 17): prototype không có → không làm (Q1).
- Màn TV03 "Đang tải" của bảng thiết kế: prototype không có → TV dùng trạng thái rỗng của từng màn (K12).
- Mua khoá học trong app, đăng ký tài khoản trong app (chỉ mở website).
- Android TV, iPad bố cục riêng (iPad chạy bản iPhone phóng to theo gDesk).
- DRM (FairPlay/Widevine), CDN, thông báo đẩy.
- Server gFly (đội backend làm theo tài liệu API v1; app chỉ cung cấp `contractcheck`).

## 3. Quyết định

| ID | Quyết định | Lý do |
|---|---|---|
| D1 | Một repo, một module Go `dancefitvn-app`, ba đích `gdesk build -os ios|android|tvos` chung một `gdesk.json`. Giao diện TV chọn lúc biên dịch bằng build tag `tvos` (`internal/ui/tv` vs `internal/ui/mobile`); lõi dùng chung. | gDesk build tvOS là `GOOS=ios` + `-tags tvos` (`kits/cli/README.md`, bảng build, dòng tvOS); hai app chung API, model, token, chuỗi. |
| D2 | Prototype thắng tài liệu. Mobile vẽ đúng một kiểu trên cả iPhone (393×852) và Android (412×892); không rẽ giao diện theo hệ điều hành. | Yêu cầu "một giao diện, hành vi giống nhau hoàn toàn". |
| D3 | `api.Transport` có 3 bản: `MockTransport` (mặc định, build không tag), `LiveTransport` (`-tags live`, `api.DefaultBase` = `https://dancefitvn.com/mobile/api/v1`, không mock/overlay/kịch bản), `HybridTransport` (chỉ bản dev: `DFV_API` + `DFV_LIVE_ROUTES`, qua `api.CheckLiveRoutes`). Không dùng `config.json` cạnh app: sim/máy không giữ tệp đó (Q-P8-3). | Backend chưa có; đổi sang server thật không sửa UI. |
| D4 | Engine phát của hệ điều hành, chép và chỉnh từ jPlayer (cùng chủ sở hữu, không import chéo repo; 04 §1). **Mobile**: AVPlayer (iOS) và MediaPlayer qua JNI (Android) như `jplayer-mobile/internal/media/{avf,amp}` — khung video vào `media.Surface`, lật gương = vẽ lật. **TV**: AVPlayer như `jplayer-tv/internal/media/avf` — mặt phẳng native `AVPlayerLayer` dưới canvas (`platform.UnderlayCanvas`, phương án B của jPlayer TV) + giao diện vẽ bằng Metal (`ui.WithGPU`, nền trong suốt); thêm tua, tốc độ (lấy từ engine mobile) và lật gương bằng transform của layer. Không libmpv trên mobile/TV. | jPlayer Mobile và jPlayer TV đã chạy ổn trên máy thật (người dùng xác nhận 2026-11-14); TV đo B rẻ hơn chép khung (~36 % so với 53 % CPU, 0 cấp phát). Tài liệu Kế hoạch kỹ thuật ghi libmpv: sai, cần sửa (Q7). |
| D5 | Font nhúng: Barlow Condensed (700, 800), Inter (400, 500, 600, 700, 800), JetBrains Mono (500, 700); OFL; subset Latin + tiếng Việt; ghi ra cache rồi đăng ký như `jplayer-mobile/internal/app/fonts.go`. Chọn mặt chữ theo **PostScript name** với `WeightRegular` như `jplayer-tv/internal/ui/theme/faces.go`, vì Core Text của gDesk gom mọi weight ≥ 600 thành bold (GT15 của jPlayer TV) — Inter 600/700/800 phải ra đúng ba mặt. | `--disp`, `--body`, `--mono` của prototype. |
| D6 | Token sinh từ `reference/design/tokens.json` → `internal/tokens/tokens.go` (generated, không sửa tay). Màu của prototype `:root` là nguồn kiểm: mọi token màu phải bằng biến CSS tương ứng (test). | Một nguồn cho thiết kế, prototype, app. |
| D7 | Dữ liệu hiển thị lấy từ API/mock, không từ bộ sinh bài mẫu của prototype (`lessonsOf`). So ảnh pixel chỉ ở các trạng thái mà dữ liệu mock trùng prototype (Set 35, tài khoản, TV pairing); các trạng thái khác so bố cục (`layout.json`), không so chữ. | Mock là hợp đồng với backend; prototype chỉ có tên bài thật cho Set 35. |
| D8 | Focus TV: hình học của gDesk (`ui.WithDirectionalFocus()`) + lớp `focus` của jPlayer TV chép sang (`jplayer-tv/internal/ui/focus`: chọn phần tử đầu, chữa focus theo khoá khi cây dựng lại (không nhớ focus từng màn: prototype không có, `tvGo` đặt focus về mặc định), chuỗi xử lý Menu và Play/Pause, giữ Select 550 ms; 0 cấp phát). Thuật toán prototype (`moveFocus`, điểm = khoảng chính + 2,2 × lệch ngang) khác của gDesk (`framework/ui/focusdir.go`: cùng hàng/khác hàng, `focusOffRow` 400). Test so 400 vector `focus.json`; khác biệt được ghi vào K9 kèm ảnh; chỉ khi một màn đi sai thứ tự mà prototype đi đúng mới làm G6. | Không viết engine focus riêng khi gDesk đã có engine TV đo trên tvOS. |
| D9 | Lưu token đăng nhập trong Keychain (iOS, tvOS) và Android Keystore qua `bridge/system/secret` khi G1 xong; trước đó dùng `secret.FileStore` mã hoá trong thư mục dữ liệu app (dự phòng F1). | `bridge/system/secret` hiện chỉ có driver macOS/Windows/Linux (`bridge/backend/{darwin,win32,gtk}/services.go`). |
| D10 | Không commit/push/merge/tag ở cả `dancefitvn-app` và `../gdesk-dancefitvn-app` khi người dùng chưa yêu cầu trong tin nhắn hiện tại. Mọi thay đổi gDesk phải được **đề xuất trước** (lý do, vị trí, ảnh hưởng) theo `../gdesk-dancefitvn-app/CLAUDE.md` §1. | Quy định của gDesk. |
| D11 | Mã, comment, godoc, test, README, commit: tiếng Anh. Chuỗi hiển thị tiếng Việt nguyên văn từ prototype trong `locales/vi.json`. | Quy ước JiveCode/gDesk. |

## 4. Kiến trúc

### 4.1 Workspace

```
~/Working/
├── gdesk/                    gDesk gốc, nhánh main — KHÔNG sửa
├── gdesk-dancefitvn-app/     worktree gDesk, nhánh dancefitvn-app-support  ← mọi sửa gDesk cho app này
├── dancefitvn-app/           repo này
└── jplayer-mobile/  jplayer-tv/      chỉ đọc: nguồn chép engine, focus TV, kit chữ (biến JPLAYER_MOBILE, JPLAYER_TV)
```

`go.mod` có `replace` trỏ vào `../gdesk-dancefitvn-app` (do `gdesk init -local` tạo).

### 4.2 Lớp và quy tắc archtest

| Lớp | Gói | Được import |
|---|---|---|
| core (không UI) | `internal/model`, `internal/api`, `internal/api/mock`, `internal/auth`, `internal/store`, `internal/progress`, `internal/download`, `internal/format`, `internal/qr`, `internal/pairing` | stdlib, `foundation/core`; không `framework/*`, không `bridge/backend/*` |
| engine | `internal/media` (interface), `internal/media/avf` (`ios && !tvos`), `internal/media/avftv` (`ios && tvos`), `internal/media/amp` (`android`), `internal/media/fake` | core, `foundation/ffi`, `bridge/backend/android/jni`, `foundation/platform` |
| player | `internal/player` (lõi phát dùng chung mobile và TV; P4.3) | core, `internal/media` (interface), `internal/progress`, `internal/clock` |
| system | `internal/sys` (secret, mạng, bộ nhớ trống, haptic, keep-awake, image picker) | core, `bridge/system/*`, `foundation/platform`, `foundation/ffi`, `bridge/backend/android/jni` |
| ui chung | `internal/tokens` (generated), `internal/ui/theme`, `internal/ui/icon`, `internal/ui/poster`, `internal/ui/kit` | core, `framework/ui`, `framework/components/headless` |
| ui mobile | `internal/ui/mobile/...` (`//go:build !tvos`) | ui chung, engine, system, player |
| ui tv | `internal/ui/tv/...` (`//go:build tvos`) | ui chung, engine, system, player |
| app | `internal/app`, `main.go` | tất cả |

Archtest (`internal/archtest`): core không import UI; `internal/ui/mobile` và `internal/ui/tv` không import nhau; không chuỗi tiếng Việt trong `.go` ngoài `locales/` và test fixtures; gói có build tag luôn có `_other.go` trả `core.ErrUnsupported`.

### 4.3 Cây thư mục

```
dancefitvn-app/
├── main.go                     chọn shell mobile hoặc tv theo build tag
├── gdesk.json                  ios, android, tvos (D1)
├── cmd/contractcheck/          kiểm server thật với mock (05 §5)
├── (cmd/avspike, cmd/ampspike: đã chuyển sang local/trash/cmd/, Q-P4-16)
├── internal/
│   ├── app/                    khởi động, font, cấu hình, điều hướng gốc, vòng đời
│   ├── api/                    Client, Transport, Live/Hybrid, lỗi → mã (05 §3)
│   ├── api/mock/               MockTransport đọc routes.json (05 §4)
│   ├── model/                  User, Course, Lesson, Progress, Device, AppConfig… (05 §2)
│   ├── auth/                   phiên, refresh một lần (single-flight), đăng xuất
│   ├── store/                  cache JSON + ETag, prefs, trạng thái ngoại tuyến
│   ├── progress/               lưu vị trí 10 s, ngưỡng 0,9, hàng chờ ngoại tuyến
│   ├── download/               tải MP3 Range, sha256, chỉ Wi-Fi, tối đa 2 song song
│   ├── pairing/                TV: start/poll; mobile: confirm
│   ├── format/                 mm:ss, "8,8 MB", ngày dd/MM/yyyy, tiếng Việt
│   ├── qr/                     mã QR (byte mode, ECC M) — gDesk không có
│   ├── media/{fake,avf,avftv,amp}/   engine phát: iOS, tvOS, Android (04)
│   ├── sys/                    hệ thống (06)
│   ├── tokens/                 generated từ reference/design/tokens.json
│   └── ui/{theme,icon,poster,kit,mobile,tv}/
├── locales/vi.json
├── assets/fonts/ assets/brand/
├── mock/dancefitvn-mock-api/   hợp đồng API (không sửa ngoài P8.1)
├── reference/                  prototype, design, tools (không sửa)
└── local/                      ghi chú, ảnh so sánh, vectors (gitignore)
```

### 4.4 Luồng (thread)

| Việc | Luồng | Cách về UI |
|---|---|---|
| Vẽ, sự kiện, focus | UI thread | — |
| HTTP (api, download, pairing poll) | goroutine | `app.Enqueue` / `window.Post` |
| Tick vị trí phát (4 Hz cho thanh, 10 s cho lưu tiến độ) | timer engine → UI | `ui.Go` |
| Callback AVPlayer / MediaPlayer | luồng hệ thống | đẩy vào kênh, UI đọc qua `window.Post` |
| Ghi file tải, sha256 | goroutine | sự kiện tiến độ ≤ 4 lần/giây |

### 4.5 Mô hình dữ liệu

Theo JSON của mock (`mock/dancefitvn-mock-api/`), chi tiết ở 05 §2. Khoá chính: `user.id` (`u_412`), `course.id` (`set-35`), `lesson.id` (`l_3503`), `device.id` (`d_101`). Prototype dùng `set35` và `set35-3` (K10).

## 5. Các phase

Mỗi phase: **Việc**, **Đọc trước**, **Nghiệm thu**. Bảng giao việc chi tiết ở 08 §3.

### P0 — Khả thi & khung (cổng khả thi)

- **Việc:** (1) kiểm workspace, worktree, `replace`, cài `gdesk` từ worktree; (2) `gdesk.json` với `ios`, `android`, `tvos`, id `com.jivecode.dancefitvn`; app tối thiểu chạy trên simulator iPhone, emulator Android 412×892 và simulator Apple TV; (3) font tiếng Việt hiển thị đủ dấu ở cả Core Text và gotext; (4) spike `cmd/avspike`: AVPlayer phát HLS dọc 9:16 từ `mock/.../media/hls/l_3503/master.m3u8` trên iOS và tvOS, đổi `rate` 0.5–1.5, vẽ khung lật, đo cấp phát mỗi khung (04 §5); (5) spike `cmd/ampspike`: MediaPlayer phát cùng HLS trên Android, `setPlaybackParams` tốc độ, vẽ khung lật; (6) kiểm `bridge/system/secret` trên iOS/Android (dự kiến `ErrUnsupported` → F1); (7) chụp prototype `capture_dancefitvn.py`, sinh vectors `gen_vectors.js`, chuỗi `extract_strings.py`; (8) quyết định đi tiếp.
- **Đọc trước:** 04, 06, 07 §3, `../gdesk-dancefitvn-app/kits/mobile/README.md`, `kits/cli/README.md`, `framework/ui/README.md` (TV, video underlay), `bridge/backend/uikit/doc.go`.
- **Nghiệm thu:** ba app tối thiểu chạy; HLS phát ở cả ba hệ với tốc độ và lật gương (hoặc dự phòng F3 được ghi); `local/p0-results.md` có số đo; `local/shots/prototype/{mobile,tv}` đủ 43 trạng thái; `go test ./...` xanh. Không đạt mục (2) hoặc (4)/(5) mà không có dự phòng → **dừng bắt buộc** (08 §5).

### P1 — Lõi dữ liệu & mock (không UI)

- **Việc:** `model`, `api` (Client, lỗi, header `X-App-Version`, `X-Platform`, `X-Install-Id`, `Accept-Language`, ETag), `api/mock` (05 §4 đủ 6 quy tắc), `auth` (đăng nhập, refresh single-flight khi 401 `token_expired`, đăng xuất), `store`, `progress`, `download`, `pairing`, `format`, `qr`, `cmd/contractcheck`.
- **Đọc trước:** 05, tài liệu API v1 (`docs/api/` nếu đã xuất; link ở README), `mock/.../README.md`, `routes.json`.
- **Nghiệm thu:** test table-driven cho 24 route × mọi kịch bản trong `routes.json`; `contractcheck` chạy `-self` (so mock với chính nó) PASS; `format` khớp `fmt.json`, `size.json`; `qr` giải được bằng decoder chuẩn (test với ảnh) ; 0 cấp phát ở `format.Clock` (benchmark).

### P2 — Nền giao diện

- **Việc:** `tokens` sinh mã, `theme`, `icon` (bộ icon của prototype `IC`), `poster` (bìa khoá vẽ bằng gradient + vòng + vũ công hoặc đĩa than, công thức `poster()`), `kit`: Button (p/g/s/r/t), Chip (c-th/c-db/c-gt/y/k/okc), Input (thường/focus/lỗi), Row, Switch, SegmentedControl, Radio, Progress, Sheet + dim, Toast, TabBar, Top bar, Badge, Codebox, Avatar; điều hướng stack của mobile (`go`, `back`, sheet).
- **Đọc trước:** 02 toàn bộ, 03 §1–§2.
- **Nghiệm thu:** trang `kit` demo (`-tags kitdemo`) chụp ảnh khớp các thành phần trong `local/shots/prototype` (layout lệch ≤ 1 pt); test token màu = biến CSS; mỗi thành phần có test `ui.Tester` trên `Phones` (393×852, 412×892).

### P3 — Mobile: vào app & duyệt khoá học

- **Việc:** màn 01 Khởi động, 02/02B Đăng nhập (lỗi), đăng nhập bằng Face ID / vân tay (G3 hoặc F6, K13), 03B Đang tải, 03 Trang chủ, 04 Khoá học (lọc 4 loại, tìm kiếm, rỗng), 05 Chi tiết Set, 05B Khoá chưa tham gia; tab bar; trạng thái ngoại tuyến trên các màn này.
- **Nghiệm thu:** so ảnh M01, M02, M02B, M03B, M03, M04, M04-special, M04-empty, M05, M05B (07 §3); luồng đăng nhập mock + mọi kịch bản lỗi của `auth.login`; tìm kiếm khớp 03 §4.

### P4 — Mobile: phát bài

- **Việc:** 06 Video dọc (tự ẩn điều khiển sau 3 s, chạm để hiện/ẩn — 03 §5a, tua ±10 s, bài trước/sau, A–B, 8 nhịp, lật gương, tốc độ), 07 sheet Tuỳ chọn phát, 08 Nghe file âm thanh (nền, Now Playing, màn khoá), 10 Hết bài (đếm 5 s tự phát tiếp), lưu tiến độ.
- **Nghiệm thu:** so ảnh M06, M06-playing, M06-hidden, M07, M08, M10; tự ẩn đúng 3 s (test `Elapse`); trên simulator/emulator: phát HLS, đổi tốc độ, lật gương, A–B lặp đúng, lưu vị trí mỗi 10 s và khi thoát, ≥ 90 % → "đã xem"; nghe nền khi khoá màn hình (iOS + Android); 0 cấp phát mỗi khung ổn định.

### P5 — Mobile: tải xuống & ngoại tuyến

- **Việc:** 09 sheet Tải (bài này / cả khoá, chỉ Wi-Fi), 11 Tải xuống, 12 rỗng, 13 Ngoại tuyến, 14 Xoá file, hàng tiến độ trong danh sách bài, đồng bộ tiến độ khi có mạng lại (`progress.batch`).
- **Nghiệm thu:** so ảnh M09, M11, M12, M13, M14; tải Range nối tiếp sau khi mất mạng; sha256 sai → xoá + toast; phát MP3 khi máy bay; tiến độ ngoại tuyến gửi lên khi có mạng.

### P6 — Mobile: tài khoản & kết nối TV

- **Việc:** 15 Tài khoản (huy hiệu, cài đặt Wi-Fi, tự phát tiếp), 15B Thông tin (form, kiểm tra), Đổi mật khẩu, 15C Hồ sơ HLV (mạng xã hội, địa điểm), 15D Ảnh đại diện (chọn ảnh, kéo, thu phóng, cắt 512×512), Đăng xuất, 16 Kết nối Apple TV (nhập mã 6 ô, đã kết nối, ngắt), thiết bị.
- **Nghiệm thu:** so ảnh M15, M15B, M15B-password, M15C, M15D, M15-logout, M16, M16-code, M16-linked, M-toast; mọi kịch bản lỗi `me.*`, `tv.pair.confirm`.

### P7 — Apple TV

- **Việc:** khung 1920×1080 (safe area 60/80 theo gDesk), thanh điều hướng 4 mục, focus theo hướng, TV01 Chào mừng, TV02 Đăng nhập bằng mã (mã + QR thật + poll 3 s + hết hạn), TV04 Trang chủ, TV05 Khoá học, TV06 Chi tiết, TV07 Phát video, TV08 Panel tuỳ chọn, TV09 Nhạc tập, TV10 Tiếp theo, TV11 Mất kết nối, TV12 Cài đặt & đăng xuất, toast; Play/Pause, Menu = quay lại; giữ màn hình sáng khi phát.
- **Nghiệm thu:** so ảnh TV01–TV12, TV-toast (07 §3); random-walk 600 phím (seed cố định) không kẹt focus; `focus.json` khác biệt được ghi K9; ghép mã với mobile (mock) thành công.

### P8 — Hoàn thiện & phát hành thử

- **Việc:** `hybrid`/`live` khi server gFly có (dùng `contractcheck`), hiệu năng, trợ năng (VoiceOver/TalkBack nhãn nút icon), `PrivacyInfo.xcprivacy`, brand assets tvOS, `gdesk package` ipa/aab/tvOS ipa, danh sách kiểm máy thật (07 §6).
- **Nghiệm thu:** đủ 07 §6; `contractcheck` PASS với server (nếu có; nếu chưa: ghi BLOCKERS); kích thước app, thời gian mở, bộ nhớ trong ngưỡng 07 §5.

## 6. Rủi ro

| ID | Rủi ro | Ảnh hưởng | Giảm thiểu |
|---|---|---|---|
| R1 | Khác biệt giữa app này và jPlayer trên tvOS (lưới bìa lớn, panel mờ, QR) — jPlayer TV đo ở máy thật: lớp mờ hộp thoại tốn bộ nhớ tới 663 MB (GT29 của jPlayer TV) | Hiệu năng/bộ nhớ TV | gDesk trên iPhone, Android và Apple TV thật đã kiểm qua jPlayer (người dùng xác nhận 2026-11-14); panel TV08 và toast không dùng nền mờ toàn màn (`FadedBackdrop`) nếu đo P7 vượt ngưỡng 07 §5; P8 máy thật |
| R2 | Tốc độ 0.5–1.5x khi HLS đang đệm, lật gương video dọc: chưa đo trong jPlayer | Tính năng chính cho HLV | Engine jPlayer đã chạy ổn trên mobile và Apple TV thật (người dùng xác nhận); spike P0 chỉ đo tốc độ + lật |
| R3 | HLS trên `MediaPlayer` kém ổn định ở một số máy | Video không phát | Thử 2 emulator + 1 máy thật; dự phòng F3: MP4 progressive (cần backend — Q8) |
| R4 | `bridge/system/secret` chưa có iOS/Android (G1) | Token lưu kém an toàn hơn | F1 FileStore mã hoá; đề xuất G1 cho gDesk |
| R5 | Tải nền iOS (URLSession background) chưa có trong gDesk (G5) | Tải dừng khi rời app | F5: tải khi app mở + nối tiếp bằng Range |
| R6 | Mock lệch server thật | Lỗi khi chuyển live | `contractcheck` trong CI của backend; mock chỉ đổi ở P8 (P8.1) khi server thật xác nhận |
| R7 | Duyệt App Store: app chỉ cho người có tài khoản | Trễ phát hành | Tài khoản demo cho reviewer, ghi chú (tài liệu Kế hoạch kỹ thuật) |
| R8 | Nội dung thật (video, nhạc) và bản quyền nhạc do DanceFITVN quản lý | Không phải quyết định của dev | Chỉ dùng media mẫu tự tạo; ghi BLOCKERS khi cần nội dung thật |

## 7. Ước lượng

| Phase | Ngày-người |
|---|---|
| P0 | 4 |
| P1 | 6 |
| P2 | 5 |
| P3 | 5 |
| P4 | 7 |
| P5 | 4 |
| P6 | 5 |
| P7 | 8 |
| P8 | 5 |
| **Tổng** | **49** (≈ 10 tuần một người; chạy tự động bằng sub-agent nhanh hơn, phần đo trên máy thật vẫn cần người) |

## 8. Câu hỏi mở

| ID | Câu hỏi | Mặc định nếu chưa trả lời |
|---|---|---|
| Q1 | Bản 1.0 có cần giao diện sáng (màn 17 của bảng thiết kế)? | **Đã trả lời 2026-11-14: không** |
| Q2 | Có đăng nhập Face ID / vân tay ở 1.0? | **Đã trả lời 2026-11-14: có** — làm ở P3 (G3, dự phòng F6 = tự viết trong `internal/sys`; K13) |
| Q3 | Giới hạn thiết bị 2 điện thoại + 1 TV mỗi tài khoản? | Như mock (`device_limit`) |
| Q4 | Bundle ID và tài khoản Apple/Google đứng tên ai? | **Đã trả lời 2026-11-14: bundle ID `com.jivecode.dancefitvn`**; tài khoản phát hành chờ xác nhận trước P8 |
| Q5 | DanceFITVN gửi 1–2 bài video/nhạc thật để thử? | Dùng media mẫu trong mock |
| Q6 | Đổi thư mục gDesk `~/Working/desk` thành `~/Working/gdesk` theo quy ước? | README hướng dẫn đổi |
| Q7 | Sửa tài liệu Kế hoạch kỹ thuật: engine phát mobile/TV là AVPlayer/MediaPlayer, không libmpv (D4)? | Theo D4 |
| Q8 | Backend có thể cung cấp MP4 progressive dự phòng cho máy Android lỗi HLS? | Không; ghi BLOCKERS nếu R3 xảy ra |
| Q9 | API thêm `bpm` cho mỗi bài để thanh 8 nhịp chạy đúng nhạc? | **Đã trả lời 2026-11-14: không có `bpm`** — luôn 120 BPM như prototype (K16) |
| Q10 | Các nút prototype chỉ hiện toast (hẹn giờ dừng, chia sẻ, "Tìm hiểu thêm") có làm thật ở 1.0? | Giữ toast như prototype (K15) |
| Q11 | Ảnh bìa thật trên website đã có chữ "SET 35"…? (quyết định vẽ chữ đè hay không) | Live: dùng ảnh nguyên, không vẽ chữ (02 §5) |
| Q12 | Màn video có tự ẩn điều khiển sau 3 s? | **Đã trả lời 2026-11-14: có** — prototype đã cập nhật (03 §5a, trạng thái M06-hidden) |
| Q13 | Duyệt các chuỗi mới không có trong prototype (lỗi, phiên hết hạn, cần cập nhật, đổi mật khẩu thành công…) | Dùng chuỗi đề xuất, ghi PROGRESS §7 |
