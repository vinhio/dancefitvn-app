// Package screens holds the screens of the phone (docs/plan/01 section 1): one
// file per prototype builder (MS.splash, MS.login, MS.loading ...). A screen is
// built by the Shell's route factory from an env.Env, stands in a nav.Page (or
// lays itself out when it has no scroll), and asks the app only through
// env.Services. The widgets' Names are the keys of the prototype's layout.json,
// so that cmd/shotcmp can pair them.
package screens
