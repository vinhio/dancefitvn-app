# mockserve

Developer tool. Serves the mock API of `mock/dancefitvn-mock-api` over HTTP on
the loopback address, with the `store.Overlay` (edits of the demo user) in
front of it and its **pair relay** on. Two dev builds, an iPhone simulator and
an Apple TV simulator, then share one mock: the code the TV shows is the code
the phone confirms, and the TV's next poll gets the session.

```sh
go run ./cmd/mockserve                       # 127.0.0.1:8787
go run ./cmd/mockserve -overlay local/mock-overlay.json   # keep edits between runs
```

| Flag       | Default                      | Meaning                                              |
|------------|------------------------------|------------------------------------------------------|
| `-addr`    | `127.0.0.1:8787`             | Listen address. Loopback only; anything else is refused (exit 2). |
| `-mock`    | `mock/dancefitvn-mock-api`   | Mock folder.                                         |
| `-overlay` | empty (memory)               | File that keeps the overlay between runs (mode 0600). |
| `-v`       | off                          | Log the mock's own lines. Never bodies, tokens or emails. |

Endpoints: `/api/v1/...` (the routes of `routes.json`) and `/media/...` (the
mock media folder; `mock-media://` in answers is rewritten to
`http://127.0.0.1:8787/media/`).

## Pair relay

* `POST /tv/pair/start` answers the mock's body with a random 6-character code
  (no `0 1 I L O`), `display_code`, `poll_secret`, `expires_in` 300 and the
  matching `qr_url`.
* `POST /tv/pair/poll` answers `pending` (with the seconds left), then, once
  after a phone confirmed, the session of `tv/pair-poll.confirmed.json` (user
  `u_412`). An unknown or expired code, or another secret, is 410
  `pair_code_expired`.
* `POST /tv/pair/confirm` (bearer required) is 404 `pair_code_invalid` for a
  code that is unknown, expired or already confirmed. The mock's own refusals
  (`device_limit`, 401) still apply.
* `POST /tv/logout` takes the TV out of the device list.
* Nothing pending survives a restart. Mock scenarios of the three pair routes
  are not used here.

## Running the simulators against it

A dev build (`-tags dancefitvndev`, `scripts/dev-build.sh`) that finds
`DFV_API=http://127.0.0.1:8787/api/v1` in its environment talks to this server
instead of the mock in its own process (simulators share the Mac's loopback):

```sh
SIMCTL_CHILD_DFV_API=http://127.0.0.1:8787/api/v1 xcrun simctl launch booted <bundle id>
```

The server has no login of its own, which is why it binds to the loopback.
