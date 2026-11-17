// Package env is the contract between the phone's screens and the app: the
// interface of what the screens ask of the app (Services), the actions of
// docs/plan/03 section 3 (Actions), and the Env that carries them with the
// texts and the navigator. It has no widgets, and imports no store, api client
// or auth manager: internal/app adapts those to Services, and the tests give the
// screens a fake (shell/shelltest).
package env
