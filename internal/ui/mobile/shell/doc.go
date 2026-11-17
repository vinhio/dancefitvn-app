// Package shell is the root of the phone: the Shell widget owns the navigator,
// the table of routes (routes.go), the boot sequence (splash, then login or home,
// 03 section 1) and the actions of 03 section 3 (actions.go). It talks to the app
// only through env.Services.
package shell
