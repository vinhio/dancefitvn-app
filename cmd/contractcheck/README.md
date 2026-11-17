# contractcheck

Checks a running DanceFITVN API against the mock API in
`mock/dancefitvn-mock-api` (docs/plan/05-data.md section 5).

It logs in, calls the routes with the bodies of `requests/` and compares the
**structure** of each answer with the file of the `default` scenario: object
keys and the kinds string, number, bool, null, array, object. Values are never
compared. A key the server lacks or a different kind is an error; an extra key
is a warning. A `null` or an empty array in the mock accepts anything, and
arrays compare their first element. When the server answers an error, the
envelope is compared with `errors/<code>.json`.

```sh
export DFV_TEST_EMAIL=...        # test account, never a real customer
export DFV_TEST_PASSWORD=...
go run ./cmd/contractcheck -base https://<host>/api/v1 [-route me.courses] [-write] [-v]

go run ./cmd/contractcheck -self   # the mock against itself (also run by go test)
```

| Flag | Meaning |
|---|---|
| `-mock dir` | mock folder, default `mock/dancefitvn-mock-api` |
| `-base url` | server base URL; plain `http` only for a host on this machine |
| `-email`, `-password` | test account; default `$DFV_TEST_EMAIL`, `$DFV_TEST_PASSWORD`. Prefer the environment: flags show in `ps`. Never printed (the report masks them, tokens and the email) |
| `-route id` | report only this route (repeatable or comma separated); reads still run as prerequisites |
| `-write` | also run routes that change data, on a test account only |
| `-password-new pw` | with `-write`, run `me.password` (this changes the password) |
| `-device id` | with `-write`, run `devices.delete` for this device |
| `-self` | serve the mock in-process over a loopback port and check every route and every scenario |
| `-platform p` | `X-Platform` to send: `ios` (default), `android` or `tvos`; `tv.pair.start` and `tv.pair.poll` always send `tvos`, as the TV app does |
| `-app-version v` | `X-App-Version`, default `1.0.0` |
| `-install-id id` | `X-Install-Id`, default a random UUID per run |
| `-v` | one line per call on stderr |

Plan: `app.config`, `auth.login`, the reads (`me.get`, `me.home`, `me.courses`,
`courses.show` with the first course, `lessons.stream` and `lessons.audio`
with its first lesson that has video or audio, `me.coach.get`, `devices.list`),
`tv.pair.start`, `tv.pair.poll` (`pending` expected), `auth.refresh`. With
`-write`: `me.update` and `me.coach.update` with the account's own current
values, `me.avatar` with a generated 512x512 grey JPEG, `lessons.progress` and
`progress.batch` with the saved position, `tv.pair.confirm`, `auth.forgot`,
`me.password` (needs `-password-new`), `devices.delete` (needs `-device`),
`tv.logout`. `auth.logout` always runs last, so a run leaves no device behind.

`-self` also selects each non-default scenario of the mock in turn and checks
its status and body, so it covers all 24 routes and 50 route and scenario
pairs.

Every request also carries `Accept-Language: vi` and the three client headers above, built by `api.ClientHeader` (the same code as the app).

Exit codes: 0 pass, 1 contract errors, 2 usage or connection failure.

## For the backend team

Run it in your CI against a **test** server and a **test** account, never a
real customer. The tool needs only Go 1.26 or newer and this repository.

```sh
export DFV_TEST_EMAIL=...        # from the CI secret store
export DFV_TEST_PASSWORD=...
go run ./cmd/contractcheck -base "$API_BASE" -write \
    -password-new "$DFV_TEST_PASSWORD_NEW" -device "$DFV_TEST_DEVICE_ID"
```

- `$API_BASE` is `https://<host>/api/v1`; give it as a CI variable, with no
  key or token in the URL. Plain `http` is accepted only for localhost.
- Credentials come from the environment (flags show in `ps`). The report
  masks the password, the email and every token.
- Without `-write` only the reads run. `-write` changes the test account:
  profile (with its own current values), avatar, progress, a pairing, a
  password change (`-password-new`) and the removal of one device (`-device`,
  an id of `devices.list` you can lose). `auth.logout` runs last.
- Exit codes: 0 pass, 1 contract errors (the table names the route and the
  key), 2 usage or connection failure. The table is on stdout, so a CI log
  keeps it.

A PASS proves that every route answers with the status of the default
scenario and with the same keys and kinds as the files in
`mock/dancefitvn-mock-api`, and that the error envelope has the mock's shape.
It does not prove values, business rules, other scenarios (use the mock
through `-self` for those) or load.

Offline proof of the tool itself: `go run ./cmd/contractcheck -self`, and
`go test ./cmd/contractcheck`, which starts `cmd/mockserve` on `127.0.0.1`
and runs the whole plan against it over HTTP.
