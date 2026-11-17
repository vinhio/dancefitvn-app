# App Privacy (App Store Connect) — DanceFITVN

*Adapted from jplayer-mobile/docs/store/app-privacy.md at 49de3c2 (structure only). DanceFITVN collects personal data, so the answers differ from jPlayer's "Data Not Collected".*

Draft answers for App Store Connect → App Privacy, for the iOS app and the tvOS app. The account holder enters them. Source of truth for the types: `assets/ios/PrivacyInfo.xcprivacy` and `assets/tvos/PrivacyInfo.xcprivacy` (P8.2a). If one of them changes, change this file too.

Status: DRAFT. Every answer marked **TBD (DanceFITVN)** needs confirmation from DanceFITVN (BLOCKERS, P8.2).

## Top-level answers

| Question | Answer |
|---|---|
| Do you or your third-party partners collect data from this app? | **Yes** |
| Is data used to track you? (App Tracking Transparency) | **No.** `NSPrivacyTracking` = false, `NSPrivacyTrackingDomains` empty. No advertising, no analytics SDK, no third-party SDK. |
| Privacy policy URL | **TBD (DanceFITVN)** — page on dancefitvn.com, Vietnamese and English versions |
| Support URL | **TBD (DanceFITVN)** — page on dancefitvn.com |

## Data types — iOS app

Every type below: **Linked to the user: Yes**, **Used for tracking: No**, **Purpose: App Functionality** only. No type is used for analytics, advertising, developer's own marketing or third-party partners.

| App Store Connect type | Manifest key | What it is in DanceFITVN | Source in the app |
|---|---|---|---|
| Contact Info → Name | `NSPrivacyCollectedDataTypeName` | First and last name shown on the account and the greeting | `GET /me`, `PATCH /me` |
| Contact Info → Email Address | `NSPrivacyCollectedDataTypeEmailAddress` | Account email (sign-in, shown as "Đã đăng nhập bằng {email}" on TV) | `GET /me`, login |
| Contact Info → Phone Number | `NSPrivacyCollectedDataTypePhoneNumber` | Account phone, optional | `GET /me` / `PATCH /me` |
| Photos or Videos → Photos | `NSPrivacyCollectedDataTypePhotosorVideos` | Profile photo (avatar) chosen from the camera or the photo library, uploaded only after "Xác nhận" | `PUT /me/avatar` |
| User Content → Other User Content | `NSPrivacyCollectedDataTypeOtherUserContent` | Coach profile: bio, subjects, social links (Facebook, Instagram, YouTube, TikTok, Zalo, Threads), places | `GET/PUT /me/coach-profile` |
| Identifiers → User ID | `NSPrivacyCollectedDataTypeUserID` | Account id returned by the server | `GET /me` |
| Identifiers → Device ID | `NSPrivacyCollectedDataTypeDeviceID` | Install id: a random UUID created at first launch, sent as `X-Install-Id`; the same value identifies the device in the device list | `SecretDir/install-id`, header |
| Usage Data → Product Interaction | `NSPrivacyCollectedDataTypeProductInteraction` | Lesson progress (watched, position), courses, "Tiếp tục xem" | `PUT /lessons/{id}/progress`, `POST /progress/batch` |

Not collected (answer "No" in App Store Connect unless DanceFITVN says otherwise):

- Health and fitness data, financial info, location, contacts, browsing history, search history, diagnostics, sensitive info, other usage data (crash logs are not sent by the app; the OS may send its own reports, which the developer does not receive as app data).
- Face ID / fingerprint: the app asks the OS to unlock the saved login ("Đăng nhập bằng Face ID / Vân tay"). The biometric data never reaches the app or the server, so it is not a collected type.

**TBD (DanceFITVN):** confirm that no server log, analytics, or error report keeps any of the above beyond the API's own operation; confirm the retention period for progress and the device list.

## Data types — tvOS app (subset)

The tvOS manifest declares five types. The Apple TV app does not upload avatars and does not edit the coach profile (its settings say "Sửa hồ sơ trên điện thoại hoặc website"), so Photos and Other User Content are not listed.

| App Store Connect type | Manifest key |
|---|---|
| Contact Info → Name | `NSPrivacyCollectedDataTypeName` |
| Contact Info → Email Address | `NSPrivacyCollectedDataTypeEmailAddress` |
| Identifiers → User ID | `NSPrivacyCollectedDataTypeUserID` |
| Identifiers → Device ID | `NSPrivacyCollectedDataTypeDeviceID` |
| Usage Data → Product Interaction | `NSPrivacyCollectedDataTypeProductInteraction` |

Same answers as iOS: linked yes, tracking no, purpose App Functionality.

**TBD (DanceFITVN):** does the Apple TV app ever receive the phone number? The TV does not show it, so it is not listed. Confirm that `GET /me` on tvOS does not store it locally (the local cache follows 05 §6: cleared on sign-out).

## Required-reason APIs (for the same review)

Not a nutrition-label question, but App Store Connect checks it at upload. The answers follow the manifests: FileTimestamp `C617.1`, SystemBootTime `35F9.1`; iOS DiskSpace `E174.1` and `85F4.1`; tvOS UserDefaults `CA92.1`. Full table: `docs/release.md`, section "Privacy manifests".

## Check before submitting

- [ ] The types above match `assets/ios/PrivacyInfo.xcprivacy` and `assets/tvos/PrivacyInfo.xcprivacy` of the build being uploaded.
- [ ] Both URLs open a real page (TBD (DanceFITVN)).
- [ ] DanceFITVN has confirmed each TBD line.
- [ ] Answers match `docs/store/data-safety.md` (Google Play).
