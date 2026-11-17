# CLAUDE.md — DanceFITVN App

Tệp này hướng dẫn Claude Code trong repo **dancefitvn-app**: ứng dụng học tập cho huấn luyện viên DanceFITVN trên iPhone, Android và Apple TV, viết bằng Go trên framework **gDesk**, một codebase cho cả ba (PLAN D1).

## 1. Nguồn sự thật (theo thứ tự)

1. Lời người dùng trong phiên hiện tại.
2. **Prototype** `reference/prototype/index.html` (Night Stage, Mobile + TV): đặc tả chuẩn về giao diện và hành vi. App giống 100% về bố cục, kích thước, màu, chữ, hiệu ứng, thao tác, chuỗi — trừ K1–K16 ở `docs/plan/01-prototype-mapping.md` §3.
   - Tài liệu và mã prototype mâu thuẫn về một con số hay hành vi → **mã prototype thắng**; sửa tài liệu và ghi vào PROGRESS.
   - Mở: `cd reference/prototype && python3 -m http.server 5180` → http://localhost:5180.
3. **Hợp đồng API**: `mock/dancefitvn-mock-api/` (JSON thắng về cấu trúc) và tài liệu API v1 (thắng về quy tắc nghiệp vụ) — `docs/plan/05-data.md`.
4. `docs/plan/PLAN.md` và các tệp trong `docs/plan/`.
5. Mã và tài liệu gDesk (§2).

`reference/` và `mock/` không sửa (trừ P8.1 cho mock, có ghi BLOCKERS).

## 2. Workspace và gDesk

```
~/Working/
├── gdesk/                    gDesk gốc, nhánh main — KHÔNG sửa
├── gdesk-dancefitvn-app/     worktree gDesk, nhánh dancefitvn-app-support (https://github.com/gdeskdev/desk)
├── dancefitvn-app/           repo này
└── jplayer-mobile/  jplayer-tv/      chỉ đọc: nguồn chép engine phát, focus TV, kit chữ ($JPLAYER_MOBILE, $JPLAYER_TV)
```

- App build với `../gdesk-dancefitvn-app` (`go.mod` có `replace`, do `gdesk init -local` tạo). Module path gDesk: `github.com/gdeskdev/desk/<layer>/<module>`. CLI: `gdesk`.
- gDesk **tự vẽ, không webview**: mọi thứ là widget (`ui.Base`) vẽ bằng `ui.Painter`. iOS, tvOS, Android build với cgo. tvOS = `GOOS=ios` + `-tags tvos` (`gdesk build -os tvos`).
- Trước khi dùng API gDesk: đọc README và mã của module. **Không đoán tên API.** Thiếu → tìm tương đương hoặc ghi vào `docs/plan/06-gdesk-extensions.md`.
- Đọc trước: `../gdesk-dancefitvn-app/kits/mobile/README.md`, `kits/cli/README.md`, `framework/ui/README.md` (tvOS, video), `framework/ui/focusdir.go`, `bridge/backend/uikit/doc.go`, `bridge/backend/android/doc.go`, `foundation/ffi/README.md`, `bridge/system/README.md`, `docs/conventions.md`.
- **Học từ jPlayer trước khi làm mới**: `$JPLAYER_MOBILE` (`~/Working/jplayer-mobile`) và `$JPLAYER_TV` (`~/Working/jplayer-tv`) đã chạy ổn trên máy thật. Engine, focus TV, kit chữ, múi giờ, font, script build lấy từ đó (04 §1, 06 §1b); đọc `docs/plan/` và `CLAUDE.md` của hai repo để biết các bẫy đã gặp. Chỉ đọc, không sửa, không import.
- **Mọi sửa gDesk** chỉ trong `../gdesk-dancefitvn-app` trên nhánh `dancefitvn-app-support`, theo `../gdesk-dancefitvn-app/CLAUDE.md` và `.claude/ai/`. **Đề xuất trước** (lý do, vị trí, ảnh hưởng) và chờ chủ gDesk đồng ý; trong lúc chờ dùng dự phòng F (06). Không tạo/đổi nhánh, không sửa `../gdesk` hay worktree của app khác.

## 3. Điều phối

Phiên chính là **orchestrator** (`claude --model opus --add-dir ../gdesk-dancefitvn-app`) và làm theo `docs/plan/08-orchestration.md`:

- Không tự viết mã sản phẩm; giao từng công đoạn cho sub-agent `df-*` ở `.claude/agents/`.
- **Chế độ tự động** (08 §5) là mặc định: chạy hết các phase, chỉ dừng ở điểm dừng bắt buộc; mọi thứ cần người khác quyết định → `docs/BLOCKERS.md` + dự phòng, làm tiếp.
- Ghi trạng thái vào `docs/plan/PROGRESS.md` sau mỗi công đoạn — đó là điểm nối lại.

## 4. Quy tắc làm việc

- Làm theo **phase** P0 → P8 (PLAN §5), đúng thứ tự; phase xong khi đạt hết nghiệm thu và `df-reviewer` PASS.
- Đầu phase: đọc lại đặc tả và đoạn mã prototype liên quan. Cuối phase: test, so ảnh (07 §3), PROGRESS.
- Dữ liệu từ API (mock), không từ bộ sinh mẫu của prototype (D7).
- Không `git commit`, push, merge, tag ở repo này và `../gdesk-dancefitvn-app` khi người dùng chưa yêu cầu trong tin nhắn hiện tại.
- Ghi chú, kế hoạch, ảnh so sánh, vector: `local/` (gitignore).
- Không thêm dependency ngoài khi stdlib hoặc gDesk đủ. Engine phát là của hệ điều hành (D4); không đóng gói thư viện media.
- Không đưa nội dung thật của DanceFITVN (video, nhạc, ảnh, logo), URL thật có ký, mật khẩu, token vào repo, test, log hay ảnh chụp (05 §7).
- Mã, comment, godoc, tên test, README, commit message: **tiếng Anh**. Chuỗi hiển thị: **tiếng Việt** nguyên văn từ prototype, trong `locales/vi.json`.

## 5. Quy ước mã (theo gDesk)

- Banner 71 ký tự, 4 tiêu đề theo thứ tự `Contracts`, `Creation`, `Main functions`, `Helper Functions` — chép từ tệp gDesk.
- Lỗi bọc sentinel của `core` bằng `%w`, chữ thường, không dấu chấm cuối; `panic` chỉ trong `Must…`.
- Enum: zero là `…Unknown`; `switch` có `default`.
- Test: testify, table-driven, `t.Parallel()`, không `sleep` (`ui.Tester.Elapse`). Mỗi màn mobile có test `ui.Tester` ở cả 393×852 và 412×892 (cùng bố cục); mỗi màn TV ở 1920×1080.
- Đường nóng (mỗi khung, mỗi chạm/phím, mỗi khung video, mỗi tick thời gian) không cấp phát: benchmark + test `…DoesNotAllocate`.
- Chỉ gọi giao diện trên UI thread; từ goroutine khác: `app.Enqueue`, `window.Post`, `ui.Go`.
- Gói có build tag (`ios`, `android`, `tvos`) luôn có `_other.go` trả `core.ErrUnsupported` để `go test ./...` chạy trên Mac.
- objc theo cách `bridge/backend/uikit` dùng `foundation/ffi/objc`; release thứ đã retain. JNI qua `bridge/backend/android/jni`; xoá local ref trong vòng lặp. Các ngoại lệ cgo/objc/JNI của engine chép từ jPlayer Mobile (shim CMTime, `frames_android.c`, …) được phép **chỉ** trong `internal/media/{avf,amp}` và giữ đúng điều kiện ghi ở `$JPLAYER_MOBILE/CLAUDE.md` §5.
- Không rẽ bố cục theo hệ điều hành ở mobile (D2); khác biệt hệ thống chỉ qua API thật (safe area, Back, `SystemGestures`).

## 6. Lệnh

```sh
go test -count=1 ./...                    # toàn bộ test (driver fake)
go test -count=1 -race ./...
go test -count=1 -p 1 -tags tvos ./...        # bộ tvOS (test cấp phát và benchmark của ui/tv)
go test -count=1 -tags dancefitvndev ./...    # tag dev (menu dev, kịch bản)
go test -count=1 -tags kitdemo ./...          # màn demo của kit
go test -count=1 -tags live ./...             # tag live (không dấu mock)
gdesk build -os ios                       # dist/ios-simulator-arm64/DanceFITVN.app
gdesk build -os android                   # dist/android-arm64/DanceFITVN.apk
gdesk build -os tvos                      # dist/tvos-simulator-arm64/DanceFITVN.app
xcrun simctl boot "DanceFITVN iPhone 16"  # 393×852 @3x
emulator -avd dfv-412x892 -no-snapshot &  # 1236×2676 @480 dpi = 412×892 DIP, android-35
xcrun simctl boot "DanceFITVN Apple TV"   # Apple TV 4K
scripts/shot.sh <ID> ios|android|tvos     # ảnh + layout.json của trạng thái (07 §3.3)
go run ./cmd/contractcheck -self          # kiểm công cụ hợp đồng với mock
go run ./cmd/mockserve                    # mock qua HTTP 127.0.0.1:8787 (chế độ live/hybrid, relay ghép TV)
DFV_TAGS=live scripts/dev-build.sh ios    # bản live (-tags live); khi chạy: SIMCTL_CHILD_DFV_API=http://127.0.0.1:8787/api/v1 (hybrid thêm SIMCTL_CHILD_DFV_LIVE_ROUTES=…); xem scripts/README.md
scripts/check-livebuild.sh                # bản live không lẫn mock/marker dev, có api.DefaultBase
scripts/release.sh ios|tvos|android|all   # bản phát hành → dist/release/ (SHA256SUMS); --sim ios|tvos; doctor = còn thiếu gì để ký
scripts/perf-start.sh ios|android|tvos    # khởi động → Trang chủ (N=10); perf-scroll.sh ios|android; perf-video.sh ios|android|tvos [phút]
scripts/a11y-dump.sh ios|tvos             # cây trợ năng như VoiceOver nhận (chỉ simulator)
(cd ../gdesk-dancefitvn-app && go install ./kits/cli/cmd/gdesk)   # cài lại gdesk khi kits/cli đổi
export JAVA_HOME=/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home ANDROID_HOME=/opt/homebrew/share/android-commandlinetools
# go vet Android cần clang của NDK android-26 (frames_android.c dùng API 26)
# máy thật — biến bắt buộc, không có mặc định, không ghi UDID vào repo:
export DFV_IPHONE_UDID=... DFV_TV_UDID=... ANDROID_SERIAL=... DFV_TEAM=...   # DFV_TEAM: team ký bộ UI test Apple TV
```

## 7. "Xong" cho mỗi tính năng

1. Hành vi khớp prototype trên iPhone, Android (và Apple TV với phần TV): cùng thao tác, cùng ngưỡng, cùng chuỗi.
2. Ảnh và `layout.json` khớp prototype theo ngưỡng 07 §3.3.
3. Có test `ui.Tester` cho luồng chính; phần engine/hệ thống có test trên simulator/emulator.
4. Không cấp phát ở khung ổn định.
5. `PROGRESS.md` cập nhật.
