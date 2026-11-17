# DanceFITVN mock API v1

Bộ JSON giả lập cho API `/api/v1`. App gDesk đọc `routes.json`, khớp `method` + `path`
(hỗ trợ tham số `{id}`), chọn response theo kịch bản (`default` nếu không đặt), chờ `default_delay_ms`
rồi trả `status` + nội dung file. `file: null` = 204 không có body.

- Token `{{path.id}}` trong file được thay bằng giá trị tham số `{id}` của đường dẫn.
- URL dạng `mock-media://…` trỏ tới thư mục `media/` (video HLS dọc, MP3, ảnh bìa, huy hiệu) do đội tự đặt file mẫu.
- `requests/` chứa body mẫu cho các endpoint có body.
- Tài liệu: "DanceFITVN API v1 — Hướng dẫn phát triển từ JSON mock".
