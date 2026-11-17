# 04 — Engine phát

## 1. Nguồn và cách chép

Engine của jPlayer Mobile đã phát MP3, MP4 và HLS trên iOS và Android bằng engine hệ thống, vẽ khung video vào `media.Surface` của app. DanceFITVN chép và rút gọn engine đó (D4):

| Nguồn (`$JPLAYER_MOBILE`, mặc định `~/Working/jplayer-mobile`) | Đích | Giữ | Bỏ |
|---|---|---|---|
| `internal/media/media.go`, `surface.go`, `doc.go` | `internal/media` | `Engine`, `Source`, `LoadOptions`, `Event`, `Surface` (3 bộ đệm) | `Prober` (không cần ảnh bìa từ khung) |
| `internal/media/fake` | `internal/media/fake` | toàn bộ | — |
| `internal/media/enginekit` | `internal/media/enginekit` | phần dùng chung | — |
| `internal/media/avf/*_ios.go`, `shim_ios.go` + `.m` | `internal/media/avf` | engine AVPlayer, `AVPlayerItemVideoOutput` → Surface, tick, session, observer | `tap_ios.go` (visualizer), `probe_ios.go` |
| `internal/media/amp/*_android.go`, `frames_android.c/.h` | `internal/media/amp` | MediaPlayer qua JNI, `AImageReader` → Surface, session (audio focus, noisy), background, wifilock | `probe_android.go`, `preflight_android.go` (nếu không cần) |

jPlayer Mobile và jPlayer TV đã chạy ổn trên máy thật — người dùng xác nhận 2026-11-14. Nguồn cho TV (`$JPLAYER_TV`, mặc định `~/Working/jplayer-tv`, bản 1.1.0 build 2):

| Nguồn jPlayer TV | Đích | Giữ | Thêm / đổi cho DanceFITVN |
|---|---|---|---|
| `internal/media/avf/*_tvos.go`, `config.go`, `state.go`, `errors.go`, `limits.go`, `interrupt*.go` | `internal/media/avftv` (`//go:build ios && tvos`) | một `AVPlayer` cả đời engine, item mới mỗi `Load`, KVO/notification, **mặt phẳng native B** (`plane_tvos.go`, `AVPlayerLayer` qua `platform.UnderlayCanvas`), giới hạn độ phân giải theo "Chất lượng video", phiên âm thanh, ngắt | engine jPlayer TV phát trực tiếp, không tua: thêm `Seek`, `SetRate` (lấy cách gọi `seekToTime`/`rate` và shim `CMTime` từ `jplayer-mobile/internal/media/avf`), sự kiện vị trí ≥ 4 Hz, `Ended`; lật gương = `AVPlayerLayer.setAffineTransform:` scale(−1, 1) |
| `internal/app/backend_tvos.go`, chế độ Metal + nền trong suốt (T4.6 của jPlayer TV) | `internal/app` | `ui.WithGPU` + `ui.WithTransparentBackground()` để video hiện dưới UI | — |
| `internal/ui/focus` | `internal/ui/tv/focus` | toàn bộ (D8) | bỏ phần riêng kênh/EPG |
| `internal/ui/kit/{label,paragraph,textshadow,glow,motion}.go`, `internal/ui/theme/faces.go` | `internal/ui/kit`, `internal/ui/theme` | giãn chữ, chiều cao dòng, bóng chữ, quầng sáng/gradient tròn vẽ CPU, chọn mặt chữ (06 §1b) | dùng cho cả mobile |
| `internal/app/tz_tvos.go` (và `jplayer-mobile/internal/app/tz_ios.go`) | `internal/app` | múi giờ thật (Go trên iOS/tvOS để `time.Local` = UTC) | cần cho lời chào, ngày `dd/MM/yyyy` |
| `scripts/tvos-device.sh`, `tvos-release.sh`, `tv-shots.sh` | `scripts/` | cài máy thật, đóng gói, chụp ảnh TV | đổi tên app/bundle |
| `assets/tvos/AppIcon.brandassets`, `PrivacyInfo.xcprivacy` | `assets/tvos/` (`DanceFITVN.brandassets`) | cấu trúc thư mục | ảnh của DanceFITVN sinh từ `docs/brand` (BRAND.md) |

Mobile vẫn dùng engine của jPlayer Mobile (bảng trên cùng §1). Hai engine chung interface `internal/media.Engine` của DanceFITVN (§4); lõi `player` không biết đang chạy engine nào.

Quy tắc chép:

- Đầu mỗi tệp: `// Adapted from jplayer-mobile/<path> at <commit>.` Đổi tên module (`jplayer-mobile/...` → `dancefitvn-app/...`).
- Không import chéo repo. Không đổi hành vi trừ phần bỏ ở bảng trên; chạy lại test đi kèm.
- Mọi ngoại lệ cgo/objc/JNI của jPlayer Mobile (CLAUDE.md §5 của repo đó) mang theo nguyên văn vào `CLAUDE.md` §5 của repo này.
- tvOS: các tệp `//go:build ios` cũng build cho tvOS (`GOOS=ios`, `-tags tvos`). API nào iOS có mà tvOS không có (ví dụ một số `AVAudioSession` option) tách ra `*_tvos.go` / `//go:build ios && !tvos`. Spike P0 xác nhận (R1).

## 2. iOS và tvOS — AVPlayer

### 2.1 Phát

- `AVPlayer` + `AVPlayerItem(URL)`; HLS dùng `master.m3u8` URL ký (`GET /lessons/{id}/stream`), MP3 dùng URL ký hoặc đường dẫn file đã tải.
- `AVAudioSession` category playback, active trước lần phát đầu (mobile và TV).
- Hết hạn URL (`expires_at`) giữa chừng: lỗi mạng 403 từ item → lấy URL mới, `Load(start = vị trí hiện tại)`.

### 2.2 Tốc độ

`AVPlayer.rate` = 0.5 / 0.75 / 1 / 1.25 / 1.5; `audioTimePitchAlgorithm = AVAudioTimePitchAlgorithmTimeDomain` (giữ cao độ khi chậm). `Play()` đặt lại rate đã chọn (AVPlayer `play` về 1.0).

### 2.3 Lật gương

Khung video đi qua `media.Surface`; widget video vẽ khung với ma trận lật ngang (`scaleX = -1` quanh tâm khung) khi bật — không đụng engine. Đo P0: không cấp phát thêm mỗi khung.

### 2.4 Now Playing (mobile và TV)

`bridge/system/nowplaying`: tiêu đề = tên bài, nghệ sĩ = tên khoá, ảnh = bìa khoá, vị trí/thời lượng/tốc độ. Lệnh (cả hai hệ): play, pause, toggle, skip ±10 s, tua tới vị trí (`SeekTo`); không có trước/sau (Q-P4-7). Android 13+ tự vẽ điều khiển thông báo và có thể không hiện tua lùi/tiến (R-P4-5). TV: `power.KeepAwake` khi phát video (`kits/mobile/README.md`, đoạn tvOS).

### 2.5 Nghe nền (mobile)

`gdesk.json` `ios.backgroundModes: ["audio"]`. Rời app khi đang ở màn video: dừng video (prototype dừng khi rời màn video). Đang nghe file âm thanh (08): tiếp tục.

## 3. Android — MediaPlayer

### 3.1 Phát

Như jPlayer Mobile: `android.media.MediaPlayer` qua `bridge/backend/android/jni` (`jni.NewProxy` cho listener), HLS bằng `setDataSource(url)`; MP3 file đã tải bằng đường dẫn.

### 3.2 Tốc độ

`setPlaybackParams(new PlaybackParams().setSpeed(r).setPitch(1))` khi đã prepared; gọi khi đang dừng sẽ phát luôn → engine nhớ rate và đặt lúc `Play()`. Đo P0 trên emulator android-35, 36.

### 3.3 Lật gương

Như §2.3 (khung qua `AImageReader` → Surface).

### 3.4 Nền và thông báo

`gdesk.json` `android.mediaPlayback: true`; dịch vụ media của gDesk chạy tiền cảnh khi `nowplaying` báo đang phát (`kits/mobile/README.md`). Nhãn nút thông báo `platform.MediaLabels` tiếng Việt.

## 4. Mô hình phát trong app

Mô hình nằm trong `internal/player` (lõi dùng chung mobile và TV, P4.3; không có gói `internal/ui/*/playback`):

| Trạng thái | Nội dung |
|---|---|
| `Lesson`, `Kind` (video/audio) | bài đang phát |
| `Pos`, `Dur`, `Playing`, `Speed` | từ sự kiện engine |
| `Mirror`, `Loop`, `A`, `B`, `Beat` | tuỳ chọn (03 §5) |
| `Token` | bỏ sự kiện của lần `Load` cũ |

Vòng lặp: sự kiện `Time` của engine (≥ 4 Hz) → cập nhật UI; `Loop && Kind==video && Pos ≥ B` → `Seek(A)`; `Ended` → màn Hết bài / Tiếp theo; lưu tiến độ theo 03 §5.

## 5. Spike P0

| Spike | Đo | Đạt khi |
|---|---|---|
| `cmd/avspike` (iOS sim, tvOS sim) | phát `mock/.../media/hls/l_3503/master.m3u8` qua máy chủ loopback; iOS: rate 0.5→1.5, vẽ khung lật; tvOS: mặt phẳng native B, `seekToTime`, rate, lật bằng `setAffineTransform:` | phát, đổi rate không giật, lật đúng chiều, 0 cấp phát ở khung ổn định |
| `cmd/ampspike` (emulator 412×892 android-35) | như trên với MediaPlayer | phát HLS; nếu HLS lỗi → ghi R3, thử MP4 progressive tạo bằng ffmpeg từ cùng nguồn |

Mock phục vụ media bằng máy chủ HTTP loopback 127.0.0.1 trong app dev (vì AVPlayer/MediaPlayer cần URL): `api/mock` mở `net/http` server trên cổng ngẫu nhiên, đổi `mock-media://x` thành `http://127.0.0.1:<port>/x`. `gdesk.json`: `ios.localNetworking: true`, `tvos.localNetworking: true`, `android.cleartextHosts: ["127.0.0.1"]` (theo `kits/cli/README.md`).
