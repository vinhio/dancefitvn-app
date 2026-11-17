// Adapted from jplayer-tv/internal/media/avf/plane_tvos.go at 781bee5.

//go:build ios && tvos

package avftv

import (
	"fmt"
	"log/slog"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/ffi/cf"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// nativePlane is the B plane (D4, docs/plan/04-media-engine.md §1): an
// AVPlayerLayer under the app's view, through Config.Underlay (gDesk's
// underlays), which the system composites where the view leaves itself
// transparent. A mirrored picture (04 §2.3) is the layer turned about
// its vertical axis: transform.scale.x -1, set by key-value coding in a
// transaction without animation, so the system flips the video it
// composites and the app copies nothing. In this package "a TV item"
// of jplayer-tv is an item with a picture (Source.AudioOnly false). UI
// thread only.
//
// It shows while the view placed it (a non-empty Place): black (the
// layer's background, as the prototype's `#fs .screen { background: #000
// }` under its <video>) until the player's picture covers it. The player
// is on the layer only while a TV item is loaded (attach, until detach or
// hide), so that a player playing radio, or nothing, is attached to no
// layer (R-T4-9). An empty Place hides it. Before T4.6 it was hidden
// while no TV item was loaded, and what lies under the app's view (grey
// on the simulator) showed through a watch screen opened before the
// channel loaded. The view's placement outlives zaps: a TV item after
// another, or after Stop, shows again where the view last placed it,
// without a new Place.
//
// The engine's observer watches the layer's readyForDisplay from the
// moment the layer is made (P7.5a): it tells the first frame of a load on
// the screen (media.EventVideoSize, pictureDue).
//
// The layer and the underlay are made on the first non-empty Place: the
// view's canvas does not exist before its first frame. When Underlay fails
// (no view yet), the plane tries again at the next Place and the next TV
// item, so a Place made too early still takes effect.
type nativePlane struct {
	underlay func(layer uintptr) (Plane, error)
	log      *slog.Logger

	layer    objc.ID   // AVPlayerLayer, owned; 0 until the first non-empty Place
	plane    Plane     // the layer under the view; nil until Underlay succeeds
	player   objc.ID   // the engine's AVPlayer, not owned
	obs      objc.ID   // the engine's observer, not owned; 0 for none
	watching bool      // obs observes the layer's readyForDisplay
	attached objc.ID   // the player the layer has, 0 for none
	rect     geom.Rect // where the view placed it
	wanted   bool      // the view placed it (non-empty rect)
	tv       bool      // a TV item is loaded
	shown    bool      // the underlay was last told to show
	warned   bool      // an Underlay failure was logged
	mirror   bool      // the picture is mirrored (setMirror)
	flipped  bool      // the layer's transform is the mirror
}

// layerSel holds what the plane sends to AVPlayerLayer.
var layerSel struct {
	once      bool
	cls       objc.Class
	setPlayer objc.SEL
	setBG     objc.SEL   // setBackgroundColor:
	color     objc.Class // UIColor
	black     objc.SEL   // blackColor
	cgColor   objc.SEL   // CGColor
}

// newNativePlane returns the plane over underlay, its layer's
// readyForDisplay observed by obs (0: not observed). It wraps
// core.ErrUnsupported when AVPlayerLayer is missing.
func newNativePlane(underlay func(layer uintptr) (Plane, error), obs objc.ID, log *slog.Logger) (*nativePlane, error) {
	if !layerSel.once {
		layerSel.cls = objc.GetClass("AVPlayerLayer")
		layerSel.setPlayer = objc.Sel("setPlayer:")
		layerSel.setBG = objc.Sel("setBackgroundColor:")
		layerSel.color = objc.GetClass("UIColor")
		layerSel.black = objc.Sel("blackColor")
		layerSel.cgColor = objc.Sel("CGColor")
		layerSel.once = layerSel.cls != 0
	}
	if layerSel.cls == 0 {
		return nil, fmt.Errorf("avf: no AVPlayerLayer: %w", core.ErrUnsupported)
	}

	return &nativePlane{underlay: underlay, obs: obs, log: log}, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Place puts the video at r, in DIPs of the view (UIKit points on tvOS);
// an empty r hides it. The first non-empty Place makes the layer and puts
// it under the view. It allocates nothing when r is unchanged.
func (p *nativePlane) Place(r geom.Rect) {
	if r.Empty() {
		if !p.wanted {
			return
		}
		p.wanted = false
		p.apply()

		return
	}
	if p.wanted && r == p.rect && p.plane != nil {
		return
	}
	p.wanted, p.rect = true, r
	p.ensure()
	if p.plane != nil {
		p.plane.SetFrame(r)
	}
	p.apply()
}

// setMirror mirrors the picture, or not: the layer's transform.scale.x is
// -1 or 1, set at once if the layer exists, else when it is made. It
// allocates nothing when the mirror is unchanged.
func (p *nativePlane) setMirror(on bool) {
	if p.mirror == on {
		return
	}
	p.mirror = on
	p.applyMirror()
}

// scaleX reads the layer's transform.scale.x back from Core Animation:
// -1 mirrored, 1 not; false before the layer exists. For device tests.
func (p *nativePlane) scaleX() (float64, bool) {
	if p.layer == 0 {
		return 0, false
	}
	var v float64
	var ok bool
	objc.WithPool(func() {
		n := objc.Call(p.layer, sel.valueForKeyPath, uintptr(av.keyScaleX))
		v, ok = cf.FloatValue(cf.Ref(n))
	})

	return v, ok
}

// showing tells whether the layer shows player, the engine's: a TV item
// is loaded and the view placed the layer, under the view.
func (p *nativePlane) showing(player objc.ID) bool {
	return p.layer != 0 && player != 0 && p.attached == player && p.tv
}

// readyForDisplay reads the layer's readyForDisplay: its player's
// current item has a frame on the layer. False before the layer exists.
// It allocates nothing.
func (p *nativePlane) readyForDisplay() bool {
	return p.layer != 0 && objc.CallBool(p.layer, sel.isReadyForDisplay)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// attach is told a TV item is coming: the plane shows if the view placed
// it, and makes its underlay if an earlier attempt failed.
func (p *nativePlane) attach(objc.ID) error {
	p.tv = true
	if p.wanted {
		p.ensure()
		if p.plane != nil {
			p.plane.SetFrame(p.rect)
		}
	}
	p.apply()

	return nil
}

// detach is told the item leaves: nothing to show until the next TV item.
func (p *nativePlane) detach(objc.ID) {
	p.tv = false
	p.apply()
}

// setPlayer is told the engine's player; a layer showing gets it.
func (p *nativePlane) setPlayer(player objc.ID) {
	p.player = player
	if p.attached != 0 && p.attached != player {
		p.attached = 0
	}
	p.apply()
}

// hide is told that nothing plays (Stop).
func (p *nativePlane) hide() {
	p.tv = false
	p.apply()
}

// close takes the layer from under the view and releases it.
func (p *nativePlane) close() {
	if p.plane != nil {
		p.plane.Remove()
		p.plane = nil
	}
	if p.layer != 0 {
		p.unwatch()
		objc.Call(p.layer, layerSel.setPlayer, 0)
		objc.Release(p.layer)
		p.layer = 0
	}
	p.attached, p.player, p.shown, p.wanted, p.tv = 0, 0, false, false, false
}

// count adds the layer.
func (p *nativePlane) count(st *Stats) {
	if p.layer != 0 {
		st.Layers++
	}
	if p.watching {
		st.Observations++
	}
}

// ensure makes the layer, black, then puts it under the view, unless
// done.
func (p *nativePlane) ensure() {
	if p.layer == 0 {
		p.layer = objc.New(layerSel.cls)
		if p.layer == 0 {
			p.warn(fmt.Errorf("avf: no AVPlayerLayer instance: %w", core.ErrUnsupported))

			return
		}
		if layerSel.color != 0 {
			// The layer keeps the CGColor; UIColor's black is the system's.
			objc.WithPool(func() {
				black := objc.Call(objc.ID(layerSel.color), layerSel.black)
				objc.Call(p.layer, layerSel.setBG, uintptr(objc.Call(black, layerSel.cgColor)))
			})
		}
		p.flipped = false
		p.applyMirror()
		p.watch()
	}
	if p.plane != nil || p.underlay == nil {
		return
	}
	pl, err := p.underlay(uintptr(p.layer))
	if err != nil {
		p.warn(err)

		return
	}
	p.plane = pl
	p.shown = false
}

// apply shows the underlay while the view placed it, with the player on
// it while a TV item is loaded; otherwise the layer has no player, and
// the underlay is hidden when the view did not place it.
func (p *nativePlane) apply() {
	if p.wanted && p.tv && p.plane != nil && p.player != 0 {
		if p.attached != p.player {
			objc.Call(p.layer, layerSel.setPlayer, uintptr(p.player))
			p.attached = p.player
		}
	} else if p.attached != 0 && p.layer != 0 {
		objc.Call(p.layer, layerSel.setPlayer, 0)
		p.attached = 0
	}
	if p.wanted && p.plane != nil {
		if !p.shown {
			p.plane.SetHidden(false)
			p.shown = true
		}

		return
	}
	if p.shown && p.plane != nil {
		p.plane.SetHidden(true)
	}
	p.shown = false
}

// watch has the engine's observer observe the layer's readyForDisplay,
// unless there is no observer.
func (p *nativePlane) watch() {
	if p.obs == 0 || p.layer == 0 || p.watching {
		return
	}
	objc.Call(p.layer, sel.addKVO, uintptr(p.obs), uintptr(av.keyReadyForDisplay), kvoOptions, 0)
	p.watching = true
}

// unwatch ends watch, before the layer is released.
func (p *nativePlane) unwatch() {
	if !p.watching {
		return
	}
	objc.Call(p.layer, sel.removeKVO, uintptr(p.obs), uintptr(av.keyReadyForDisplay), 0)
	p.watching = false
}

// warn logs the first failure to show the plane.
func (p *nativePlane) warn(err error) {
	if p.warned {
		return
	}
	p.warned = true
	p.log.Warn("avf: native video plane not under the view yet", "err", err)
}

// applyMirror sets the layer's transform.scale.x to -1 for a mirrored
// picture, 1 otherwise, unless it is so already or there is no layer
// yet. The change goes in a CATransaction with its actions disabled, so
// the layer turns at once rather than over the implicit 0.25 s
// animation. Allocation-free but for the CFNumber of the value.
func (p *nativePlane) applyMirror() {
	if p.layer == 0 || p.flipped == p.mirror {
		return
	}
	v := 1.0
	if p.mirror {
		v = -1
	}
	n := cf.Float(v)
	defer cf.Release(n)
	tx := av.transaction
	objc.Call(tx, sel.begin)
	objc.Call(tx, sel.setDisableActions, 1)
	objc.Call(p.layer, sel.setValueForKeyPath, uintptr(n), uintptr(av.keyScaleX))
	objc.Call(tx, sel.commit)
	p.flipped = p.mirror
}
