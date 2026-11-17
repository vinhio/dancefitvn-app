// Package shelltest is the fake app the phone's screens are tested against:
// env.Services over the real api.Client, auth.Manager and mock transport of
// mock/dancefitvn-mock-api (D7: the screens see the API's own answers and its
// real *api.Error), with the knobs a test needs: the scenario of a route, the
// network, a gate that holds a request, the biometry, the clock. It is a
// helper package (not a _test file) so that internal/app can use it too; it
// imports testing only for MockFS.
package shelltest
