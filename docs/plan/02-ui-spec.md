# 02 — Đặc tả giao diện

Mọi con số dưới đây chép từ `reference/prototype/index.html`. Khi số ở đây khác mã prototype, **mã prototype thắng** — sửa tài liệu. Kích thước tính bằng pt (iOS) = DIP (Android) = px CSS của prototype ở scale 1.

Nhiều kích thước nằm trong `style="…"` của các hàm dựng HTML (`MS.*`, `SH.*`, `TS.*`, `poster()`), không có trong CSS. Với những số đó, nguồn chuẩn là tệp `<ID>.layout.json` do `reference/tools/capture_dancefitvn.py` tạo cho từng trạng thái (hộp x, y, w, h và style tính toán của mọi phần tử có `data-*`, `.btn`, `.chip`, `.les`, `.card`…). Widget đặt `Name` bằng đúng khoá trong `layout.json` để `df-matcher` so được (07 §3).

## 1. Khung

| Mục | Mobile | TV | Nguồn |
|---|---|---|---|
| Khung | 393×852 (iPhone), 412×892 (Android, cùng bố cục, rộng hơn) | 1920×1080 | `#m`, `#tv` |
| Vùng trạng thái | 54 (hệ thống vẽ, K4); nội dung cuộn bắt đầu ở `top:54px` | — | `.sb`, `#m .scroll` |
| Lề ngang | 20 | 90 (logo, avatar), nội dung 130–160 theo màn | `#m .scroll{padding:0 20px 120px}`, `.tlogo`, `.tav` |
| Đệm cuối danh sách | 120 (chừa tab bar 84 + mini) | — | `#m .scroll` |
| Tab bar | cao 84, nền `#1A1B1F`, viền trên 1 `--bd` | — | `.tabs` |
| Safe area | `ui.Context.SafeArea()` | gDesk trả 60 trên/dưới, 80 hai bên (`framework/ui/README.md`); bố cục prototype đã nằm trong (logo x 90) | — |

## 2. Màu (token)

| Token (tokens.json) | Biến CSS | Giá trị |
|---|---|---|
| `color.bg` | `--bg` | #141518 |
| `color.surface` | `--sf` | #1F2125 |
| `color.surface2` | `--sf2` | #2A2D33 |
| `color.border` | `--bd` | #33363D |
| `color.text` | `--tx` | #F4F4F2 |
| `color.text2` | `--tx2` | #8E9196 |
| `color.text3` | `--tx3` | #5E6167 |
| `color.primary` | `--pr` | #FFD028 |
| `color.onPrimary` | `--on` | #141518 |
| `color.red` | `--red` | #E5383B |
| `color.success` | `--ok` | #3DD68C |
| `color.warning` | `--warn` | #FFB020 |
| `color.danger` | `--dg` | #FF5A5F |
| `color.info` | `--info` | #4DA3FF |
| `color.scrim` | `.dim-bg` | rgba(0,0,0,.55) |

Màu dùng trực tiếp trong CSS/HTML (không qua biến) — giữ đúng: tab bar `#1A1B1F`; toast nền `#F4F4F2` chữ `#141518`; chip "Thường" nền rgba(229,56,59,.18) chữ `#FF6B6E`; pill tab đang chọn rgba(255,208,40,.16); hàng bài đang xem rgba(255,208,40,.12); thanh ngoại tuyến rgba(255,176,32,.16); nền ô cắt ảnh `#2A2D33`.

Bảng `tokens.json` có thêm bộ màu sáng (`[1]`): không dùng ở 1.0 (Q1).

## 3. Chữ

| Kiểu | Font | Cỡ / đậm / giãn | Nguồn |
|---|---|---|---|
| Tiêu đề màn | Barlow Condensed | 34 / 700 / 1, in hoa, line-height 1 | `.h1` |
| Hiển thị chung | Barlow Condensed | 700, giãn .6, in hoa | `.d` (cỡ theo chỗ dùng: 18 mục, 22 sheet, 24 hộp thoại, 30 ô mã, 72–150 TV) |
| Nhãn nhỏ (eyebrow) | Inter | 11 / 700 / 1.4, in hoa, `--tx2` | `.eb` |
| Nút | Inter | 15 / 800 / 1, in hoa (nút p), 600 không in hoa (g, s) | `.btn` |
| Chip | Inter | 11 / 700 | `.chip` |
| Dòng bài | Inter | tên 15/600, phụ 12 `--tx2` | `.les .tt` |
| Ô nhập | Inter | 15, nhãn 11/700/1.2 in hoa | `.inp input`, `.field label` |
| Số thời gian | JetBrains Mono | theo chỗ dùng (11–12 mobile, 26–30 TV) | `.mono` |
| TV nav | Inter | 28 / 500, đang chọn 700 | `.tnav .f` |
| TV nút | Inter | 30 / 700 | `.tbtn` |
| TV dòng | Inter | tên 30/700, phụ 22 (độ mờ .7) | `.trow .tt` |

## 4. Chuyển động

| Hiệu ứng | Thời lượng / easing | Nguồn |
|---|---|---|
| Sheet trượt lên | 250 ms ease-out, từ translateY(100%) | `.sheet` `@keyframes su` |
| Nền mờ, toast, panel TV hiện | 200 ms, opacity 0 → 1 | `@keyframes fi` |
| Công tắc | 200 ms | `.sw` |
| Nút bấm | scale .98 khi nhấn | `.btn:active` |
| Focus TV | 150 ms transform + shadow; scale 1.06, viền 6 `--pr`, bóng 0 26 50 rgba(0,0,0,.6) | `#tv .f.focus` |
| Focus TV nav | scale 1.04, nền `--pr`, không bóng | `#tv .tnav .f.focus` |
| Focus TV dòng | scale 1.02, nền `--pr`, bóng 0 20 40 rgba(0,0,0,.5) | `#tv .trow.focus` |
| Focus TV tuỳ chọn | không scale, nền `--pr` | `#tv .topt.focus` |
| Lật gương video minh hoạ | 300 ms | `.vstage .who` |
| Tự ẩn điều khiển video | opacity 1 → 0 trong 250 ms, sau 3000 ms không chạm (03 §5a) | `.vui`, `.scr.vid.hid .vui`, `CTL_HIDE_MS` |

`tokens.json`: `motion.fastMs` 150, `motion.baseMs` 250, `motion.tvFocusScale` 1.06 — trùng các số trên.

## 5. Bìa khoá học (`poster(cid, w, h, o)`)

- Khung: bo góc `o.r` (mặc định 16), nền gradient 135° từ `g[0]` tới `g[1]`.
- `m = min(w, h)`, cỡ tiêu đề `ts = o.ts || min(h·0.2, w·0.14)`.
- Vòng tròn: đường kính `m·1.1`, phải `−m·0.25`, trên `−m·0.2`, viền `m·0.07` màu rgba(255,255,255,.13).
- Khoá đĩa than (`vinyl`): đĩa đường kính `r = m·0.7`, tâm tại (w·0.62, h·0.42); tâm cam `#FF6A1A` 0–14 %, nền `#1a1a1a`, 2 vành `#3a3a3a`.
- Khoá thường: 2 vũ công (cả hai không lật: `DANCER` nhận hai thuộc tính `style` trùng ở `$P:242`/`$P:295`, trình phân tích giữ thuộc tính đầu nên `scaleX(-1)` bị bỏ), cao `fh = min(h·(h<130 ? 0.62 : 0.72), w·(o.title===false ? 0.42 : 0.3))`, đặt phải `w·0.04`, dưới `h·0.06`, độ mờ .92, vũ công thứ hai lùi trái `fh·0.25`, màu `ink`.
- Tiêu đề: trái 8 %, dưới 8 %; dòng "DANCEFIT" cỡ `ts·0.42`, Barlow 700, giãn 1.5, mờ .85; dòng tên (`short` bỏ "DANCEFIT ") cỡ `ts`, Barlow 800, line-height .95, bóng chữ 0 2 12 rgba(0,0,0,.18).
- **Mode mock**: luôn vẽ bìa bằng công thức trên với bảng màu dưới đây (ảnh `cover_url` của mock là ảnh giả "MOCK COVER", không dùng), để ảnh app khớp prototype.
- **Mode live**: có `cover_url` đã tải → vẽ ảnh phủ khung (cover), giữ bo góc, không vẽ vòng/vũ công/tiêu đề; ảnh chưa tải hoặc lỗi → công thức; khoá lạ không có trong bảng → gradient `--sf2` → `--bg`. Q11 xác nhận ảnh bìa thật.

Bảng màu bìa (chép từ `addC` trong prototype; `internal/ui/poster/palette.go`):

| Prototype → API id | Gradient 135° | Mực (`ink`) | Hình | Tiêu đề ngắn |
|---|---|---|---|---|
| `set35` → `set-35` | #FF4FB8 → #9E1470 | #fff | vũ công | SET 35 |
| `set34` → `set-34` | #3CC3D6 → #0B5466 | #fff | vũ công | SET 34 |
| `hotextra` → `hot-extra` | #FFF1EE → #F8C9CC | #8A1424 | đĩa than | HOT EXTRA |
| `set33` → `set-33` | #2FB8C8 → #0E6E80 | #fff | vũ công | SET 33 |
| `set32` → `set-32` | #F23D6E → #A30E3A | #fff | vũ công | SET 32 |
| `set31` → `set-31` | #4B2A86 → #E8A23A | #fff | vũ công | SET 31 |
| `set30` → `set-30` | #FF7A2F → #B3261E | #fff | vũ công | SET 30 |
| `set29` → `set-29` | #1E6FE0 → #22B8CF | #fff | vũ công | SET 29 |
| `set28` → `set-28` | #7A3FD1 → #E5383B | #fff | vũ công | SET 28 |
| `set27` → `set-27` | #1F7A4D → #5BC489 | #fff | vũ công | SET 27 |
| `set26` → `set-26` | #C2185B → #FF8A65 | #fff | vũ công | SET 26 |
| `set25` → `set-25` | #0D47A1 → #26C6DA | #fff | vũ công | SET 25 |
| `set24` → `set-24` | #AD1457 → #F06292 | #fff | vũ công | SET 24 |
| `hotextra2` → `hot-extra-2` | #FFF1EE → #F8C9CC | #8A1424 | đĩa than | HOT EXTRA 2 |
| `intro` → `intro` | #2E2E35 → #E5383B | #fff | vũ công | GIỚI THIỆU |
| `set36` → `set-36` | #3A3D44 → #16171A | #fff | vũ công | SET 36 |

## 6. Thành phần và trạng thái

Mỗi thành phần có các trạng thái sau; ảnh tham chiếu ở `reference/prototype/screenshots/`.

| Thành phần | CSS | Trạng thái |
|---|---|---|
| Button | `.btn` `.p .g .s .r .t` | thường, nhấn (.98), tắt (mờ .45 — `MS.connect` khi mã < 6 ký tự, ảnh M16) |
| Chip | `.chip` `.c-th .c-db .c-gt .y .k .okc` | — |
| Ô nhập | `.inp` | thường, focus (viền `--pr` 1.6), lỗi `.err` (viền `--dg` 1.6 + `.help`) |
| Dòng cài đặt | `.row` | có icon `.ico` 36×36 r10, giá trị `.val`, công tắc `.sw` 51×31 |
| Công tắc | `.sw` | tắt, bật (nền `--pr`, núm `--on`, núm 24, x 3 → 23) |
| Phân đoạn | `.segc` (sheet), `.seg` (lọc khoá học dùng chip) | chọn: nền `--pr` chữ `--on` 700 |
| Radio | `.radio` | chọn: viền 1.6 `--pr`, chấm 10 |
| Dòng bài | `.les` | thường, `.cur` (đang xem), `.w` (đã xem, số mờ), `.dim` (.38, ngoại tuyến không có file), nút tải `.dlb`: chưa tải, đang tải (vòng `stroke-dasharray` 94.2), đã tải (`--ok`), tắt |
| Sheet | `.sheet` + `.dim-bg` | bo trên 24, đệm 14/20/40, tay nắm 36×5, tối đa 92 % chiều cao |
| Toast | `.toast` | trái/phải 16, dưới 100, r14, đệm 13/16 |
| Thanh ngoại tuyến | `.offbar` | dưới vùng trạng thái, cao 40 |
| Tab bar | `.tabs` | pill 56×30 r15 của tab đang chọn |
| Ô mã | `.codebox` | 46×60 r12, đang nhập `.f` viền 1.8 `--pr` |
| Huy hiệu | `.badge` | — |
| Video: thanh tua | `.scrub` | rãnh 4, đã chạy `--pr`, đoạn A–B rgba(229,56,59,.7) cao 8, núm 14 |
| Video: nút bên | `.vside` | 46×46 tròn rgba(0,0,0,.45); bật: `--pr` |
| 8 nhịp | `.beats` | ô cao 24 r6; nhịp hiện tại nền `--red` |
| Sóng âm | `.wave` | cao 40, khe 2; phần đã nghe `--pr` |
| Cắt ảnh | `.crop` | khung 230 r16, ảnh 260×260, mặt nạ tròn 200 viền 2.5 trắng |
| TV nút | `.tbtn` | cao 88, r44, đệm 44; focus nền `--pr` |
| TV dòng | `.trow` | cao 88, r22; số 56 tròn, focus số nền `--red` |
| TV panel | `.tpanel` | phải 60, trên/dưới 60, rộng 640, r36, đệm 44 |
| TV tuỳ chọn | `.topt` | cao 74, r18 |
| TV toast | `.ttoast` | dưới 60, r24, đệm 22/34, chữ 28/700 |

## 7. Toàn bộ CSS của prototype

Sinh bằng `python3 reference/tools/css_spec.py`; chạy lại khi prototype đổi.

### Tokens and base

| Selector | Declarations |
|---|---|
| `:root` | --bg:#141518; --sf:#1F2125; --sf2:#2A2D33; --bd:#33363D; --tx:#F4F4F2; --tx2:#8E9196; --tx3:#5E6167; --pr:#FFD028; --on:#141518; --red:#E5383B; --ok:#3DD68C; --warn:#FFB020; --dg:#FF5A5F; --info:#4DA3FF;  --disp:'Barlow Condensed','Arial Narrow',sans-serif; --body:Inter,-apple-system,'Segoe UI',Roboto,sans-serif; --mono:'JetBrains Mono',ui-monospace,monospace |
| `#m` | position:absolute; left:8px; top:8px; width:393px; height:852px; border-radius:50px; overflow:hidden; background:var(--bg) |
| `#tv` | position:relative; width:1920px; height:1080px; overflow:hidden; background:var(--bg); border-radius:6px |

### Shared atoms

| Selector | Declarations |
|---|---|
| `.d` | font-family:var(--disp); font-weight:700; text-transform:uppercase; letter-spacing:.6px; line-height:1 |
| `.eb` | font-size:11px; font-weight:700; letter-spacing:1.4px; text-transform:uppercase; color:var(--tx2) |
| `.mono` | font-family:var(--mono) |
| `.acc` | color:var(--pr) |
| `.muted` | color:var(--tx2) |
| `.chip` | display:inline-flex; align-items:center; gap:5px; height:22px; padding:0 9px; border-radius:11px; font-size:11px; font-weight:700; white-space:nowrap |
| `.chip.c-th` | background:rgba(229,56,59,.18); color:#FF6B6E |
| `.chip.c-db` | background:rgba(77,163,255,.18); color:var(--info) |
| `.chip.c-gt` | background:rgba(61,214,140,.16); color:var(--ok) |
| `.chip.y` | background:var(--pr); color:var(--on) |
| `.chip.k` | background:rgba(0,0,0,.55); color:#fff |
| `.chip.okc` | background:rgba(61,214,140,.16); color:var(--ok) |
| `.btn` | display:flex; align-items:center; justify-content:center; gap:8px; height:54px; border-radius:14px; font-weight:800; font-size:15px; letter-spacing:1px; text-transform:uppercase; width:100% |
| `.btn.p` | background:var(--pr); color:var(--on) |
| `.btn.g` | border:1.2px solid var(--bd); text-transform:none; letter-spacing:0; font-weight:600 |
| `.btn.s` | background:var(--sf2); text-transform:none; letter-spacing:0; font-weight:600 |
| `.btn.r` | background:var(--dg); color:#fff |
| `.btn.t` | color:var(--pr); text-transform:none; letter-spacing:0; font-weight:700 |
| `.btn:active` | transform:scale(.98) |
| `svg.i` | width:22px; height:22px; stroke:currentColor; fill:none; stroke-width:1.9; stroke-linecap:round; stroke-linejoin:round; flex:none |
| `.poster` | position:relative; overflow:hidden; border-radius:16px; flex:none |
| `.poster .ring` | position:absolute; border-radius:50%; border:solid rgba(255,255,255,.13) |
| `.poster .ttl` | position:absolute; left:8%; bottom:8%; color:var(--ink,#fff); z-index:2; text-shadow:0 2px 12px rgba(0,0,0,.18) |
| `.poster .ttl b` | display:block; font-family:var(--disp); font-weight:800; text-transform:uppercase; line-height:.95 |
| `.poster .ttl s` | text-decoration:none; display:block; font-family:var(--disp); font-weight:700; letter-spacing:1.5px; opacity:.85 |
| `.poster svg.dn` | position:absolute; bottom:6%; height:72% |
| `.vinyl` | position:absolute; border-radius:50%; background:radial-gradient(circle,#FF6A1A 0 14%,#1a1a1a 14.5% 100%); box-shadow:inset 0 0 0 6px #1a1a1a,inset 0 0 0 7px #3a3a3a,inset 0 0 0 16px #1a1a1a,inset 0 0 0 17px #3a3a3a |

### Mobile

| Selector | Declarations |
|---|---|
| `#m .scr` | position:absolute; inset:0; overflow:hidden |
| `#m .scroll` | position:absolute; left:0; right:0; top:54px; bottom:0; overflow-y:auto; scrollbar-width:none; padding:0 20px 120px |
| `#m .scroll::-webkit-scrollbar` | display:none |
| `.sb` | position:absolute; left:0; right:0; top:0; height:54px; display:flex; align-items:center; justify-content:space-between; padding:6px 30px 0 34px; font-weight:600; font-size:16px; z-index:40; pointer-events:none |
| `.sb .ic` | display:flex; gap:6px; align-items:center |
| `.hi` | position:absolute; left:50%; bottom:8px; width:134px; height:5px; margin-left:-67px; border-radius:3px; background:var(--tx); z-index:60; pointer-events:none |
| `.tabs` | position:absolute; left:0; right:0; bottom:0; height:84px; background:#1A1B1F; border-top:1px solid var(--bd); display:flex; z-index:30 |
| `.tabs button` | flex:1; display:flex; flex-direction:column; align-items:center; gap:5px; padding-top:8px; font-size:11px; color:var(--tx2); font-weight:500 |
| `.tabs button .pill` | width:56px; height:30px; border-radius:15px; display:flex; align-items:center; justify-content:center |
| `.tabs button.on` | color:var(--pr); font-weight:700 |
| `.tabs button.on .pill` | background:rgba(255,208,40,.16) |
| `.top` | display:flex; align-items:center; justify-content:space-between; height:52px |
| `.rb` | width:40px; height:40px; border-radius:50%; background:var(--sf); display:flex; align-items:center; justify-content:center; flex:none |
| `.rb.dk` | background:rgba(0,0,0,.4); color:#fff |
| `.card` | background:var(--sf); border-radius:16px |
| `.h1` | font-family:var(--disp); font-weight:700; font-size:34px; text-transform:uppercase; letter-spacing:1px; line-height:1 |
| `.sec` | display:flex; justify-content:space-between; align-items:baseline; margin:22px 0 12px |
| `.sec .d` | font-size:18px; letter-spacing:1px |
| `.sec a` | font-size:13px; color:var(--pr); font-weight:600; cursor:pointer |
| `.les` | display:flex; align-items:center; gap:12px; min-height:58px; border-bottom:1px solid var(--bd); cursor:pointer; position:relative |
| `.les .no` | width:36px; height:36px; border-radius:50%; border:1.5px solid var(--bd); display:flex; align-items:center; justify-content:center; font-weight:800; font-size:13px; flex:none |
| `.les.cur` | background:rgba(255,208,40,.12); margin:0 -8px; padding:0 8px; border-radius:12px; border-bottom-color:transparent |
| `.les.cur .no` | background:var(--pr); border-color:var(--pr); color:var(--on) |
| `.les.w .no` | color:var(--tx3) |
| `.les .tt` | flex:1; min-width:0 |
| `.les .tt b` | display:block; font-size:15px; font-weight:600; white-space:nowrap; overflow:hidden; text-overflow:ellipsis |
| `.les .tt span` | font-size:12px; color:var(--tx2) |
| `.les.cur .tt span` | color:var(--pr) |
| `.les .dlb` | width:34px; height:34px; display:flex; align-items:center; justify-content:center; border-radius:50%; color:var(--tx3); position:relative |
| `.les .dlb.on` | color:var(--ok) |
| `.les .dlb .pr` | position:absolute; inset:3px |
| `.les.dim` | opacity:.38 |
| `.field` | margin-bottom:16px |
| `.field label` | display:block; font-size:11px; font-weight:700; letter-spacing:1.2px; text-transform:uppercase; color:var(--tx2); margin-bottom:8px |
| `.field label em` | color:var(--dg); font-style:normal |
| `.inp` | display:flex; align-items:center; gap:10px; height:54px; border-radius:14px; background:var(--sf); border:1px solid var(--bd); padding:0 14px; color:var(--tx2) |
| `.inp:focus-within` | border-color:var(--pr); border-width:1.6px |
| `.inp.err` | border-color:var(--dg); border-width:1.6px |
| `.inp input` | flex:1; background:none; border:0; outline:0; font-size:15px; color:var(--tx); min-width:0 |
| `.inp input::placeholder` | color:var(--tx3) |
| `.help` | font-size:12px; margin-top:7px; color:var(--dg); display:flex; gap:6px; align-items:center |
| `.row` | display:flex; align-items:center; gap:12px; min-height:52px; border-bottom:1px solid var(--bd); cursor:pointer |
| `.row:last-child` | border-bottom:0 |
| `.row .ico` | width:36px; height:36px; border-radius:10px; background:var(--sf2); display:flex; align-items:center; justify-content:center; flex:none |
| `.row .tt` | flex:1 |
| `.row .tt b` | display:block; font-size:15px; font-weight:600 |
| `.row .tt span` | font-size:12px; color:var(--tx2) |
| `.row .val` | font-size:14px; color:var(--tx2) |
| `.sw` | width:51px; height:31px; border-radius:16px; background:var(--sf2); border:1px solid var(--bd); position:relative; flex:none; transition:.2s |
| `.sw:after` | content:''; position:absolute; top:2.5px; left:3px; width:24px; height:24px; border-radius:50%; background:#fff; transition:.2s |
| `.sw.on` | background:var(--pr); border-color:var(--pr) |
| `.sw.on:after` | left:23px; background:var(--on) |
| `.segc` | display:flex; background:var(--sf); border:1px solid var(--bd); border-radius:22px; padding:3px |
| `.segc button` | flex:1; height:34px; border-radius:17px; font-size:13px; color:var(--tx2); font-weight:500 |
| `.segc button.on` | background:var(--pr); color:var(--on); font-weight:700 |
| `.dim-bg` | position:absolute; inset:0; background:rgba(0,0,0,.55); z-index:70; animation:fi .2s |
| `.sheet` | position:absolute; left:0; right:0; bottom:0; background:var(--sf); border-radius:24px 24px 0 0; z-index:71; padding:14px 20px 40px; animation:su .25s ease-out; max-height:92%; overflow-y:auto |
| `.sheet .grab` | width:36px; height:5px; border-radius:3px; background:var(--tx3); margin:0 auto 14px |
| `.sheet .sh` | display:flex; justify-content:space-between; align-items:center; margin-bottom:16px |
| `.sheet .sh .d` | font-size:22px; letter-spacing:1px |
| `@keyframes su` | from{transform:translateY(100%)} |
| `@keyframes fi` | from{opacity:0} |
| `.toast` | position:absolute; left:16px; right:16px; bottom:100px; background:#F4F4F2; color:#141518; border-radius:14px; padding:13px 16px; font-size:13px; font-weight:600; z-index:90; display:flex; gap:10px; align-items:center; animation:fi .2s; box-shadow:0 10px 30px rgba(0,0,0,.4) |
| `.offbar` | position:absolute; left:0; right:0; top:54px; height:40px; background:rgba(255,176,32,.16); color:var(--warn); display:flex; align-items:center; gap:8px; padding:0 20px; font-size:13px; font-weight:700; z-index:35 |
| `.radio` | display:flex; gap:12px; align-items:center; padding:12px 14px; border-radius:14px; border:1px solid var(--bd); margin-bottom:10px; cursor:pointer; background:var(--bg) |
| `.radio.on` | border:1.6px solid var(--pr) |
| `.radio .dot` | width:20px; height:20px; border-radius:50%; border:2px solid var(--tx3); flex:none; position:relative |
| `.radio.on .dot` | border-color:var(--pr) |
| `.radio.on .dot:after` | content:''; position:absolute; inset:3px; border-radius:50%; background:var(--pr) |
| `.prog` | height:5px; border-radius:3px; background:var(--sf2); overflow:hidden |
| `.prog i` | display:block; height:100%; background:var(--pr); border-radius:3px |

### Video stage

| Selector | Declarations |
|---|---|
| `.vstage` | position:absolute; overflow:hidden; container-type:size; background:linear-gradient(160deg,#3A1A1C,#0E0E10) |
| `.vstage:before` | content:''; position:absolute; inset:0; background:radial-gradient(ellipse at 50% 35%,rgba(255,208,40,.32),transparent 60%) |
| `.vstage .floor` | position:absolute; left:0; right:0; bottom:0; height:28%; background:rgba(0,0,0,.4) |
| `.vstage .wall` | position:absolute; inset:0 0 28% 0; background:repeating-linear-gradient(90deg,transparent 0 15.5%,rgba(255,255,255,.05) 15.5% 16%) |
| `.vstage .who` | position:absolute; left:50%; bottom:25%; transform:translateX(-50%); height:min(42%,75cqw); transition:transform .3s |
| `.vstage .who svg` | height:100%; animation:bop 1s ease-in-out infinite alternate; animation-play-state:paused; transform-origin:50% 100% |
| `.vstage.play .who svg` | animation-play-state:running |
| `.vstage.mir .who` | transform:translateX(-50%) scaleX(-1) |
| `@keyframes bop` | 0%{transform:rotate(-6deg) translateY(0)}50%{transform:rotate(0) translateY(-4%)}100%{transform:rotate(6deg) translateY(0)} |

### Auto-hide of the video controls (q12): 3 s after the last touch while playing

| Selector | Declarations |
|---|---|
| `.vui` | transition:opacity .25s |
| `.scr.vid.hid .vui` | opacity:0; pointer-events:none |
| `.vctrl` | position:absolute; left:0; right:0; bottom:0; padding:0 20px 34px; z-index:10; background:linear-gradient(transparent,rgba(0,0,0,.85) 35%) |
| `.scrub` | position:relative; height:18px; cursor:pointer |
| `.scrub .tr` | position:absolute; left:0; right:0; top:7px; height:4px; border-radius:2px; background:rgba(255,255,255,.3) |
| `.scrub .fl` | position:absolute; left:0; top:7px; height:4px; border-radius:2px; background:var(--pr) |
| `.scrub .ab` | position:absolute; top:5px; height:8px; background:rgba(229,56,59,.7); border-radius:2px |
| `.scrub .kn` | position:absolute; top:2px; width:14px; height:14px; margin-left:-7px; border-radius:50%; background:var(--pr) |
| `.vside` | position:absolute; right:14px; top:300px; display:flex; flex-direction:column; gap:16px; z-index:10 |
| `.vside button` | display:flex; flex-direction:column; align-items:center; gap:4px; font-size:10px; font-weight:700; color:#fff |
| `.vside button i` | width:46px; height:46px; border-radius:50%; background:rgba(0,0,0,.45); display:flex; align-items:center; justify-content:center; font-style:normal |
| `.vside button.on i` | background:var(--pr); color:var(--on) |
| `.beats` | display:flex; gap:5px |
| `.beats b` | flex:1; height:24px; border-radius:6px; background:rgba(255,255,255,.12); display:flex; align-items:center; justify-content:center; font-size:11px; font-weight:800; color:rgba(255,255,255,.6) |
| `.beats b.on` | background:var(--red); color:#fff |
| `.wave` | display:flex; align-items:center; gap:2px; height:40px; cursor:pointer |
| `.wave i` | flex:1; border-radius:2px; background:var(--sf2) |
| `.wave i.on` | background:var(--pr) |
| `.crop` | position:relative; height:230px; border-radius:16px; background:#2A2D33; overflow:hidden; cursor:grab; touch-action:none |
| `.crop .img` | position:absolute; left:50%; top:50%; width:260px; height:260px; margin:-130px 0 0 -130px; background-size:cover; background-position:center |
| `.crop .mask` | position:absolute; inset:0; pointer-events:none; background:radial-gradient(circle at 50% 50%,transparent 99px,rgba(0,0,0,.55) 100px) |
| `.crop .mask:after` | content:''; position:absolute; left:50%; top:50%; width:200px; height:200px; margin:-100px; border:2.5px solid #fff; border-radius:50% |
| `input[type=range]` | -webkit-appearance:none; appearance:none; width:100%; height:6px; border-radius:3px; background:var(--sf2); outline:none |
| `input[type=range]::-webkit-slider-thumb` | -webkit-appearance:none; width:20px; height:20px; border-radius:50%; background:var(--red); cursor:pointer |
| `input[type=range]::-moz-range-thumb` | width:20px; height:20px; border-radius:50%; background:var(--red); border:0 |
| `.badge` | display:flex; flex-direction:column; align-items:center; gap:8px; background:var(--sf); border-radius:16px; padding:10px 6px 12px; flex:1 |
| `.badge b` | font-size:10px; font-weight:800; letter-spacing:.6px; text-align:center |
| `.codebox` | width:46px; height:60px; border-radius:12px; background:var(--sf); border:1px solid var(--bd); display:flex; align-items:center; justify-content:center; font-family:var(--disp); font-weight:700; font-size:30px |
| `.codebox.f` | border:1.8px solid var(--pr) |

### Tv

| Selector | Declarations |
|---|---|
| `#tv .scr` | position:absolute; inset:0 |
| `#tv .f` | transition:transform .15s,box-shadow .15s; cursor:pointer |
| `#tv .f.focus` | transform:scale(1.06); box-shadow:0 0 0 6px var(--pr),0 26px 50px rgba(0,0,0,.6); z-index:5; position:relative |
| `#tv .f.focus.flat` | transform:none; box-shadow:none |
| `.tnav` | position:absolute; top:34px; left:50%; transform:translateX(-50%); display:flex; gap:12px; padding:8px; border-radius:40px; background:rgba(255,255,255,.06); z-index:10 |
| `.tnav .f` | padding:12px 32px; border-radius:30px; font-size:28px; color:var(--tx2); font-weight:500 |
| `.tnav .f.on` | background:var(--sf2); color:var(--tx); font-weight:700 |
| `#tv .tnav .f.focus` | background:var(--pr); color:var(--on); box-shadow:none; transform:scale(1.04) |
| `.tlogo` | position:absolute; left:90px; top:56px; font-family:var(--disp); font-weight:800; font-size:40px; color:var(--pr); letter-spacing:1px; z-index:10 |
| `.tav` | position:absolute; right:90px; top:42px; width:60px; height:60px; border-radius:50%; background:var(--pr); color:var(--on); font-weight:800; font-size:26px; display:flex; align-items:center; justify-content:center; z-index:10 |
| `.tbtn` | display:inline-flex; align-items:center; gap:14px; height:88px; padding:0 44px; border-radius:44px; background:rgba(255,255,255,.12); font-size:30px; font-weight:700 |
| `#tv .tbtn.focus` | background:var(--pr); color:var(--on); box-shadow:0 26px 50px rgba(0,0,0,.6) |
| `.tbtn svg.i` | width:34px; height:34px |
| `.trow` | display:flex; align-items:center; gap:26px; height:88px; padding:0 30px; border-radius:22px; background:var(--sf); margin-bottom:12px |
| `#tv .trow.focus` | background:var(--pr); color:var(--on); box-shadow:0 20px 40px rgba(0,0,0,.5); transform:scale(1.02) |
| `.trow .no` | width:56px; height:56px; border-radius:50%; border:3px solid var(--bd); display:flex; align-items:center; justify-content:center; font-weight:800; font-size:24px; flex:none |
| `#tv .trow.focus .no` | background:var(--red); border-color:var(--red); color:#fff |
| `.trow .tt` | flex:1 |
| `.trow .tt b` | display:block; font-size:30px; font-weight:700 |
| `.trow .tt span` | font-size:22px; opacity:.7 |
| `.tpanel` | position:absolute; right:60px; top:60px; bottom:60px; width:640px; background:var(--sf); border-radius:36px; padding:44px; z-index:20; box-shadow:0 30px 80px rgba(0,0,0,.6); animation:fi .2s |
| `.topt` | display:flex; align-items:center; justify-content:space-between; height:74px; padding:0 26px; border-radius:18px; font-size:30px; font-weight:600; margin-bottom:6px |
| `#tv .topt.focus` | background:var(--pr); color:var(--on); box-shadow:none; transform:none |
| `.ttoast` | position:absolute; left:50%; bottom:60px; transform:translateX(-50%); background:#F4F4F2; color:#141518; border-radius:24px; padding:22px 34px; font-size:28px; font-weight:700; z-index:90; animation:fi .2s; white-space:nowrap |

Simulator chrome, not built (K3): `*`, `html,body`, `button`, `input,textarea`, `.app`, `.bar`, `.brand`, `.brand small`, `.seg`, `.seg button`, `.seg button.on`, `.tog`, `.tog i`, `.tog i:after`, `.tog.on i`, `.tog.on i:after`, `.reset`, `.note`, `.stage`, `.col`, `.cap`, `.holder`, `.phone`, `.phone .island`, `.tvbox`, `.tvframe`, `.tvstand`, `.remote`, `.remote button`, `.remote button:hover`, `.remote span`
