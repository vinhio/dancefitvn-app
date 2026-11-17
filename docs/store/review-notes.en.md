# App Review notes — DanceFITVN 1.0.0 (iOS and tvOS)

*Adapted from jplayer-mobile/docs/store/review-notes.en.md at 49de3c2 (structure only).*

Notes for the App Store reviewer (iOS app and Apple TV app). The store listing and the app UI are in Vietnamese; the quoted strings below are the app's exact Vietnamese text.

Status: DRAFT. Items marked **TBD (DanceFITVN)** must be filled in before the review. The demo account is **not** included in this repository.

## What the app is

DanceFITVN is a learning app for coaches of DanceFITVN. A coach signs in with an account that already exists, then watches the lesson videos and plays the lesson audio of the courses assigned to that account. The app is also the companion for the Apple TV app: the TV shows a pairing code and the phone confirms it.

- **The app does not support account creation.** Accounts are created for members by DanceFITVN on its website; the app only signs in existing members (shown on the sign-in screen: "Ứng dụng hỗ trợ cho các thành viên của DanceFITVN" — "This app is for DanceFITVN members"). The same sentence is on the Apple TV welcome screen. The login screen offers "Đăng nhập bằng Face ID / Vân tay" for a saved login and "Quên mật khẩu?" (forgot password, opens the website's reset page). There is no sign-up screen, button or link in the iPhone, Android or Apple TV app.
- No in-app purchase, no subscription purchase, no sign-up, no advertising, no third-party SDK.
- The app shows only the content of the account. The app has no purchase, enrolment or registration button or link and does not sell anything. A course the member has not joined shows its description, a few rows and the plain text "Liên hệ DanceFITVN để tham gia khoá học" ("Contact DanceFITVN to join the course"), with no call to action and no link; the address of the website's enrolment page that the API may send is not used by the app.
- **TBD (DanceFITVN):** confirm how a coach gets a course (assigned by DanceFITVN, or a paid course on the website). The app must not link to a purchase page in a way that breaks guideline 3.1.1; it does not.

## Demo account

| Field | Value |
|---|---|
| Email | <to be provided by DanceFITVN> |
| Password | <to be provided by DanceFITVN> |
| Role | Coach (coach profile enabled) |
| Courses | <to be provided by DanceFITVN> |

**TBD (DanceFITVN):** create a reviewer account on the live server, with the courses and the coach profile above, and enter it in App Store Connect → App Review Information. Do not write the password in this repository; use the App Review field only. The account must work for the full review (iPhone and Apple TV).

## How to test

1. **iPhone.** Launch the app. On the login screen, enter the demo email and password, then tap the sign-in button. Home shows the greeting, "Tiếp tục xem" (continue watching) and the courses of the account.
2. Open a course, then a lesson. The lesson plays its video (HLS over HTTPS). Open the lesson's audio track from the player.
3. **Account tab.** Shows the account information and "Hồ sơ huấn luyện viên" (coach profile). Changing the profile photo: "Chỉnh sửa ảnh đại diện" → "Chụp ảnh mới" or "Chọn từ thư viện" → "Xác nhận". The photo is saved only after "Xác nhận".
4. **Sign out.** Account → "Đăng xuất" → confirm "Đăng xuất?" → "Đăng xuất".

## How to pair Apple TV with the phone

Apple TV (tvOS) does not type a password. The coach signs in with a code:

1. Launch the DanceFITVN app on Apple TV. Choose "Đăng nhập" (sign in). The TV shows a code and a QR code. Its text: "Đăng nhập Apple TV", "Dùng tài khoản huấn luyện viên đã đăng ký trên dancefitvn.com".
2. On the iPhone, signed in with the same demo account: open the Account tab → "Kết nối Apple TV" → enter the code shown on the TV ("Nhập mã bên dưới"). Or scan the QR code.
3. The TV signs in with that account ("Đã đăng nhập bằng {email}") and shows the same courses.
4. To unpair: on the TV, "Đăng xuất khỏi Apple TV" (sign out of Apple TV). The code expires after a few minutes and a new code is made automatically ("mã mới sẽ tự tạo").

Apple TV reviewers do not need a second account. The pairing code works only while the phone is signed in to the demo account.

The TV has no password field: it shows only the code and the QR code. A reviewer therefore needs the phone (or a second phone or browser signed in to the demo account) to complete the pairing.

**TBD (DanceFITVN):** confirm the pairing works on the live server with the review account before submission.

## Links out of the app

- "Tìm hiểu thêm" (learn more) on the Apple TV welcome screen shows a message: "Xem dancefitvn.com để biết thêm". It does not open a browser.
- "Hỗ trợ" (support) in the settings: **TBD (DanceFITVN)** whether it opens a web page or a phone number.
- The Account screen points the user to the website to edit information ("Thay đổi được đồng bộ với tài khoản trên website dancefitvn.com").

## Account deletion

Guideline 5.1.1(v) (account deletion inside the app) applies to apps that let the user create an account. This app does not support account creation (see "What the app is"), so in-app account deletion does not apply. Requests to delete an account and its data go through the contact in the privacy policy (a page or email on dancefitvn.com).

**TBD (DanceFITVN):** give the contact of the privacy policy used for deletion requests here: <to be provided by DanceFITVN>.

## Permissions

- **Face ID** (`NSFaceIDUsageDescription`, purpose text "DanceFITVN dùng Face ID để bạn đăng nhập nhanh"): unlocks the saved login on the phone. The biometric data stays in the OS; the app receives only success or failure. The key is written by `scripts/release.sh` (gDesk G3a pending); it is not optional for a release.
- **Photo library / camera:** the profile photo only, and only when the user taps "Chụp ảnh mới" or "Chọn từ thư viện".
- **Background audio** (`UIBackgroundModes: audio`): lesson audio keeps playing when the screen locks; the playback controls appear on the Lock Screen and in Control Center.
- **Network:** the release build connects to dancefitvn.com only.

## Export compliance

The app uses only standard HTTPS/TLS (through the system and the Go standard library). `ITSAppUsesNonExemptEncryption` is set to false in both bundles. See `docs/store/export-compliance.md`.

## Privacy

The data the app collects (name, email, phone, profile photo, coach profile, user id, install id, lesson progress) is described in `docs/store/app-privacy.md`. The privacy policy and support URLs are **TBD (DanceFITVN)**.

## Contacts

- Support: **TBD (DanceFITVN)** (URL on dancefitvn.com)
- Privacy policy: **TBD (DanceFITVN)** (URL on dancefitvn.com)
- Developer contact for the review: **TBD (DanceFITVN)**
