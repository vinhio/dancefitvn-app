# 01 — Prototype → mã Go

Prototype là một tệp `reference/prototype/index.html` (≈ 821 dòng, vanilla). Số dòng dưới đây tính theo tệp đó; khi tệp đổi, tìm theo tên hàm.

Mở prototype: `cd reference/prototype && python3 -m http.server 5180`, vào http://localhost:5180. Nút trên thanh trên cùng: Cả hai / Mobile / TV, công tắc "Ngoại tuyến", "Làm lại". TV dùng phím mũi tên, Enter, Esc, Space hoặc remote dưới khung TV.

## 1. Bảng ánh xạ

| Prototype (hàm / biến, dòng) | Gói Go | Ghi chú |
|---|---|---|
| `:root` biến CSS (dòng ~8) | `internal/tokens` (sinh từ `reference/design/tokens.json`) | Test: mỗi màu token = biến CSS (D6) |
| `IC` bộ icon (223–238), `PLAY`, `PAUSE` (240–241), `ic()` (239) | `internal/ui/icon` | Path SVG 24×24, nét 1.9 (`svg.i`); vẽ bằng `ui.Painter` path |
| `DANCER()` (242) | `internal/ui/poster` | Hình vũ công: tròn đầu r 9 tại (52,17), nét 8 hoặc 7, đầu tròn |
| `PAL`, `C`, `addC`, `COURSE_ORDER` (244–256) | `internal/ui/poster/palette.go` (màu bìa); dữ liệu khoá từ mock | Bảng 02 §5; K14 |
| `SET35`, `TAGS`, `lessonsOf()` (259–267) | — | Dữ liệu từ `courses/{id}.json` (D7) |
| `sec`, `fmt` (261–262) | `internal/format` `ParseClock`, `Clock` | Vector `fmt.json` |
| `mb()` (268) | `internal/format` `Size(bytes)` | `bytes/1e6`, 1 số lẻ, dấu phẩy, " MB"; vector `size.json` |
| `key()` (269) | `model.LessonID` | `set35-3` ↔ `l_3503` (K10) |
| `initState()` (273–287) | `internal/store` (trạng thái), mock `me/*.json` | Prefs mặc định: Wi-Fi only = bật, tự phát tiếp = bật, chất lượng "Tự động" |
| `poster()` (291–299) | `internal/ui/poster.Draw` | Công thức ở 02 §5 |
| `photo()` (302–306) | `kit.Avatar` | Ảnh đại diện hoặc hình mặc định |
| `medal()` (307–310), `BADGES` (311) | `kit.Badge` | Ảnh huy hiệu từ API `badges[].image_url`; hình vẽ chỉ khi ảnh lỗi |
| `myCourses()` (312–315) | `store.Courses(filter, query)` | Lọc theo `category`, tìm theo tên + tagline không dấu |
| `SB` thanh trạng thái (322) | — | Hệ thống vẽ (K4) |
| `TABS` (323) | `kit.TabBar` | 4 tab: Trang chủ, Khoá học, Tải xuống, Tài khoản |
| `MS.*` màn mobile (327–504) | `internal/ui/mobile/screens/*` | Một tệp mỗi màn: `splash`, `login`, `loading`, `home`, `courses`, `detail`, `locked`, `video`, `audio`, `done`, `downloads`, `offline`, `account`, `info`, `coach`, `connect` |
| `continueCard()` (353) | `mobile/screens/home.ContinueCard` | |
| `courseCardH()` (371) | `kit.CourseCard` (ngang 156×100) | |
| `lesRow()` (382–393) | `kit.LessonRow` | Trạng thái: thường, `cur`, `w` (đã xem), `dim`, nút tải 4 trạng thái |
| `offbar()` (414) | `kit.OfflineBar` | |
| `SH.*` sheet (505–530) | `internal/ui/mobile/sheets/*` | `options`, `download`, `del`, `avatar`, `password`, `logout` |
| `renderM` (532), `go` (543), `back` (551) | `internal/ui/mobile/nav` | Stack, sheet, toast, giữ vị trí cuộn khi vẽ lại cùng màn |
| `toastM` (552) | `kit.Toast` | 2200 ms |
| `startDl` (558), vòng tải (776–790) | `internal/download` | Prototype tăng 3,5 %/250 ms; app tiến độ thật |
| `bindM` (559) | các màn tương ứng | Form đăng nhập, tìm kiếm, thông tin, hồ sơ HLV, mã TV, thu phóng ảnh, kéo ảnh |
| bảng hành động `A` (588–620) | `mobile/actions.go` | Bảng `act → func` giữ đúng tên (03 §3) |
| `liveM` (630–636) | `mobile/screens/video`, `audio`, `downloads` | Vị trí, thanh, sóng âm, 8 nhịp, vòng tải |
| `TS.*` màn TV (645–717) | `internal/ui/tv/screens/*` | `welcome`, `code`, `home`, `courses`, `detail`, `player`, `music`, `upnext`, `error`, `settings` |
| `tnav` (643) | `tv/tvkit.Nav` | |
| `qrSvg()` (660) | `internal/qr` + `tv/tvkit.QR` | Prototype vẽ mẫu giả; app mã hoá `qr_url` thật (K1) |
| `renderTV`, `tvGo`, `tvBack` (719–731) | `internal/ui/tv/nav` | |
| `tvToast` (732) | `tv/tvkit.Toast` | 2400 ms |
| `tvActivate` (733–750) | `tv/actions.go` | |
| `moveFocus` (751–760) | gDesk `ui.WithDirectionalFocus` | D8, K9; vector `focus.json` |
| `tvKey` (761–766) | ánh xạ phím Siri Remote | 03 §7 |
| Lớp focus và khung màn TV | `internal/ui/tv/{focus,env,nav,shell}` | `focus`: chép từ jPlayer TV (04 §1, D8); `nav`: dòng `renderTV` ở trên; `env`, `shell`: xem P7 §13 |
| `liveTV` (770–772) | `tv/screens/player`, `music` | |
| đồng hồ 250 ms + 1 s (775–791) | engine thật + `time.Ticker` UI | Đếm ngược 5 s ở màn Hết bài / Tiếp theo |
| `layout()` (795–808), công tắc ngoại tuyến (810–814) | — | Khung giả lập (K3) |

## 2. Dữ liệu: prototype ↔ API mock

| Prototype | API (mock) |
|---|---|
| `C.set35` | `GET /courses/set-35` → `courses/set-35.json` |
| `set35-3` | `l_3503` |
| `c.cat` "Thường" / "Đặc biệt" / "Giới thiệu" | `category` `regular` / `special` / `intro` + `category_label` |
| `c.quote` | `tagline` |
| `c.short` "SET 35" | suy từ `name` (bỏ "DanceFIT ", in hoa) — `model.Course.Short()` |
| `c.lessons` | `lesson_count` |
| `c.isNew` | `is_new` |
| `c.locked` | `enrolled: false` |
| `l.song`, `l.tag`, `l.dur` | `title`, `tag`, `duration_sec` |
| `S.watched[k]` | `progress.watched` |
| `S.pm.pos` khởi đầu 72 | `resume.position_sec` (72) |
| `S.dl` | trạng thái tải cục bộ (`internal/download`), không từ API |
| `S.user` | `GET /me` `user` |
| `S.coach` | `GET /me/coach-profile` |
| `S.prefs` | `internal/store` prefs (cục bộ) |
| mã TV `7K4M2P` | `tv/pair-start.json` `code`, `display_code` "7K4-M2P" |

## 3. Khác biệt bắt buộc (K)

App giống prototype trừ các điểm sau. Điểm khác nào không có ở đây là lỗi.

| ID | Khác biệt | Lý do |
|---|---|---|
| K1 | QR trên TV02 là mã QR thật của `qr_url` (prototype vẽ mẫu ngẫu nhiên 25×25, `qrSvg`) | Phải quét được |
| K2 | Bỏ các chữ chỉ có trong bản thử: "Bản thử: bất kỳ ≥ 6 ký tự", "Bản thử: mật khẩu chưa được thay đổi", "Bản thử: phát video xem thử", "Bản thử · không gửi dữ liệu đi đâu · nội dung mẫu", "DanceFITVN 1.0 (prototype)" → phiên bản thật, nút TV "Mô phỏng: xác nhận từ điện thoại" | Chỉ để giả lập |
| K3 | Không dựng khung giả lập: thanh trên cùng, khung điện thoại, khung TV, remote, công tắc ngoại tuyến (CSS của chúng liệt kê cuối 02 §4) | Không phải app |
| K4 | Thanh trạng thái (9:41, sóng, pin) và thanh home do hệ thống vẽ; app chừa `SafeArea()` | Hệ điều hành |
| K5 | Phát thật bằng AVPlayer/MediaPlayer; sân khấu minh hoạ `.vstage` (nền gradient + vũ công nhún) chỉ hiện khi video chưa có khung đầu tiên, không có vũ công | Video thật thay hình minh hoạ |
| K6 | Đăng nhập gọi `POST /auth/login`; lỗi theo mã API (`invalid_credentials`, `account_locked`, `device_limit`) thay vì kiểm "≥ 6 ký tự" | Hợp đồng API |
| K7 | Ảnh đại diện chọn bằng bộ chọn ảnh của hệ thống; "Chụp ảnh mới" cần G2 — chưa có G2 thì nút này mở bộ chọn ảnh như "Chọn từ thư viện" | gDesk chưa có camera/picker ảnh |
| K8 | TV02: mã, thời gian hết hạn và QR lấy từ `POST /tv/pair/start`, đếm ngược thật ("Mã hết hạn sau mm:ss"), tự tạo mã mới khi hết hạn | Prototype ghi cứng 04:32 |
| K9 | Focus TV theo gDesk `focusdir.go` (`ui.WithDirectionalFocus`). Đo trên 332 bước của 11 bố cục TV prototype (`local/focus-diff.md`): 331 trùng prototype, 1 khác (K9a) | D8 |
| K9a | **Đề xuất, chờ người dùng** (`docs/BLOCKERS.md`). TV05: bấm Phải ở thẻ đầu `cg0` đi tới thẻ kế `cg1` (cùng hàng); prototype nhảy lên chip `cf1` vì điểm tâm–tâm (`$P:751-760`) chọn chip hẹp ở trên hơn thẻ rộng 540 bên cạnh. App giữ cách của gDesk. Chưa có màn "sai thứ tự" nào ngoài dòng này (P7.8: không cần G6) | `focus-diff.md`, `TestScreensFollowVectors` |
| K9b | TV12: bấm Trái ở dòng cài đặt `s0`–`s5` đi tới tab đang bật `nav3` (như prototype); gDesk thuần đưa tới `nav1`. Màn TV12 ghi đè bằng mã (override cấp màn), không cần G6 | `$P:751-760`, `tv/screens/settings.go` |
| K10 | ID khác dạng (`set35` → `set-35`, `set35-3` → `l_3503`) | API |
| K11 | "Bộ nhớ trống: 18,2 GB", "Trống 18,2 GB" lấy dung lượng trống thật của máy (`internal/sys.FreeSpace`) | Thật |
| K12 | Không có màn TV03 "Đang tải" riêng; TV hiện nội dung khi dữ liệu về (mỗi màn có trạng thái chờ là nền + nav) | Prototype không có |
| K13 | Nút "Đăng nhập bằng Face ID / Vân tay" chỉ hiện khi máy có sinh trắc học đã đăng ký **và** máy này đã đăng nhập bằng mật khẩu ít nhất một lần (có phiên lưu). Bấm → hộp sinh trắc học của hệ thống → mở khoá refresh token → 03B. Android < 9 (API 28) không có `BiometricPrompt` → ẩn. Prototype luôn hiện nút | Q2 = có (2026-11-14) |
| K14 | Bài của các Set ngoài Set 35 lấy tên, nhóm, thời lượng từ mock (khác `lessonsOf` của prototype). Bìa: mode mock vẽ theo bảng màu prototype (02 §5); mode live dùng `cover_url` thật khi đã tải (`poster.SetCover`, `Services.Cover`, cache đĩa `store.Images`; máy chủ media của mock bị từ chối nên mode mock giữ bảng màu) | D7 |
| K15 | Các nút prototype chỉ hiện toast giữ chỗ ("Hẹn giờ dừng: 15 / 30 / 45 phút", "Chia sẻ / Hẹn giờ / Xoá file", "Xem dancefitvn.com để biết thêm") giữ nguyên toast đó ở 1.0 (Q10). "Quên mật khẩu?" mở `links.forgot_password` của `app/config` trong trình duyệt | Prototype là đặc tả |
| K16 | Không khác: thanh đếm 8 nhịp luôn 120 BPM như prototype (`Math.floor(pos*2)%8`), API không có `bpm` | Q9 = không (2026-11-14) |
| K17 | **Đề xuất, chờ người dùng.** Sheet và toast chỉ chạy hoạt ảnh khi mở, không phát lại mỗi lần vẽ lại; đóng không hoạt ảnh (như prototype) | Prototype phát lại hoạt ảnh do render lại DOM — lỗi phụ, không phải ý đồ (`$P:552`) |
| K18 | **Đề xuất, chờ người dùng.** Tìm khoá học bỏ dấu tiếng Việt (`đ` → `d`): "khoa" khớp "Khoá". Hiện tại làm theo mã prototype (không bỏ dấu, `$P:312-315`); đổi bằng công tắc `SearchFoldAccents` | Tài liệu 03 §4 yêu cầu bỏ dấu; mã prototype thắng tài liệu (Q-P3-11) |
| K19 | **Đề xuất, chờ người dùng.** Bỏ 🎉 trong "Bạn đã xem hết khoá học" trên cả iOS và Android | Android (gotext) không vẽ emoji màu; giữ một giao diện cho hai hệ (D2) (Q-P3-13) |
| K20 | **Đề xuất, chờ người dùng.** Dòng đầu trang chủ (thứ, ngày, lời chào sáng/chiều/tối) lấy từ đồng hồ máy; prototype ghi cứng "Thứ bảy, 14/11 · Chào buổi sáng" (`MS.home`, `$P:362`) | Đúng ngày thật, như K11 (Q-P3-15) |
| K21 | **Đề xuất, chờ người dùng.** Chọn chất lượng video (Tự động / 1080p / 720p / 480p) chỉ áp trên iOS; Android không chọn được biến thể HLS | MediaPlayer không chọn biến thể HLS (Q-P4-3) |
| K22 | **Đề xuất, chờ người dùng.** Bài trước, bài sau và tự phát bài tiếp theo giữ A/B như prototype (không tính lại). Chỉ tính lại nếu người dùng muốn | Prototype `$P:611-612,789` (Q-P4-4) |
| K23 | **Đề xuất, chờ người dùng.** Tải khi đang dùng dữ liệu di động với "Chỉ tải qua Wi‑Fi" bật: xếp hàng chờ, toast "Đang chờ Wi‑Fi để tải", tự tải khi có Wi‑Fi; prototype không có trường hợp này | Prototype không có trường hợp này |
| K24 | **Đề xuất, chờ người dùng.** M16 (Kết nối Apple TV) ẩn dòng "Mã hết hạn sau 04:18 · gợi ý: 7K4M2P" (`$P:501`). "gợi ý" là chữ phụ trợ của prototype | Điện thoại không biết mã trên TV còn hạn bao lâu (đếm ngược thật chỉ có ở TV, K8) |
| K25 | **Đề xuất, chờ người dùng.** 15D (ảnh đại diện): "Xác nhận" khi chưa chọn ảnh mới chỉ đóng sheet, không tải lên, không toast "Đã cập nhật ảnh đại diện". Prototype `avatarOk` (`$P:619`) luôn toast | Không có gì để lưu nên không báo đã lưu |
| K26 | **Đề xuất, chờ người dùng.** Năm chỗ TV có inline style đè lên khung focus của prototype (F-9): hàng `pn*` nền `#1F2125` và số màu `--pr` (`$P:688`); `oc` nền `--sf2` (`$P:693`); dòng `s*` Đăng xuất màu `--dg` (`$P:711`); chip lọc giữ nền (`$P:673`). App tái hiện đúng prototype. Đề xuất: khi focus dùng nền và màu focus (`--pr`, `--on`) ở năm chỗ này, đổi bằng công tắc `tvkit.PrototypeInlineFocus` | Prototype: inline style thắng CSS focus (`$P:181,193,199`) |
| K27 | **Đề xuất, chờ người dùng.** TV Select (Enter) kích hoạt khi nhả phím; giữ đủ 550 ms (D8) thì chỉ kích hoạt một lần, không lặp. Prototype kích hoạt ngay khi nhấn xuống (`tvKey` `$P:762`, `keydown` `$P:767`) | Tránh kích hoạt lặp hoặc nhầm khi giữ phím trên remote |
| K28 | **Đề xuất, chờ người dùng.** TV: tải dữ liệu lỗi khi ngoại tuyến (không có cache) chuyển sang màn TV11 "Mất kết nối" (`error`) thay vì để màn trống. Prototype chỉ vào TV11 khi bấm phát lúc ngoại tuyến (`playLes`, `$P:739`) hoặc bật ngoại tuyến khi đang ở player (`$P:812`) | Prototype không có trường hợp tải lỗi (PROGRESS, TV11 K28) |
| K29 | **Đề xuất, chờ người dùng** (`docs/BLOCKERS.md`). Màn 15B: hai ô Tên/Họ mỗi ô 201 px nên ô "Họ" tràn mép phải (lỗi CSS của prototype: `.field` thiếu `min-width:0`, `index.html:480/110`). Đề xuất: mỗi ô (rộng cột − 12)/2 ≈ 166,5 px. **Đã duyệt sửa 2026-11-17**: mỗi ô (rộng cột − 12)/2 (`info_parts.go`) | Prototype có lỗi CSS; mã prototype thắng |
| K30 | **Quyết định của người dùng 2026-11-17** (ghi đè prototype; trên iPhone, Android và Apple TV người dùng không bao giờ tạo được tài khoản trong app — tài khoản chỉ tạo trên website DanceFITVN, app không có nút mua/đăng ký/tham gia và không mở `links.register` hay `enroll_url`). (1) M02/M02B: bỏ dòng chân "Chưa có tài khoản? **Đăng ký trên website**" (`login.noAccount`, `login.register`); thay bằng chữ mờ thường, không bấm, không focus, căn giữa, cùng chỗ/cỡ/màu với đoạn mờ cũ (13 px, `--tx2`, cách trên 36), nhãn trợ năng là chính chữ: `login.membersOnly` = "Ứng dụng hỗ trợ cho các thành viên của DanceFITVN". (2) TV01: dòng thứ hai của `p.muted` "Dành cho huấn luyện viên đã có tài khoản dancefitvn.com" (`tv.welcome.audience`) thành `tv.welcome.membersOnly` = cùng câu trên; khối chữ hẹp hơn (rộng 755,26 thay 821,39), kiểu chữ giữ nguyên, một dòng, không cắt. (3) M05B (khoá học chưa tham gia): bỏ nút chân "Đăng ký trên website" (`locked.enroll`, mở `enroll_url`, `a.btn.p` và biểu tượng quả cầu); thay bằng chữ mờ thường căn giữa, 14 px/1,5, cùng vùng chân (`padding:16px 20px 38px`, nền `--bg`): `locked.contact` = "Liên hệ DanceFITVN để tham gia khoá học"; `locked.body` đổi từ "Đăng ký trên dancefitvn.com, khoá học sẽ tự xuất hiện trong “Khoá học của tôi”." thành "Khi tham gia khoá học, khoá học sẽ tự xuất hiện trong “Khoá học của tôi”.". Câu "Dùng tài khoản bạn đã đăng ký trên website dancefitvn.com" (`login.intro`) giữ nguyên theo người dùng. `model.Links.Register` và `model.Course.EnrollURL` vẫn nằm trong model (trường hợp đồng API), UI không mở chúng | Quyết định của người dùng (app chỉ cho thành viên đã có tài khoản; không có lời kêu gọi đăng ký/mua) |
| K31 | **Đề xuất, chờ người dùng** (đóng BLOCKERS B3 / backend Q-A1; prototype không có sheet chọn máy khi đăng nhập, `MS.login` chỉ kiểm "≥ 6 ký tự"). Đăng nhập gặp `409 device_limit` (có `devices[]`): banner "Đã đạt giới hạn thiết bị · Gỡ bớt một thiết bị để tiếp tục" vẫn hiện, **và** app mở sheet chọn máy có sẵn của `tv.pair.confirm` (`DevicesSheet`, cùng dáng và chữ: tiêu đề, dòng phụ, hàng radio, nút đỏ, "Huỷ"): liệt kê các **điện thoại** của lỗi (iOS/Android; không có TV, server chỉ gỡ máy cùng loại), mới dùng gần nhất lên đầu; **chưa chọn sẵn máy nào** (chọn là gỡ một máy), nút đỏ "Gỡ và đăng nhập" (`devices.removeAndLogin`, chuỗi mới) mờ đến khi chọn. Bấm nút → gửi lại đúng đăng nhập (email, mật khẩu của form, chỉ giữ trong bộ nhớ cho lần gửi lại, không lưu) kèm `replace_device_id` (tuỳ chọn trong body `POST /auth/login`; server chỉ xử lý sau khi mật khẩu đúng, gỡ máy rồi trả phiên bình thường) → đường thành công thường (Home). Huỷ (dấu ×, "Huỷ", nền, Back) → về form, mật khẩu còn trong ô, banner còn và bấm vào banner là hỏi lại (danh sách mới). `400 validation_failed` trên `fields.replace_device_id` (danh sách cũ): sheet đóng, toast "Thông tin chưa hợp lệ", banner còn để thử lại; mất mạng: toast "Không kết nối được, thử lại sau", sheet còn; sai mật khẩu/khoá/lỗi máy chủ: sheet đóng, banner của form. Đăng nhập sinh trắc học dùng refresh token nên không gặp `device_limit`; nếu gặp thì hiện banner, đưa focus vào ô mật khẩu (đăng nhập bằng mật khẩu sẽ mở sheet). Mock: scenario `device_limit` + body có `replace_device_id` không rỗng → trả `auth/login.json` (overlay ghi máy đã gỡ) | Người dùng không tự thoát được khi đủ giới hạn thiết bị; server có `replace_device_id` (backend Q-A1) |
| K32 | **Đề xuất, chờ người dùng** (cửa hàng: Google Play yêu cầu chính sách bảo mật mở được trong app; prototype không có hàng này). Màn 15 (Tài khoản), thẻ "Ứng dụng": thêm hàng cuối "Chính sách bảo mật" (`account.privacy`, chuỗi mới; biểu tượng `shield`, chevron, kiểu hàng như các hàng còn lại) mở `links.privacy` của `app/config` trong trình duyệt hệ thống bằng `OpenURL` (như "Quên mật khẩu?", K15). Ẩn khi `links.privacy` rỗng hoặc chưa biết cấu hình (không mạng lúc mở; màn hỏi cấu hình một lần). Chỉ ở điện thoại, **không** ở TV. Không có mục xoá tài khoản trong app (K30; `links.delete_account` không được mở). Hàng đẩy "Đăng xuất" và dòng phiên bản xuống 52 px; các trạng thái M15* so với prototype chạy với `links.privacy` rỗng (knob `noPrivacy`), M15-privacy là trạng thái có hàng | Yêu cầu cửa hàng (Google Play) |
| K34 | **Người dùng duyệt 2026-11-17** (prototype không có hàng này). Màn 15 (Tài khoản), thẻ "Ứng dụng": thêm hàng "Điều khoản sử dụng" (`account.terms`, chuỗi mới; biểu tượng `list`, chevron, kiểu hàng như K32) ngay dưới "Chính sách bảo mật", mở `links.terms` của `app/config` trong trình duyệt hệ thống bằng `OpenURL` (như K32). `links.terms` là trường thêm sau mock (mock không có, không sửa mock): ẩn khi rỗng hoặc chưa biết cấu hình, nên ở chế độ mock hàng luôn ẩn; mobile API trả mặc định `https://www.dancefitvn.com/dieu-khoan-su-dung` (đường `/dieu-khoan-su-dung` ghép với website). Hàng cuối đang hiện của thẻ không có đường kẻ (`.row:last-child`). Chỉ ở điện thoại, **không** ở TV. Các trạng thái M15* so với prototype chạy với cả `links.privacy` và `links.terms` rỗng (knob `noPrivacy`) | Quyết định chủ sản phẩm (điều khoản sử dụng trên website) |

## 4. Tương thích

- Không import mã từ `../jplayer-mobile`; chép có chỉnh các tệp engine (D4) và ghi nguồn ở đầu tệp (`// Adapted from jplayer-mobile/internal/media/avf/<file>.go`).
- Hợp đồng dữ liệu là `mock/dancefitvn-mock-api/`. Không đổi JSON mock để vừa với app; nếu mock sai, ghi `docs/BLOCKERS.md` và đề xuất sửa cả tài liệu API v1.
