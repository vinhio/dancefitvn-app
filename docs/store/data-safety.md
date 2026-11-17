# Data safety (Google Play Console) — DanceFITVN

*Adapted from jplayer-mobile/docs/store/data-safety.md at 49de3c2 (structure only). jPlayer answers "No" to every question; DanceFITVN collects account data, so these answers are different.*

Draft answers for Play Console → App content → Data safety. The developer enters them. The data list is the same as `docs/store/app-privacy.md` (iOS/tvOS), so both stores tell the same story.

Status: DRAFT. Every answer marked **TBD (DanceFITVN)** needs confirmation from DanceFITVN (BLOCKERS, P8.2).

## Overview

| Question | Answer |
|---|---|
| Does the app collect or share any of the required user data types? | **Yes, collects.** |
| Does the app share user data with third parties? | **No.** The app sends data only to DanceFITVN's own API (dancefitvn.com). No third-party SDK, no analytics, no advertising. |
| Is all user data encrypted in transit? | **Yes.** All calls use HTTPS (TLS) to `https://dancefitvn.com/mobile/api/v1`. The app does not send account data over cleartext. (`gdesk.json` `cleartextHosts` lists `127.0.0.1` for local development; that host is loopback only, Q-P8-23.) |
| Can users request that their data be deleted? | **TBD (DanceFITVN).** Proposed: yes, by a request through the website/support. The app has no in-app account deletion (see the note below). Deletion path URL: **TBD (DanceFITVN)**. |
| Is the app for families (children)? | **TBD (DanceFITVN).** Proposed: No (adults; coaches). |
| Does the app use advertising ID / ads SDK? | **No.** |

## Data types collected

For each type: **Collected: Yes**, **Shared: No**, **Processing: ephemeral No**, **Required or optional**, **Purpose: App functionality** only.

| Play category → data type | What it is in DanceFITVN | Required / optional |
|---|---|---|
| Personal info → Name | First and last name | Required (account) |
| Personal info → Email address | Account email | Required (account) |
| Personal info → Phone number | Account phone | **TBD (DanceFITVN)**: optional in the profile |
| Personal info → User IDs | Account id from the server | Required |
| Personal info → Other info | Coach profile: bio, subjects, social links, places | Optional (coach only) |
| Photos and videos → Photos | Profile photo (avatar), uploaded only after the user confirms | Optional |
| App activity → App interactions | Lesson progress, watched state, playback position | Required for "Tiếp tục xem" |
| Device or other IDs → Device or other IDs | Install id (random UUID created at first launch) and the device list (platform, device name, app version) | Required |

Not collected: financial info, health and fitness, messages, location, contacts, calendar, files and docs, audio or voice recordings (lesson audio is played, not recorded), app info and performance (no crash/analytics SDK), web browsing, and "Other actions".

Biometrics: the app uses Android's biometric prompt (`USE_BIOMETRIC`) to unlock the saved login. The biometric data stays in the OS and is not collected by the app.

Note on the Android permissions: `ACCESS_NETWORK_STATE` (to show the offline state) and `USE_BIOMETRIC`. No location, camera or storage permission is declared by the app; the avatar picker uses the system photo picker, which does not need one. **TBD (DanceFITVN):** confirm the photo flow on Android does not need `READ_MEDIA_IMAGES` in the release manifest.

## Deletion and the account

The app does not support account creation (K30, user decision 2026-11-17): accounts are created for members by DanceFITVN on its website, and the app only signs in existing members (the sign-in screen says "Ứng dụng hỗ trợ cho các thành viên của DanceFITVN"). The app has no sign-up or enrolment link; deletion requests go through the contact of the privacy policy. The app offers sign-out ("Đăng xuất" / "Đăng xuất khỏi Apple TV") and no account deletion. Play may ask for a deletion path for apps that let users create an account; the answer depends on whether the website offers account deletion.

- Deletion request path shown to users (Play asks for it): **TBD (DanceFITVN)** — a page or email on dancefitvn.com. Not a real URL yet.
- Data kept after deletion (backups, logs, progress records): **TBD (DanceFITVN)**.

## Check before submitting

- [ ] Every answer matches `docs/store/app-privacy.md` and the build being uploaded.
- [ ] The Google Play deletion path URL is a real page (TBD (DanceFITVN)).
- [ ] The privacy policy URL is entered in Play Console (TBD (DanceFITVN)).
- [ ] DanceFITVN has confirmed each TBD line.
