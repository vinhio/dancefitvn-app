# Bộ kế hoạch DanceFITVN App (Mobile + Apple TV trên gDesk)

Bộ tài liệu để Claude Code xây **DanceFITVN cho iPhone, Android và Apple TV** trên gDesk, trong codebase `dancefitvn-app`, giống 100% prototype Night Stage, chạy trên bộ JSON mock cho tới khi backend gFly sẵn sàng. Làm xong các bước dưới, bạn chỉ gửi **một prompt**.

## Bố cục workspace

```
~/Working/
├── gdesk/                    gDesk gốc, nhánh main (không sửa)
├── gdesk-dancefitvn-app/     worktree gDesk, nhánh dancefitvn-app-support  ← mọi sửa gDesk cho app này
├── dancefitvn-app/           repo app (bộ kế hoạch này chép vào đây)
└── jplayer-mobile/  jplayer-tv/      chỉ đọc — nguồn chép engine phát, focus TV, kit chữ
```

## Nội dung

```
CLAUDE.md                          quy tắc cho Claude Code trong repo dancefitvn-app
docs/plan/PLAN.md                  mục tiêu, quyết định D1–D11, kiến trúc, 9 phase P0–P8, rủi ro R1–R8, câu hỏi Q1–Q13
docs/plan/01-prototype-mapping.md  prototype → gói Go, dữ liệu prototype ↔ API, khác biệt K1–K16
docs/plan/02-ui-spec.md            khung, màu, chữ, chuyển động, bìa khoá học, thành phần, toàn bộ CSS
docs/plan/03-behavior-spec.md      điều hướng, ngoại tuyến, bảng hành động, phát, ảnh đại diện, TV, chuỗi, ca biên
docs/plan/04-media-engine.md       AVPlayer (iOS, tvOS), MediaPlayer (Android), chép từ jPlayer Mobile, spike
docs/plan/05-data.md               API, model, MockTransport, contractcheck, lưu trữ, ghi chú pháp lý
docs/plan/06-gdesk-extensions.md   G1–G7 cho gDesk + dự phòng F1–F6
docs/plan/07-testing.md            test, 43 trạng thái so ảnh, ngưỡng, hiệu năng, danh sách máy thật
docs/plan/08-orchestration.md      agent/model, quy trình, 86 công đoạn, leo thang, chế độ tự động
docs/plan/PROGRESS.md              điểm nối lại (orchestrator cập nhật)
docs/BLOCKERS.md                   việc cần người quyết định + đề xuất gDesk
.claude/agents/df-*.md             10 sub-agent
mock/dancefitvn-mock-api/          24 route, 16 mã lỗi, media mẫu (hợp đồng API)
reference/prototype/               prototype Night Stage (đặc tả, không sửa) + ảnh 43 trạng thái
reference/design/                  bảng thiết kế Bộ A (SVG, PNG), tokens.json/.go/.css
reference/tools/capture_dancefitvn.py   chụp 43 trạng thái prototype + layout.json
reference/tools/gen_vectors.js          vector test (mm:ss, dung lượng, focus TV) từ mã prototype
reference/tools/extract_strings.py      danh sách chuỗi hiển thị của prototype
reference/tools/css_spec.py             bảng CSS cho 02 §7
```

## Cách dùng

0. **Thư mục gDesk.** Bạn clone gDesk vào `~/Working/desk`; quy ước là `~/Working/gdesk`:
   ```sh
   mv ~/Working/desk ~/Working/gdesk
   ```
1. **Tạo worktree gDesk cho app:**
   ```sh
   cd ~/Working/gdesk && git fetch origin
   git worktree add -b dancefitvn-app-support ../gdesk-dancefitvn-app origin/main
   cd ../gdesk-dancefitvn-app && git push -u origin dancefitvn-app-support
   ```
   Nhánh đã có trên GitHub: `git worktree add ../gdesk-dancefitvn-app dancefitvn-app-support`.
2. **Cài `gdesk` từ worktree và tạo repo app:**
   ```sh
   cd ~/Working/gdesk-dancefitvn-app && go install ./kits/cli/cmd/gdesk
   cd ~/Working && gdesk init -local ./gdesk-dancefitvn-app dancefitvn-app
   cd dancefitvn-app && git init      # nếu gdesk init chưa tạo git
   ```
3. **Chép bộ kế hoạch vào repo** (dấu `/.` để chép cả `.claude`; `CLAUDE.md` của bộ này thay bản do `gdesk init` tạo):
   ```sh
   unzip ~/Downloads/dancefitvn-app-plan.zip -d /tmp/dfv
   cp -R /tmp/dfv/dancefitvn-app-plan/. ~/Working/dancefitvn-app/
   ls ~/Working/dancefitvn-app/.claude/agents     # 10 tệp df-*.md
   ```
4. **Công cụ trên Mac:**
   - Go 1.27+, Xcode với nền tảng iOS và tvOS (`xcodebuild -downloadPlatform tvOS`), Android Studio (SDK android-35 và 36, NDK, Platform-Tools, Emulator), JDK 21, `ffmpeg`, Node 20+.
   - Python + Playwright: `python3 -m venv local/.venv && local/.venv/bin/pip install playwright && local/.venv/bin/python -m playwright install chromium`.
   - Thiết bị ảo:
     ```sh
     xcrun simctl create "DanceFITVN iPhone 16" "iPhone 16"
     xcrun simctl create "DanceFITVN Apple TV" "Apple TV 4K (3rd generation)"
     sdkmanager --list_installed | grep system-images      # dùng đúng image đã cài, ví dụ system-images;android-35;default;arm64-v8a
     avdmanager create avd -n dfv-412x892 -k "system-images;android-35;default;arm64-v8a" -d pixel_7
     sed -i '' -e 's/^hw.lcd.width=.*/hw.lcd.width=1236/' -e 's/^hw.lcd.height=.*/hw.lcd.height=2676/' -e 's/^hw.lcd.density=.*/hw.lcd.density=480/' ~/.android/avd/dfv-412x892.avd/config.ini
     # đã có AVD jplayer-412x892 của jPlayer Mobile thì dùng luôn: đổi tên AVD trong CLAUDE.md §6 và 08
     ```
   - `gdesk doctor` phải xanh cho ios, android, tvos.
5. **(Nên làm) Lưu tài liệu API offline:** mở tài liệu "DanceFITVN API v1 — Hướng dẫn phát triển từ JSON mock" (https://claude.ai/artifact/G9snadFLb98W8TBT1vNjdX), Export → Markdown, lưu thành `docs/api/api-v1.md`.
6. **Mở Claude Code:**
   ```sh
   cd ~/Working/dancefitvn-app
   claude --model opus --add-dir ../gdesk-dancefitvn-app
   ```
   Kiểm tra: hỏi "Liệt kê các subagent bạn có thể gọi trong dự án này, kèm model" — phải có đủ 10 agent `df-*` (architect, systems, gdesk, ui, logic, tests, matcher, reviewer, chores, expert).
7. **Gửi một prompt duy nhất:**

   > Bạn là orchestrator của DanceFITVN App. Đọc CLAUDE.md, docs/plan/PLAN.md, docs/plan/08-orchestration.md và docs/plan/PROGRESS.md, rồi chạy ở chế độ tự động: thực hiện tuần tự mọi phase từ phase đang dở đến hết theo đúng quy trình của 08, giao việc cho các sub-agent, chỉ dừng ở các điểm dừng bắt buộc (§ chế độ tự động). Mọi sửa đổi gDesk làm trong ../gdesk-dancefitvn-app trên nhánh dancefitvn-app-support. Không commit, không push. Cuối cùng báo cáo: phase đã xong, phần còn dở, blockers.

   Hết phiên hoặc hết ngữ cảnh: mở lại Claude Code và gửi **đúng prompt đó**; orchestrator đọc `PROGRESS.md` và đi tiếp.

## Bạn cần làm gì trong lúc chạy

- Xem `docs/BLOCKERS.md`: đề xuất gDesk (G1 Keychain/Keystore, G2 chọn ảnh, G4 trạng thái mạng, G5 tải nền…) cần bạn đồng ý; chứng chỉ ký, tài khoản thử server, máy thật. App vẫn chạy bằng dự phòng trong lúc chờ.
- Trả lời câu hỏi PLAN §8 bất cứ lúc nào (ghi vào PROGRESS §6 hoặc nói với orchestrator); chưa trả lời thì dùng mặc định.

Claude Code không commit, không push ở cả `dancefitvn-app` lẫn `gdesk-dancefitvn-app`; bạn tự làm khi muốn.
