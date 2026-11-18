//go:build tvos

package screens

import (
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/tvkit"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the player's panel (index.html:689-693), in DIPs.
const (
	// panelTitleSize, panelTitleBottom are `.d` at `font-size:44px;margin-bottom:26px`.
	panelTitleSize, panelTitleBottom float32 = 44, 26
	// panelEyebrowSize, panelEyebrowBottom, panelEyebrowLeft are `.eb` at
	// `font-size:22px;margin:0 0 12px 26px`; the second one has 26 above it.
	panelEyebrowSize, panelEyebrowBottom, panelEyebrowLeft, panelEyebrowTop float32 = 22, 12, 26, 26
	// panelOptBottom is `.topt{margin-bottom:6px}`; panelCloseTop is `oc`'s
	// `margin-top:16px`.
	panelOptBottom, panelCloseTop float32 = 6, 16
)

// The keys of the panel's controls: `data-fid` of `.tpanel`.
const (
	keyPanelSpeed  = "o"
	keyPanelToggle = "ot"
	keyPanelClose  = "oc"
)

// panelSpeeds are the speeds of the panel, `[.5,.75,1,1.25]` (index.html:690).
var panelSpeeds = [...]float64{.5, .75, 1, 1.25}

// panelToggles are the three toggles, `[['mirror',...],['loop',...],['beat',...]]`
// (:691).
const (
	toggleMirror = iota
	toggleLoop
	toggleBeat
	toggleCount
)

// playerPanel is `.tpanel` of TS.player (TV08): the speeds `o0..o3`, the toggles
// `ot0..ot2` (Lật gương, Lặp đoạn A–B, Thanh đếm nhịp) and `oc` (Đóng).
type playerPanel struct {
	*tvkit.Panel

	speeds [len(panelSpeeds)]*tvkit.Opt
	toggle [toggleCount]*tvkit.Opt
	close  *tvkit.Opt

	on, off string
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newPanel makes the panel of the player, showing what it holds now.
func (s *Player) newPanel() *playerPanel {
	e, pb := s.e, s.pb
	p := &playerPanel{Panel: tvkit.NewPanel(), on: e.T("tv.on"), off: e.T("tv.off")}

	title := kit.Display(e.T("options.title"), panelTitleSize)
	title.Heading = 1 // the panel is a screen of its own over the player
	p.Put(title, 0, panelTitleBottom, 0)
	p.Put(panelEyebrow(e.T("tv.panel.speed")), 0, panelEyebrowBottom, panelEyebrowLeft)
	for i, v := range panelSpeeds {
		o := tvkit.NewOpt(keyPanelSpeed+itoa(i), tvkit.OptRow, jsNumber(v)+"x")
		o.Now = e.T("a11y.current") // read after the hint of the speed that is on
		switch v {
		case .5:
			o.Sub = e.T("tv.panel.slow")
		case 1:
			o.Sub = e.T("tv.panel.normal")
		}
		o.OnSelect = func() { pb.SetRate(v) } // `tspeed`
		p.speeds[i] = o
		p.Put(o, 0, panelOptBottom, 0)
	}
	p.Put(panelEyebrow(e.T("tv.panel.visual")), panelEyebrowTop, panelEyebrowBottom, panelEyebrowLeft)
	names := [toggleCount]string{e.T("options.mirror"), e.T("options.loop"), e.T("tv.panel.beat")}
	acts := [toggleCount]func(){
		func() { pb.SetMirror(!pb.Mirror()) },
		func() { pb.SetLoop(!pb.Loop()) }, // on, the loop begins at the position, 30 s long (K38); no B to edit on TV
		func() { pb.SetBeat(!pb.Beat()) },
	}
	for i := range p.toggle {
		o := tvkit.NewOpt(keyPanelToggle+itoa(i), tvkit.OptRow, names[i]) // `ttoggle`
		o.Switch = true
		o.OnSelect = acts[i]
		p.toggle[i] = o
		p.Put(o, 0, panelOptBottom, 0)
	}
	p.close = tvkit.NewOpt(keyPanelClose, tvkit.OptClose, e.T("tv.panel.close"))
	p.close.OnSelect = s.closePanel // `tclose`
	p.Put(p.close, panelCloseTop, panelOptBottom, 0)

	p.syncRate(pb.Rate())
	p.syncOptions(pb)

	return p
}

// panelEyebrow is `.eb` at 22 px.
func panelEyebrow(txt string) *kit.Label {
	l := kit.Eyebrow(txt)
	l.Size = panelEyebrowSize
	l.Heading = 2 // the title of a group of the panel

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// syncRate puts the check on the speed that is on.
func (p *playerPanel) syncRate(r float64) {
	for i, o := range p.speeds {
		o.SetCheck(panelSpeeds[i] == r)
	}
}

// syncOptions writes "Bật" or "Tắt" at each toggle.
func (p *playerPanel) syncOptions(pb env.Playback) {
	p.setToggle(toggleMirror, pb.Mirror())
	p.setToggle(toggleLoop, pb.Loop())
	p.setToggle(toggleBeat, pb.Beat())
}

// item returns the control of the key, nil when there is none.
func (p *playerPanel) item(key string) *tvkit.Item {
	for _, o := range p.speeds {
		if o.Key == key {
			return &o.Item
		}
	}
	for _, o := range p.toggle {
		if o.Key == key {
			return &o.Item
		}
	}
	if p.close.Key == key {
		return &p.close.Item
	}

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// setToggle writes the value of toggle i.
func (p *playerPanel) setToggle(i int, on bool) {
	p.toggle[i].On = on
	if on {
		p.toggle[i].SetValue(p.on)
	} else {
		p.toggle[i].SetValue(p.off)
	}
}

// openPanel is `topts` (index.html:742): the panel joins the screen and takes the
// focus, which stays in it until it closes.
func (s *Player) openPanel() {
	if s.panel != nil || s.e.Act == nil {
		return
	}
	s.panel = s.newPanel()
	s.Add(s.panel.Panel)
	s.e.Act.OpenPanel(s.panel.Panel)
}

// closePanel is `tclose` (:742): the panel goes and the focus is on `p2`.
func (s *Player) closePanel() {
	s.dropPanel()
	if s.e.Act != nil {
		s.e.Act.ClosePanel(keyPlayerOpts)
	}
}

// dropPanel takes the panel off the screen.
func (s *Player) dropPanel() {
	if s.panel != nil {
		s.Remove(s.panel.Panel)
		s.panel = nil
		s.Relayout()
		s.touch() // K46: the 3 s of the chrome begin when the panel is gone
	}
}
