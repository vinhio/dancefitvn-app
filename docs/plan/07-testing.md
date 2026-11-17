# 07 — Kiểm thử

## 1. Các lớp test

| Lớp | Công cụ | Chạy ở |
|---|---|---|
| Đơn vị (core) | `testing` + testify, table-driven, `t.Parallel()` | `go test ./...` trên Mac |
| Hợp đồng mock | test của `internal/api/mock` + `cmd/contractcheck -self` | Mac |
| Vector prototype | `local/vectors/{fmt,size,focus}.json` từ `gen_vectors.js` | Mac |
| Giao diện | `ui.Tester` (driver fake), `Tester.Elapse` thay `sleep`, `KeyDown/KeyRepeat/KeyUp` cho remote | Mac |
| Engine & hệ thống | simulator iPhone, emulator Android 412×892, simulator Apple TV | Mac + Xcode + Android SDK |
| So ảnh | `capture_dancefitvn.py` (prototype) vs ảnh app (`scripts/shot.sh`) | Mac |
| Máy thật | danh sách tay §6 | người |

`go test -count=1 ./...` chạy được trên Mac không cần simulator: gói có build tag có `_other.go`.

## 2. Ca bắt buộc

- `format`: mọi vector `fmt.json`, `size.json`; ngày `dd/MM/yyyy`; `Course.Short()`.
- `api/mock`: 24 route × mọi kịch bản trong `routes.json` trả đúng status và file; `{{path.id}}`; 204; route `auth` khi chưa đăng nhập; đổi `mock-media://`; dự phòng `_sample`.
- `api`: header gửi đi; lỗi → `*api.Error` đúng `Code`; refresh single-flight (10 yêu cầu đồng thời nhận 401 → đúng 1 lần refresh); refresh lỗi → `ErrSignedOut`; ETag 304.
- `auth`, `store`: lưu/đọc/xoá phiên (F1); đăng xuất giữ file tải.
- `progress`: 10 s; ngưỡng 0,9; hàng chờ ngoại tuyến gửi theo lô ≤ 100; không gửi trùng.
- `download`: Range nối tiếp; sha256 sai; huỷ; tối đa 2 song song; Wi-Fi only.
- `pairing`: poll đúng nhịp, dừng khi rời màn, hết hạn → start lại.
- `qr`: mã hoá `https://dancefitvn.com/tv?c=7K4M2P` rồi giải lại được (decoder trong test, viết tay hoặc vector từ thư viện chuẩn chạy ngoài — không thêm dependency vào module app).
- Locale: mọi giá trị `locales/vi.json` có trong `extract_strings.py` hoặc 03 §8, hoặc đánh dấu "mới" (Q13).
- Archtest: PLAN §4.2.
- UI mobile: mỗi màn và sheet có test `ui.Tester` ở cả hai máy `Phones` (393×852, 412×892): dựng không lỗi, hành động chính (03 §3), toast đúng chữ và tự tắt sau 2200 ms.
- UI TV: mỗi màn, mặc định focus (03 §7.2), Menu/Play-Pause, random-walk 600 phím seed 20261114: sau mỗi phím đúng một widget có focus, không kẹt panel, không panic.
- Focus: 400 vector `focus.json` chạy qua `View.MoveFocus` trên cây widget dựng từ các hình chữ nhật; ghi số khác prototype vào `local/focus-diff.md` (D8, K9) — test không fail vì khác biệt, fail nếu focus đi vào phần tử không tồn tại hoặc mất focus.
- Hiệu năng: benchmark + `…DoesNotAllocate` cho vẽ khung ổn định (Trang chủ, danh sách bài, màn video đang phát, TV lưới khoá học khi di focus), `format.Clock`, cập nhật vị trí phát.

## 3. So ảnh với prototype

### 3.1 Chụp prototype

```sh
cd reference/prototype && python3 -m http.server 5180 &
python3 -m venv local/.venv && local/.venv/bin/pip install playwright && local/.venv/bin/python -m playwright install chromium
TZ=UTC local/.venv/bin/python reference/tools/capture_dancefitvn.py --url http://localhost:5180 --out local/shots/prototype --dpr 3   # mobile @3x
TZ=UTC local/.venv/bin/python reference/tools/capture_dancefitvn.py --url http://localhost:5180 --out local/shots/prototype-1x       # layout.json chuẩn
node reference/tools/gen_vectors.js local/vectors
python3 reference/tools/extract_strings.py --out local/strings.json
```

Ảnh 1x đã chụp sẵn để tham khảo: `reference/prototype/screenshots/{mobile,tv}/`.

### 3.2 Trạng thái

| ID | Màn | Phase | So |
|---|---|---|---|
| M01 | Khởi động | P3 | pixel |
| M02 | Đăng nhập | P3 | pixel, che dòng gợi ý (K2) |
| M02B | Đăng nhập lỗi | P3 | pixel, che gợi ý (K2) |
| M03B | Đang tải | P3 | pixel |
| M03 | Trang chủ | P3 | pixel, che lời chào ngày (đồng hồ) |
| M04 | Khoá học | P3 | pixel |
| M04-special | Lọc Đặc biệt | P3 | pixel |
| M04-empty | Tìm không thấy | P3 | pixel |
| M05 | Chi tiết Set 35 | P3 | pixel |
| M05B | Khoá chưa tham gia | P3 | layout |
| M06 | Video (dừng) | P4 | layout (vùng video là khung thật, K5) |
| M06-playing | Video (phát) | P4 | layout |
| M06-hidden | Video (phát, điều khiển đã ẩn sau 3 s) | P4 | layout |
| M07 | Sheet tuỳ chọn phát | P4 | pixel phần sheet |
| M08 | Nghe file âm thanh | P4 | pixel |
| M09 | Sheet tải | P5 | pixel, che "Bộ nhớ trống" (K11) |
| M10 | Hết bài | P4 | pixel |
| M11 | Tải xuống | P5 | pixel, che dung lượng trống (K11) |
| M12 | Tải xuống rỗng | P5 | pixel |
| M13 | Ngoại tuyến | P5 | pixel |
| M14 | Sheet xoá file | P5 | pixel |
| M15 | Tài khoản | P6 | pixel, che avatar `[33,131,82,82]` và 3 huy hiệu (x 39.5, 160.5, 281.5; y 286; 72×72) |
| M15B | Thông tin tài khoản | P6 | pixel |
| M15B-password | Sheet đổi mật khẩu | P6 | pixel, che avatar `[33,131,82,82]` và 3 huy hiệu (x 39.5, 160.5, 281.5; y 286; 72×72) |
| M15C | Hồ sơ HLV | P6 | pixel |
| M15D | Sheet ảnh đại diện | P6 | pixel, che avatar `[33,131,82,82]` và 3 huy hiệu (x 39.5, 160.5, 281.5; y 286; 72×72); che vùng trong khung cắt `[20,393,353,230]` (ảnh mock) |
| M15-logout | Sheet đăng xuất | P6 | pixel, che avatar `[33,131,82,82]` và 3 huy hiệu (x 39.5, 160.5, 281.5; y 286; 72×72) |
| M16 | Kết nối Apple TV | P6 | pixel, che dòng K24 `[20,448,353,18]` |
| M16-code | Đã nhập mã | P6 | pixel, che dòng K24 `[20,448,353,18]` |
| M16-linked | Đã kết nối | P6 | pixel |
| M-toast | Toast | P6 | pixel phần toast; che mọi thứ khác |
| TV01 | Chào mừng | P7 | pixel |
| TV02 | Đăng nhập bằng mã | P7 | pixel, che QR (K1), giờ hết hạn (K8), bỏ nút mô phỏng (K2) |
| TV04 | Trang chủ | P7 | pixel |
| TV05 | Khoá học | P7 | pixel |
| TV06 | Chi tiết | P7 | pixel |
| TV07 | Phát video | P7 | layout (vùng video) |
| TV08 | Panel tuỳ chọn | P7 | pixel phần panel |
| TV09 | Nhạc tập | P7 | pixel |
| TV10 | Tiếp theo | P7 | pixel |
| TV11 | Mất kết nối | P7 | pixel |
| TV12 | Cài đặt | P7 | pixel, che phiên bản (K2) |
| TV-toast | Toast | P7 | pixel phần toast |

### 3.3 Ngưỡng

- **Layout:** mỗi phần tử có cùng khoá trong `layout.json` của app và prototype: x, y, w, h lệch ≤ 1 pt (mobile), ≤ 2 px (TV); cỡ chữ, đậm, màu, bo góc bằng nhau.
- **Pixel:** sau khi che vùng ghi ở §3.2, ≤ 1,5 % điểm ảnh lệch quá 32/255 ở một kênh (chênh khử răng cưa giữa Chromium và Core Text/gotext); ảnh chênh kèm vào `local/shots/diff/<ID>.png`.
- Ảnh Android 412×892 so với prototype 393×852 bằng **layout** (cùng bố cục, rộng hơn): khoảng cách hai lề giữ 20, phần tử kéo giãn theo chiều ngang như CSS (flex) của prototype.
- App chụp bằng `scripts/shot.sh <ID>`: build dev (`-tags dancefitvndev`) đọc `DFV_STATE=<ID>` dựng trạng thái tương ứng từ mock, chụp bằng `xcrun simctl io booted screenshot` / `adb exec-out screencap` / simulator Apple TV, và dump `layout.json` cùng định dạng (widget `Name` = khoá).

## 4. Dữ liệu và media thử

Media mẫu trong `mock/dancefitvn-mock-api/media/` (HLS 480×854 12 s, MP3 30 s 128 BPM, ảnh "MOCK"). Không dùng nội dung thật (05 §7).

## 5. Hiệu năng

| Chỉ số | Ngưỡng | Đo |
|---|---|---|
| Mở app tới Trang chủ (mock, máy đã đăng nhập) | ≤ 1,5 s iPhone 13 / ≤ 2,5 s Android tầm trung | `GDESK_FRAMESTATS=1`, log mốc |
| Khung hình khi cuộn danh sách bài | 60 fps, không khung > 32 ms | framestats |
| TV di focus | ≤ 1 khung trễ, 60 fps | framestats |
| Cấp phát mỗi khung ổn định | 0 | benchmark |
| Bộ nhớ khi phát video 10 phút | không tăng > 10 MB | Instruments / `dumpsys meminfo` |
| Kích thước app | ≤ 40 MB (ipa), ≤ 30 MB (aab mỗi ABI) | `gdesk package` |

Cách đo (P8.4, `scripts/perf-*.sh`; chi tiết ở `scripts/README.md`): khởi động = `perf-start.sh` (N lần khởi động lạnh, trung vị và p90 từ tiến trình tới mốc `dancefitvn: mark home`; simulator đo theo đồng hồ Mac, Android theo `am start -W`); cuộn = `perf-scroll.sh` (khung > 32 ms, fps qua `GDESK_FRAMESTATS`); TV focus = `tvos-framestats.sh`; bộ nhớ video = `perf-video.sh` (footprint/PSS mỗi 30 s, tăng sau 30 s làm nóng); kích thước = ipa/APK của `release.sh`. Số đo trên simulator và emulator chỉ là ước lượng, ghi nhãn; máy thật ở mục 6.

## 6. Danh sách kiểm máy thật (P8)

- [ ] iPhone (iOS 17+, 26): đăng nhập, phát video HLS 4G và Wi-Fi, đổi tốc độ, lật gương, A–B, nghe nền khi khoá máy, Now Playing, tải 10 bài, máy bay → nghe được
- [ ] Android 9 / 14 / 15+: như trên, Back hệ thống, thông báo media, cử chỉ mép
- [ ] Apple TV 4K (tvOS 17+, 26): ghép mã thật từ điện thoại, quét QR, focus toàn bộ màn, Menu về Home khi ở màn gốc, Play/Pause, màn hình không tắt khi phát, Top Shelf, icon lớp
- [ ] Mất mạng giữa chừng: video dừng → màn 13 / TV11; có mạng lại → tiếp tục
- [ ] VoiceOver đọc được mọi nút icon. TalkBack: chưa có trong 1.0 (gDesk chưa có cầu, đề xuất G10 — giới hạn đã biết, Q-P8-15)
- [ ] Bản `-tags live`: không có menu dev, mode live (`scripts/check-livebuild.sh`)
