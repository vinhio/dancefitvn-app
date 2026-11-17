# media/ — file mẫu cho `mock-media://`

`MockTransport` đổi `mock-media://<đường-dẫn>` thành `media/<đường-dẫn>`.

| Thư mục | Nội dung |
|---|---|
| `covers/<course_id>.jpg` | Ảnh bìa mẫu 1600×900 cho cả 16 khoá |
| `avatars/u_412.jpg` | Ảnh đại diện HLV mẫu 512×512 |
| `badges/<code>.png` | 3 huy hiệu HDV DanceFIT / StepX / BodyX, nền trong |
| `hls/<lesson_id>/master.m3u8` | Video dọc 9:16 (480×854), 12 giây, đoạn 4 giây. Có sẵn cho 10 bài của Set 35 (`l_3501` … `l_3510`) |
| `audio/<lesson_id>.mp3` | MP3 30 giây, 128 BPM, 128 kbps. Có sẵn cho 10 bài của Set 35 |
| `hls/_sample/`, `audio/_sample.mp3` | Bản dự phòng chung |

**Quy tắc dự phòng:** khi bài học không có file riêng (các Set khác), `MockTransport` dùng `hls/_sample/master.m3u8` và `audio/_sample.mp3`. Nhờ đó mọi bài đều phát được trên mock.

Tất cả là file giả để thử giao diện, không phải nội dung thật của DanceFITVN. Thay bằng video và nhạc thật khi khách cung cấp.
