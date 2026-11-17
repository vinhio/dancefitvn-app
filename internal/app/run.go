package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"

	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/bridge/gpu"
	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/framework/host"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// appRoot is what a shell gives launch (root_mobile.go, root_tv.go): the
// widget to mount, what to do once it is mounted, what to close with the
// window and what to run after each frame. mounted, close and afterFrame
// may be nil.
type appRoot struct {
	widget  ui.Widget
	mounted func(v *ui.View)
	close   func()
	// afterFrame, when set, runs after every frame of the view
	// (ui.AfterFrame): the phone's makes its playback after the first.
	afterFrame func()
}

// Options are what the app is started with.
type Options struct {
	// Mock is the mock API's folder (routes.json, courses/, me/, media/ ...) the
	// app was built with, the root package's mockAPI(); nil in a live build.
	// The kitdemo build (-tags kitdemo) draws its pages from it.
	Mock fs.FS
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// options is what Run was given, read by the shell's root (root_mobile.go,
// root_kitdemo.go) while the window is being made; Run sets it once, before
// the host starts.
var options Options

// appName is the app's name, shown by the system and the window title.
const appName = "DanceFITVN"

// appID is the bundle id and application id (gdesk.json's id), and the
// name of the app's folders under the user's data and cache folders.
const appID = "com.jivecode.dancefitvn"

// The prototype's phone, 393×852 DIPs (PLAN D2): the window the mobile
// shell asks for (root_mobile.go).
const (
	mobileW = 393
	mobileH = 852
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Run starts DanceFITVN on the system's driver and blocks until the app
// ends: the time zone and the fonts first, then the window, opened once
// the system has launched the app. o is what main gives it.
func Run(o Options) error {
	options = o
	SetLocalZone()
	registerAppFonts()
	if err := useAppTextEngine(); err != nil && !errors.Is(err, core.ErrUnsupported) {
		log.Printf("dancefitvn: text engine: %v", err)
	}
	h, err := host.New(host.WithName(appName), host.WithID(appID))
	if err != nil {
		return fmt.Errorf("app: run: %w", err)
	}
	h.OnLaunch(func(context.Context) error { return launch(h) })
	if err := h.Run(); err != nil {
		return fmt.Errorf("app: run: %w", err)
	}

	return nil
}

// newTheme returns the view's theme: the Night Stage look, with Inter 16 /
// 400 as the face of any text that names none (internal/ui/theme).
func newTheme() *ui.Theme { return theme.New(theme.DefaultNaming()) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// launch opens the window and mounts the shell's root screen on it
// (root_mobile.go, root_tv.go), drawn on the GPU when the system has one;
// the view, the root's services and the GPU are closed with the window.
// A developer's build first reads the variables of the launch
// (applyLaunchEnv). Call it on the UI thread, from a launch hook.
func launch(h *host.App) error {
	w, err := h.OpenWindow(host.WindowOptions{Title: appName, Width: windowW, Height: windowH})
	if err != nil {
		return fmt.Errorf("app: open window: %w", err)
	}
	opts := viewOptions()
	g, gerr := gpu.New()
	if gerr == nil {
		opts = append(opts, ui.WithGPU(g))
	} else {
		log.Printf("dancefitvn: no GPU, drawing on the CPU: %v", gerr)
	}
	applyLaunchEnv()
	root, err := newAppRoot(h, locales.MustTexts())
	if err != nil {
		closeGPU(g)

		return fmt.Errorf("app: root: %w", err)
	}
	if root.afterFrame != nil {
		opts = append(opts, ui.AfterFrame(root.afterFrame))
	}
	v, err := ui.Mount(w, root.widget, opts...)
	if err != nil {
		closeGPU(g)
		closeRoot(root)

		return fmt.Errorf("app: mount: %w", err)
	}
	mounted(v)
	if root.mounted != nil {
		root.mounted(v)
	}
	w.OnClosed(func(*host.Window) {
		v.Close()
		closeRoot(root)
		closeGPU(g)
	})

	return nil
}

// closeGPU closes g, when there is one, logging a failure.
func closeGPU(g gpu.Renderer) {
	if g == nil {
		return
	}
	if err := g.Close(); err != nil {
		log.Printf("dancefitvn: close GPU: %v", err)
	}
}

// closeRoot closes what the root holds, when it holds anything.
func closeRoot(r appRoot) {
	if r.close != nil {
		r.close()
	}
}
