# 03 — Đặc tả hành vi

Nguồn: mã JS của `reference/prototype/index.html`. Khi tài liệu này khác mã, mã thắng.


## 1. Điều hướng mobile

- Stack: `go(scr)` đẩy màn hiện tại vào stack; `go(scr, {root})` (tab bar, `data-root="1"`) xoá stack; `back()` lấy màn trên cùng, không còn thì về Trang chủ. Rời màn video (khi đang ở chế độ video) thì dừng phát.
- Mở sheet không đổi màn; đóng sheet bằng nút ✕, "Huỷ", "Xong", chạm nền mờ, hoặc vuốt xuống tay nắm (app thêm vuốt xuống — chuẩn hệ thống; ngưỡng 80 pt hoặc vận tốc > 600 pt/s).
- Vẽ lại cùng màn giữ vị trí cuộn; vào màn mới cuộn về đầu (`renderM`, `go`).
- Back của Android và vuốt mép trái của iOS = `back()`; đang mở sheet thì đóng sheet trước.
- Khởi động: màn 01 hiện 1300 ms (prototype) — app: hiện tới khi đọc xong phiên lưu (`Restore`) và đã đủ 600 ms, tối đa 1300 ms; nếu tới 1300 ms việc đọc phiên vẫn chạy thì chờ tiếp. Không chờ `GET /app/config` (chạy nền; 426 → màn nâng cấp, Q-P3-22); có phiên → 03B Đang tải → 03; không → 02.
- Đăng nhập: đúng → 03B tới khi `GET /me/home` về (prototype 900 ms) → 03, stack rỗng.
- Đăng xuất (sheet): gọi `POST /auth/logout`, xoá token, giữ file đã tải, về 02, stack rỗng.

## 2. Ngoại tuyến (mobile)

App coi là ngoại tuyến khi `internal/sys` báo mất mạng (G4) hoặc, khi chưa có G4, khi một yêu cầu thất bại vì lỗi mạng (không phải lỗi HTTP) và thử lại `GET /app/config` sau 5 s, 10 s, 20 s… (tối đa 60 s) để biết khi nào có mạng lại.

| Thao tác khi ngoại tuyến | Kết quả (prototype) |
|---|---|
| Mở chi tiết khoá (`data-go="detail"`) | Màn 13 Ngoại tuyến của khoá đó |
| Đang ở màn video | Dừng, chuyển màn 13 |
| Đang ở chi tiết | Chuyển màn 13; có mạng lại → về chi tiết |
| Phát video | Toast "Video bài NN cần có mạng để xem" |
| Chạm bài ở màn 13 | Có file → phát âm thanh; không → toast như trên |
| Nút tải / "Tải hết" | Toast "Không có mạng để tải" |
| Tải đang chạy | Dừng tiến độ, tiếp tục (Range) khi có mạng |

Thanh `offbar` "Không có mạng · Đang dùng chế độ ngoại tuyến" ở các màn có nó trong prototype (13, và mọi màn có `offbar()`).

## 3. Bảng hành động mobile

Tên hành động giữ đúng `data-act` của prototype (`mobile/actions.go`).

| Hành động | Điều kiện | Kết quả |
|---|---|---|
| `back` | — | §1 |
| `toast` | — | Toast `data-msg` (K15) |
| `closeSheet` | — | Đóng sheet, bỏ ảnh đang chỉnh |
| `sheet` | — | Mở sheet `data-v` |
| `pwEye` | — | Hiện/ẩn mật khẩu |
| `bioLogin` | K13 | Hộp sinh trắc học (lý do: "Đăng nhập DanceFITVN") → đúng: mở khoá refresh token, `POST /auth/refresh` → 03B → 03; huỷ: ở lại 02; sai 3 lần hoặc khoá sinh trắc: ở lại 02 với mật khẩu |
| `filter` | — | Lọc `Tất cả` / `Thường` / `Đặc biệt` / `Giới thiệu` |
| `playVideo` | ngoại tuyến → toast | Bài khác bài đang phát: vị trí 0 (hoặc `progress.position_sec` nếu < 90 %), A = round(dur·0.33), B = A + 28; phát; mở màn 06 (đang ở 06/10 thì thay tại chỗ) |
| `playAudio` | — | Phát file âm thanh (file đã tải nếu có, không thì stream URL ký), mở 08 |
| `offTap` | — | §2 |
| `dlTap` | chưa tải: ngoại tuyến → toast; không → sheet 09 "Chỉ bài này" · đang tải: huỷ, toast "Đã huỷ tải" · đã tải: sheet 14 |
| `dlChoice` | — | `one` / `all` |
| `dlConfirm` | — | Tải bài này hoặc mọi bài còn thiếu của khoá; toast "Đang tải file âm thanh…" |
| `dlAll` | ngoại tuyến → toast | Sheet 09 chọn sẵn "Cả khoá" |
| `askDelete` / `delConfirm` | — | Xoá file, toast "Đã xoá file âm thanh" |
| `togglePlay` | — | Phát / dừng |
| `seek` | — | ±10 s, kẹp [0, dur − 1] |
| `mirror`, `loop`, `beat` | — | Bật/tắt; bật `loop` thì nhảy tới A |
| `speed` | sheet 07 | 0.5 / 0.75 / 1 / 1.25 / 1.5 |
| `cycleSpeed` | nút `{s}x` cuối hàng điều khiển màn 08 (nút bên màn 06 mở sheet 07, `sheet:options`, F-9) | Vòng 0.75 → 1 → 1.25 → 0.75 (giá trị khác vòng về 0.75) |
| `quality` | — | Vòng Tự động → 1080p → 720p → 480p (mobile) |
| `prevLes` / `nextLes` | trong khoá | Bài trước / sau, vị trí 0 |
| `toDetail` | — | Mở chi tiết, stack = [Khoá học] |
| `pref` | — | Bật/tắt `wifi` (Chỉ tải qua Wi-Fi), `auto` (Tự phát bài tiếp theo) |
| `addSoc` | ≥ 6 liên kết → toast "Tối đa 6 liên kết" | Thêm ô trống |
| `rmSoc`, `addPlace`, `rmPlace` | — | Thêm/xoá ô (địa điểm tối đa 5 theo API `max_places`) |
| `saveCoach` | URL không khớp `/(facebook|instagram|youtube|tiktok|zalo|threads)/i` → toast "Chỉ hỗ trợ Facebook, Instagram, YouTube, TikTok, Zalo, Threads" | Bỏ ô trống, `PUT /me/coach-profile`, back, toast "Đã cập nhật hồ sơ HLV" |
| `pickPhoto` | — | Bộ chọn ảnh (K7) → sheet 15D, zoom 1, x 0, y 0 |
| `avatarOk` | — | Cắt 512×512 theo khung tròn 200 (§6), `PUT /me/avatar`, toast "Đã cập nhật ảnh đại diện" |
| `pwSave` | Kiểm cục bộ trước khi gọi API: thiếu ô → `toast.pwRequired`; mật khẩu mới < 8 ký tự → `toast.pwShort`; nhập lại không khớp → `toast.pwMismatch` (chuỗi mới, Q13; P6 §12) | `PUT /me/password` (K2: bỏ toast bản thử); sai mật khẩu hiện tại → `toast.pwWrong`; đúng: đóng sheet, toast `toast.pwSaved` "Đã đổi mật khẩu" |
| `logout` | — | §1 |
| `linkTV` | mã < 6 ký tự: không làm gì | `POST /tv/pair/confirm`; sai → toast "Mã không đúng. Kiểm tra lại mã trên TV"; đúng → trạng thái đã kết nối, toast "Đã kết nối Apple TV" |
| `unlinkTV` | — | Xoá thiết bị TV (`DELETE /me/devices/{id}`), về trạng thái nhập mã |
| Form thông tin `infoForm` | tên rỗng hoặc email không có "@" → toast "Tên và email là bắt buộc" | `PATCH /me`, back, toast "Đã cập nhật thông tin tài khoản" |
| Tìm kiếm `qInp` | — | Lọc mỗi lần gõ (§4) |
| Ô mã `codeInp` | — | In hoa, chỉ `0-9A-Z`, tối đa 6; hiện thành 6 ô `.codebox`, gạch giữa sau ô 3 |
| Chạm thanh tua (`data-scrub="m"`) | — | Vị trí = tỉ lệ x trong thanh × dur |

## 4. Tìm kiếm & lọc khoá học

- Lọc theo `category`; đếm trên chip lấy `counts` của `GET /me/courses` (không theo bộ lọc).
- Tìm theo `name` và `tagline`, không phân biệt hoa thường. **Hiện tại không bỏ dấu**, theo mã prototype (`$P:312-315`: `trim().toLowerCase()` + `includes`; công tắc `SearchFoldAccents` = false). Bỏ dấu tiếng Việt (`đ` → `d`) là K18: đề xuất, chờ người dùng duyệt; khi duyệt, đổi công tắc `SearchFoldAccents` thành true.
- Không có kết quả: màn 04-empty "Không tìm thấy khoá học “{q}”" (ảnh M04-empty).
- Dòng "Hiển thị N khoá học" cập nhật theo kết quả.

## 5. Phát

| Quy tắc | Giá trị | Nguồn |
|---|---|---|
| Nhịp cập nhật vị trí trên UI | 250 ms | `setInterval(...,250)` |
| Lưu tiến độ | mỗi 10 s khi phát, khi dừng, khi rời màn (`PUT /lessons/{id}/progress`) | `app/config.progress.save_interval_sec` |
| Đã xem | `position ≥ 0.9 × duration` | `watched_threshold` |
| Hết bài (video, ở màn 06) | Màn 10 Hết bài, đếm 5 → 0 mỗi giây; về 0 và "Tự phát bài tiếp theo" bật → phát bài sau | vòng 1 s |
| Lặp A–B | bật: A = vị trí lúc bật, B = min(A + 30 s, dur); `pos ≥ B` → `pos = A` (video và audio, K38; prototype: A = round(dur·0,33), B = A + 28, chỉ video) | vòng 250 ms |
| Tua | kéo thanh vị trí tới mọi điểm (K38): xem trước khi kéo, **một** seek lúc thả, dừng ở `dur − 1 s`; vào phần ≥ 90 % bằng tua tay không tính đã xem cho tới khi phát tới cuối | `kit.Scrub`, `kit.Wave`, `progress.Tracker.Seeked` |
| 8 nhịp | `floor(pos × 2) mod 8` (K16) | `liveM` |
| Sóng âm (08) | thanh thứ i "đã nghe" khi `i / n ≤ pos / dur` | `liveM` |
| Tốc độ áp cho engine | `rate` AVPlayer / `PlaybackParams.speed` MediaPlayer | — |
| Lật gương | lật ngang khung video (04 §2.3, §3.3) | `.vstage.mir` |
| Âm thanh nền | mobile: tiếp tục khi khoá màn hình, Now Playing có tên bài, khoá, ±10 s | 04 §4 |

## 5a. Tự ẩn điều khiển video (Q12, prototype `ctlHidden`, `stageTap`, `CTL_HIDE_MS`)

- Phần tử ẩn (`.vui`): dải gradient trên, thanh trên (quay lại, tên khoá, ⋯), cột nút bên, khối điều khiển dưới. Thanh trạng thái và thanh home giữ nguyên.
- Ẩn khi: đang phát **video**, không mở sheet, và 3000 ms không chạm (đếm từ lần chạm cuối, lúc vào màn 06, lúc bấm phát/dừng, lúc tự phát bài sau, lúc chuyển từ tải sang phát — Q-P4-5). Hiệu ứng: opacity 1 → 0 trong 250 ms, ẩn thì không nhận chạm (`.scr.vid.hid .vui`).
- Chạm vùng video (`stageTap`): đang phát → đổi ẩn/hiện và đặt lại đồng hồ; đang dừng → hiện.
- Chạm bất kỳ nút nào khi đang hiện: đặt lại đồng hồ 3 s. Chạm thanh tua (`data-scrub`) tua tới vị trí nhưng **không** đặt lại đồng hồ.
- Dừng phát hoặc mở sheet → luôn hiện, không đếm.
- TV07 không tự ẩn (prototype TV không đổi).

## 6. Ảnh đại diện

- Khung `.crop` 230 cao; ảnh 260×260 căn giữa, `translate(x, y) scale(zoom)`; zoom 1–3 bước 0.01 (thanh trượt); kéo một ngón đổi x, y.
- Vùng giữ lại: vòng tròn đường kính 200 ở giữa khung. Xuất: phần ảnh dưới vòng tròn, vuông 512×512 JPEG chất lượng 0.9, ≤ 2 MB.
- "Chỉ lưu sau khi bấm Xác nhận" — huỷ thì bỏ.

## 7. TV

### 7.1 Phím

| Siri Remote (gDesk) | Prototype | Hành động |
|---|---|---|
| Mũi tên / vuốt (`KeyLeft…`) | ArrowLeft… | Di chuyển focus (D8) |
| Select (`KeyEnter`) | Enter | `tvActivate` phần tử đang focus |
| Menu (`KeyEscape`) | Escape / Backspace | Panel mở → đóng panel, focus `p2`; còn stack → `tvBack`; stack rỗng → để hệ thống về Home (gDesk tự chuyển khi app không dùng phím) |
| Play/Pause (`KeyMediaPlayPause`) | Space | Ở màn phát video / nhạc: phát/dừng |

### 7.2 Điều hướng

- Nav 4 mục (Trang chủ, Khoá học, Nhạc tập, Cài đặt): chọn mục → đổi màn tại chỗ, stack rỗng, giữ focus ở mục nav.
- `tvGo(scr)`: đẩy stack, focus về mặc định của màn mới, đóng panel; rời màn phát (trừ sang nhạc) thì dừng.
- Mặc định focus mỗi màn: phần tử `.f` đầu tiên theo thứ tự tài liệu của prototype (`renderTV`, `$P:725`; màn Cài đặt lấy `nav0`, TV12), trừ: sau Hết bài tự phát tiếp → `p1`; đóng panel → `p2`. `layout.json` không có trường `focused`: focus hiện là hộp đã phóng to (scale 1.06, F-6). Không nhớ focus theo màn: `tvGo` đặt focus về mặc định (`S.tv.focus = null`; D8).
- Danh sách và lưới cuộn theo focus: đẩy lên `max(0, top − (cao vùng − cao phần tử − 80))` (`scrollTV`).

### 7.3 Hành động TV

| `data-tact` | Kết quả |
|---|---|
| `toast` | Toast `data-msg` (K15) |
| `tback` | `tvBack` |
| `play`, `playLes` | Ngoại tuyến → TV11; bài khác: vị trí 0, A = round(dur·0.33), B = A + 28; phát video; mở TV07 |
| `tplay` | Phát/dừng |
| `topts` / `tclose` | Mở / đóng panel TV08 |
| `tspeed` | 0.5 / 0.75 / 1 / 1.25 (nhãn "Bình thường" ở 1x) |
| `ttoggle` | `mirror`, `loop` (bật → về A), `beat` |
| `tfilter` | Lọc khoá học |
| `tprev` / `tnext` | Bài trước / sau |
| `tretry` | Còn ngoại tuyến → toast "Vẫn chưa có mạng"; không → back |
| `tset` | `quality` vòng Tự động → 1080p → 720p; `auto`; `beat`; `tlogout` → `POST /tv/logout`, về TV01 |

### 7.4 Đăng nhập bằng mã

1. TV01 → TV02: `POST /tv/pair/start` → hiện `display_code`, QR của `qr_url` (K1), "Mã hết hạn sau mm:ss" đếm ngược từ `expires_in`.
2. Poll `POST /tv/pair/poll` mỗi `poll_interval_sec` (3 s). `pending` → tiếp; `confirmed` → lưu token, TV04, toast "Đã đăng nhập bằng {email}"; `410 pair_code_expired` hoặc hết giờ → tự `start` lại (mã mới).
3. Rời TV02 → ngừng poll.

## 8. Chuỗi có tham số

| Chuỗi | Mẫu |
|---|---|
| Video cần mạng | `Video bài {NN} cần có mạng để xem` (NN 2 chữ số) |
| Đăng nhập TV | `Đã đăng nhập bằng {email}` |
| Không tìm thấy | `Không tìm thấy khoá học “{q}”` |
| Hiển thị | `Hiển thị {n} khoá học` |
| Tải | `Tải xuống · {size}` (một bài: `Size(bytes)`; cả khoá: tổng các bài còn thiếu) |
| Tiến độ tải | `{done} / {total} · {pct}%` |
| Đếm khoá | `Tất cả {n}` (chỉ nút "Tất cả" có số, không có "·"); `Thường`, `Đặc biệt`, `Giới thiệu` không có số (mã prototype `$P:376`, Q-P3-12) |
| Bài | `Bài {NN} / {N}`, `Xong bài {NN}!` |
| Thời gian | `mm:ss` (`format.Clock`) |
| Thành viên | `Thành viên từ {dd/MM/yyyy}` |

Mọi chuỗi khác: nguyên văn, lấy từ `reference/tools/extract_strings.py`; test locale đối chiếu (07 §2).

## 9. Ca biên

- Token hết hạn giữa chừng: một lần `POST /auth/refresh` (các yêu cầu đang chờ đợi cùng kết quả), gửi lại; refresh lỗi → về 02 với thông báo "Phiên đăng nhập đã hết hạn" (Q13).
- `426 upgrade_required` → màn chặn "Cần cập nhật ứng dụng" với nút mở `store_url` (Q13).
- `409 device_limit` khi đăng nhập (K31) → banner P3 **và** sheet chọn máy (`DevicesSheet`, chế độ đăng nhập): các điện thoại của `devices[]`, mới dùng gần nhất lên đầu, chưa chọn sẵn; "Gỡ và đăng nhập" gửi lại đăng nhập kèm `replace_device_id` (server gỡ máy sau khi kiểm mật khẩu, rồi trả phiên thường). Huỷ → về form, mật khẩu còn, bấm banner để hỏi lại; `400` trên `replace_device_id` → toast "Thông tin chưa hợp lệ" và đóng sheet; mất mạng → toast, sheet còn. Sinh trắc học không đi qua `/auth/login` nên không gặp lỗi này (nếu gặp: banner + focus ô mật khẩu). Sheet thiết bị vẫn dùng cho `tv.pair.confirm` (gỡ máy cần token, Q-P6-14).
- `403 not_enrolled` khi lấy URL phát → màn 05B.
- URL media hết hạn (`expires_at`) khi đang phát → lấy URL mới, tiếp tục ở vị trí cũ.
- sha256 file tải sai → xoá file, toast "Tải lỗi, thử lại" (Q13), trạng thái về chưa tải.
- Bộ nhớ không đủ → không bắt đầu tải, toast "Không đủ bộ nhớ" (Q13).

Các chuỗi đánh dấu Q13 không có trong prototype: thêm vào `locales/vi.json` với khoá riêng, ghi trong PROGRESS "chuỗi mới chờ duyệt".
