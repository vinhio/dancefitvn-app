# 05 — Dữ liệu, API, mock, lưu trữ

Hợp đồng: `mock/dancefitvn-mock-api/` (24 route) và tài liệu "DanceFITVN API v1 — Hướng dẫn phát triển từ JSON mock" (link trong README; bản xuất Markdown, nếu có, đặt ở `docs/api/api-v1.md`). Khi hai nguồn khác nhau: JSON mock thắng cho cấu trúc, tài liệu thắng cho quy tắc nghiệp vụ; ghi chỗ lệch vào `docs/BLOCKERS.md`.

## 1. Quy ước chung

| Mục | Quy ước |
|---|---|
| Base URL | live `https://dancefitvn.com/mobile/api/v1`; mock `mock://` |
| Định dạng | JSON UTF-8, `snake_case`, thời gian ISO 8601 có múi giờ, ngày `YYYY-MM-DD`, thời lượng `*_sec` (giây), dung lượng `*_bytes` |
| Header gửi | `Authorization: Bearer <access>`, `X-App-Version`, `X-Platform: ios|android|tvos`, `X-Install-Id` (UUID cố định mỗi lần cài, lưu tại `SecretDir/install-id`), `Accept-Language: vi` |
| Lỗi | `{"error": {"code", "message", ...}}`; app rẽ nhánh theo `code`, không theo `message` (16 mã, `mock/.../errors/`) |
| Phân trang | `?cursor=&limit=` (mặc định 20, tối đa 50), `page.next_cursor` null = hết |
| Cache | `GET` trả `ETag`; app gửi `If-None-Match`, 304 → bản lưu; ngoại tuyến → bản lưu |

## 2. Model (`internal/model`)

Kiểu Go theo đúng JSON (`encoding/json/v2`, `RejectUnknownMembers` **tắt** cho response để server thêm trường không làm vỡ app; bật trong test của mock để bắt sai chính tả).

| Kiểu | JSON nguồn | Trường chính |
|---|---|---|
| `Session` | `auth/login.json` | `access_token`, `expires_in`, `refresh_token`, `refresh_expires_in`, `user`, `device` |
| `User` | `me/me.json` `.user` | `id`, `first_name`, `last_name`, `full_name`, `email`, `phone`, `avatar_url`, `member_since`, `role` |
| `Badge` | `.badges[]` | `code`, `name`, `image_url` |
| `CoachProfile` | `me/coach-profile.json` | `bio`, `subjects`, `subjects_editable`, `socials[]{platform,url}`, `allowed_platforms`, `max_socials`, `places`, `max_places`, `public_url` |
| `Home` | `me/home.json` | `greeting_name`, `continue`, `stats`, `my_courses`, `updates` |
| `Course` | `courses/*.json`, `me/courses.json` `.items[]` | `id`, `name`, `tagline`, `category`, `category_label`, `lesson_count`, `total_duration_sec`, `cover_url`, `is_new`, `enrolled`, `progress.watched_count`, `description`, `lessons[]`, `resume`, `enroll_url`, `updated_at` |
| `Lesson` | `.lessons[]` | `id`, `no`, `title`, `tag`, `duration_sec`, `has_video`, `has_audio`, `audio_size_bytes`, `video_aspect`, `progress{position_sec,watched}`, `locked` |
| `Stream` | `lessons/stream.json` | `hls_url`, `expires_at`, `aspect`, `variants[]`, `resume_position_sec` |
| `Audio` | `lessons/audio.json` | `url`, `expires_at`, `format`, `bitrate_kbps`, `duration_sec`, `size_bytes`, `sha256`, `filename` |
| `Device` | `me/devices.json` | `id`, `platform`, `name`, `app_version`, `last_seen_at`, `current` |
| `PairStart`, `PairPoll` | `tv/pair-*.json` | `code`, `display_code`, `poll_secret`, `poll_interval_sec`, `expires_in`, `expires_at`, `qr_url`, `status` |
| `AppConfig` | `app/config.json` | `min_version`, `latest_version`, `features`, `download`, `progress`, `support`, `links`, `server_time` |
| `APIError` | `errors/*.json` | `code`, `message`, `fields`, `retry_after`, `attempts_left`, `devices`, `enroll_url`, `max_bytes`, `store_url` |

Ngày `YYYY-MM-DD` là `model.Date` (MarshalText/UnmarshalText): stdlib `encoding/json/v2` từ chối tuỳ chọn `format:DateOnly`. `PairPoll` phẳng (trường tường minh + `PairPoll.Session()`), không nhúng `Session`, vì `expires_in` khác nhau giữa trạng thái pending (TTL mã) và confirmed (TTL access token).

`Course.Short()`: bỏ tiền tố "DanceFIT ", in hoa ("SET 35", "HOT EXTRA"); "Giới thiệu DanceFIT" → "GIỚI THIỆU".

## 3. Client (`internal/api`)

```go
type Transport interface {
    Do(ctx context.Context, req *Request) (*Response, error) // Response.Status, Header, Body
}
type Client struct { /* base, transport, auth hook, install id, version, platform */ }
func (c *Client) Login(ctx context.Context, email, password string, dev model.DeviceInfo) (model.Session, error)
// … một hàm mỗi route; tên theo route id của routes.json (auth.login → Login, me.courses → MyCourses …)
```

- Lỗi: `*api.Error{Status, Code, …}`; bọc sentinel `core` (`core.ErrNotFound` cho 404, `core.ErrUnsupported` cho 426 …) bằng `%w`; lỗi mạng bọc `api.ErrOffline`.
- 401 `token_expired` → `auth.Refresh` một lần (single-flight), gửi lại; vẫn 401 → `auth.ErrSignedOut`.
- `Live`: `net/http` (không bị ATS chặn), timeout 15 s, thử lại GET lỗi mạng 2 lần (0,5 s, 1,5 s).
- `Hybrid`: `live_routes` (danh sách route id) đi Live, còn lại Mock. `api.CheckLiveRoutes(ids)` kiểm danh sách trước: id lạ, hoặc route có đăng nhập khi `auth.login`/`auth.refresh` còn ở mock → `core.ErrInvalid` (tên route trong thông báo, không có token). Chỉ dùng ở bản dev (`DFV_LIVE_ROUTES`, P8.1b).

## 4. MockTransport (`internal/api/mock`)

Đọc `mock/dancefitvn-mock-api/routes.json` từ `embed.FS` (build dev và test) hoặc thư mục cấu hình. Sáu quy tắc (theo `mock/.../README.md` và tài liệu API §2):

1. Khớp `method` + `path` (bỏ base `/api/v1`); `{id}` khớp một đoạn đường dẫn bất kỳ.
2. Kịch bản: `scenarios[routeID]` từ cấu hình (`local/mock-scenarios.json` hoặc menu dev), không có → `default`.
3. `{{path.id}}` trong tên file và trong nội dung được thay bằng giá trị `{id}` thật.
4. Trễ `default_delay_ms` (250 ms) trước mỗi trả lời; `file: null` → 204 không body.
5. Route `auth: true` khi chưa có token → `errors/token_expired.json` (401).
6. `mock-media://<path>` → `http://127.0.0.1:<port>/<path>` của máy chủ media loopback (04 §5); file media không có (`hls/<id>/`, `audio/<id>.mp3`) → `_sample` (`media/README.md`).

Mock không lưu trạng thái: `PUT`/`PATCH` trả bản đã cập nhật nhưng `GET` sau vẫn đọc file gốc. App giữ thay đổi trong `store` (lớp ghi đè cục bộ) để demo liền mạch; lớp này **chỉ bật ở mode mock**.

Menu dev (build tag `dancefitvndev`): chạm dòng phiên bản ở màn Tài khoản 8 lần → danh sách route và kịch bản, chọn để thử màn lỗi; ở mode live menu không có. (Màn Tài khoản không có logo; theo Q-P6-16 mở menu bằng dòng phiên bản. Chỉ bản dev; P6.1c là tuỳ chọn, chưa làm.)

## 5. `cmd/contractcheck`

```sh
go run ./cmd/contractcheck -mock mock/dancefitvn-mock-api -base https://staging.dancefitvn.com/mobile/api/v1 \
   -email $DFV_TEST_EMAIL -password $DFV_TEST_PASSWORD [-route me.courses] [-self]
```

- Đăng nhập lấy token, gọi mọi route với body `requests/<route>.json`, so **cấu trúc** response với file mock của kịch bản `default` (đủ khoá, cùng kiểu: chuỗi/số/bool/null/mảng/object; mảng so phần tử đầu), không so giá trị. Trường thừa ở server = cảnh báo; thiếu hoặc sai kiểu = lỗi.
- `-self`: so mock với chính nó (kiểm công cụ, chạy trong `go test`).
- Chạy trên máy không cần server: `go run ./cmd/mockserve` (127.0.0.1:8787) rồi `go run ./cmd/contractcheck -base http://127.0.0.1:8787/api/v1` — cùng đường HTTP mà server thật sẽ đi (`internal/api` `net/http`, P8.1a). Máy chủ thật (`-base https://dancefitvn.com/mobile/api/v1`, tài khoản thử, `-write` chỉ trên tài khoản thử) → BLOCKERS (backend).
- Không ghi mật khẩu vào log; route ghi (`PATCH /me`…) chỉ chạy với `-write` trên tài khoản thử.

## 6. Lưu trữ cục bộ

| Dữ liệu | Nơi | Ghi chú |
|---|---|---|
| Access/refresh token | `bridge/system/secret` (G1) hoặc `secret.FileStore` (F1) | service `com.jivecode.dancefitvn`, account = `install_id` |
| Install id | `SecretDir/install-id` | thư mục không sao lưu (Q-P1-5/6) |
| DataDir / SecretDir tvOS | `Library/Caches` | như jPlayer TV; mất phiên nếu tvOS dọn cache → G1/P7.5 (Q-P1.4-D2) |
| Cache API | `<cache>/api/<hash(url)>.json` + ETag | xoá khi đăng xuất |
| Ảnh (bìa, avatar, huy hiệu) | `framework/components` imagecache | |
| File âm thanh | `<data>/audio/<lesson_id>.mp3` + `index.json` (lesson, course, size, sha256, ngày tải) | iOS: `Application Support`, loại trừ iCloud backup; Android: bộ nhớ riêng của app; giữ khi đăng xuất (prototype: "File âm thanh đã tải vẫn được giữ trên máy") nhưng chỉ phát được khi đăng nhập lại đúng tài khoản |
| Tiến độ chờ gửi | `<data>/progress-queue.json` | gửi bằng `POST /progress/batch` khi có mạng |
| Prefs | `<data>/state/prefs.json` | `wifi_only`, `auto_next`, `quality`, `scenarios` (mock) |

## 7. Ghi chú pháp lý (không phải tư vấn pháp lý)

- Video, nhạc, ảnh bìa, huy hiệu, logo là tài sản của DanceFITVN; repo chỉ chứa media mẫu tự tạo trong `mock/.../media` (ghi rõ "MOCK"). Không đưa nội dung thật vào repo, test hay ảnh chụp.
- Bản quyền bài nhạc trong file âm thanh do DanceFITVN chịu trách nhiệm; quyết định có cho tải ngoại tuyến hay không thuộc về khách (đã có trong yêu cầu).
- Font Barlow Condensed, Inter, JetBrains Mono: SIL OFL 1.1 — kèm `assets/fonts/OFL.txt`.
- Không lưu mật khẩu; không log token, email đầy đủ.
