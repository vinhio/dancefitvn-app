package model

import (
	"encoding/json/v2"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecretsAreNotPrinted checks that no verb shows a password, a token, a
// poll secret or an e-mail of a request or session.
func TestSecretsAreNotPrinted(t *testing.T) {
	t.Parallel()

	const secret = "s3cr3t-value"
	for _, tc := range []struct {
		name string
		v    any
	}{
		{"login", LoginRequest{Email: secret + "@example.com", Password: secret, Device: DeviceInfo{InstallID: secret}}},
		{"password", PasswordRequest{CurrentPassword: secret, NewPassword: secret}},
		{"refresh", RefreshRequest{RefreshToken: secret, InstallID: secret}},
		{"forgot", ForgotRequest{Email: secret + "@example.com"}},
		{"poll request", PairPollRequest{Code: "ABC", PollSecret: secret}},
		{"tokens", Tokens{AccessToken: secret, RefreshToken: secret}},
		{"session", Session{Tokens: Tokens{AccessToken: secret}, User: UserRef{Email: secret + "@example.com"}}},
		{"pair start", PairStart{PollSecret: secret}},
		{"pair poll", PairPoll{AccessToken: secret, RefreshToken: secret}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%d"} {
				assert.NotContains(t, fmt.Sprintf(verb, tc.v), secret, verb)
				assert.NotContains(t, fmt.Sprintf(verb, &tc.v), secret, verb+" pointer")
			}
			assert.NotContains(t, fmt.Sprint(struct{ In any }{tc.v}), secret, "nested")
		})
	}
}

func TestPasswordNotPrinted(t *testing.T) {
	t.Parallel()

	r := LoginRequest{Email: "a@b.c", Password: "hunter2"}
	got := fmt.Sprintf("%v %+v %#v", r, r, r)
	assert.NotContains(t, got, "hunter2")
	assert.NotContains(t, got, "a@b.c")
	assert.Equal(t, "model.LoginRequest{***}", r.String())
}

// TestLoginRequestReplaceDeviceID checks the optional field of K31: omitted when empty (the
// request file of the mock has no such key), written under its API name when set, and read back;
// the redacted print hides it with the rest.
func TestLoginRequestReplaceDeviceID(t *testing.T) {
	t.Parallel()

	plain, err := json.Marshal(LoginRequest{Email: "a@b.c", Password: "pw"})
	require.NoError(t, err)
	assert.NotContains(t, string(plain), "replace_device_id")

	with, err := json.Marshal(LoginRequest{Email: "a@b.c", Password: "pw", ReplaceDeviceID: "d_102"})
	require.NoError(t, err)
	assert.Contains(t, string(with), `"replace_device_id":"d_102"`)
	var back LoginRequest
	require.NoError(t, json.Unmarshal(with, &back))
	assert.Equal(t, "d_102", back.ReplaceDeviceID)
	assert.Equal(t, "model.LoginRequest{***}", back.String())
}
