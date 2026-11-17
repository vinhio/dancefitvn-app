//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/views/form_link.go at 49de3c2 (a form of
// kit.Input and kit.Button: the Enter of a field moves on or submits, one request at
// a time, the answer comes back through ui.Go and a stale one is dropped) and from
// dancefitvn-app/internal/ui/mobile/screens/info.go (the same screen shape over a
// PUT: the page, the button over its end, the kept view drawn first and the read
// behind it). The screen is MS.coach (index.html:486-494, M15C) over GET and PUT
// /me/coach-profile (docs/plan/03 section 3, local/plans/P6.md sections 1.5 and 5).

package screens

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The limits when the profile has not been read (`S.coach.socials.length>=6` of
// `addSoc`, index.html:615; max_places of the API, docs/plan/03 section 3) and the
// platforms of the prototype's pattern (`plat`, `saveCoach`, :486, :617).
const (
	defaultMaxSocials = 6
	defaultMaxPlaces  = 5
)

// defaultPlatforms are the platforms the prototype tests a link against.
var defaultPlatforms = []string{"facebook", "instagram", "youtube", "tiktok", "zalo", "threads"}

// socialField is the key of the server's message for the link at index i of the
// request: `socials[i].url`.
const (
	socialFieldPrefix = "socials["
	socialFieldSuffix = "].url"
)

// Coach is MS.coach, screen 15C: the public profile of the coach. The bio, the
// subjects the administrator sets (chips, with a note that they are not editable),
// the links to the social pages (each with its platform in front and a button that
// removes it, "+ Thêm liên kết" up to the limit of the API) and the places taught
// at, then "Cập nhật hồ sơ HLV" over the end of the page. It draws what the kept
// view of the account holds at once and reads GET /me/coach-profile behind it.
//
// "Cập nhật hồ sơ HLV" checks the links as the prototype does (a link that names none
// of the platforms is the toast "Chỉ hỗ trợ Facebook, …, Threads"), drops the empty
// links and places and sends the rest (PUT, one request at a time); a refusal is
// shown on the link the server names, success goes back with "Đã cập nhật hồ sơ HLV".
// Back leaves the edits behind: nothing is kept until the button saves them.
type Coach struct {
	ui.Base

	e    *env.Env
	page *nav.Page
	body *coachBody
	bar  *coachBar
	save *kit.Button

	bio              *kit.TextArea
	subjects         *chipFlow
	lock             *iconLine
	socBox, placeBox *rowStack
	addSoc, addPlace *inlineLink
	view             *inlineLink

	socials, places []*entryRow
	allowed         []string
	maxSocials      int
	maxPlaces       int
	publicURL       string

	busy    bool
	touched bool // the user typed or added or removed: a later read does not overwrite it
	cancel  func()
}

// coachBody is the page's content, in the order of the document (the order layout.json
// numbers its keys in): laid out by the numbers of the prototype's line boxes.
type coachBody struct {
	ui.Base

	top      *kit.TopBar
	lead     *kit.Label
	link     *inlineLink
	bioLabel *kit.Label
	bio      *kit.TextArea
	subjLbl  *kit.Label
	subjects *chipFlow
	lock     *iconLine
	socLbl   *kit.Label
	socBox   *rowStack
	addSoc   *inlineLink
	socHint  *kit.Label
	placeLbl *kit.Label
	placeBox *rowStack
	addPlace *inlineLink
}

var (
	_ nav.Tabbed   = (*Coach)(nil)
	_ nav.Scroller = (*Coach)(nil)
	_ ui.Mounter   = (*Coach)(nil)
	_ ui.Unmounter = (*Coach)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewCoach returns screen 15C (MS.coach, `$P:486-494`, M15C), no tab bar, filled with
// the coach profile the account view holds.
func NewCoach(e *env.Env) *Coach {
	s := &Coach{e: e, maxSocials: defaultMaxSocials, maxPlaces: defaultMaxPlaces, allowed: defaultPlatforms}
	s.bio = kit.NewTextArea("")
	s.bio.Label = e.T("coach.bio")
	s.bio.Name = "bio" // `<textarea id="bio">`
	s.bio.OnChange = func(string) { s.touched = true }
	s.subjects = &chipFlow{}
	s.lock = newIconLine(icon.Lock, coachLockIcon, coachLockStroke, e.T("coach.subjectsLocked"), coachLockSize, coachLockLine)
	s.lock.Name = nameMutedDiv
	s.socBox, s.placeBox = &rowStack{}, &rowStack{}
	s.addSoc = newInlineLink(e.T("coach.addSocial"), coachAddSize, 800, theme.InkText, icon.NameUnknown, 0, s.addSocial)
	s.addSoc.Name = nameAddSoc
	s.addPlace = newInlineLink(e.T("coach.addPlace"), coachAddSize, 800, theme.InkText, icon.NameUnknown, 0, s.addPlaceRow)
	s.addPlace.Name = nameAddPlace
	s.view = newInlineLink(e.T("coach.viewPage"), coachLinkSize, 800, theme.InkPrimary, icon.NameUnknown, 0, s.openPage)
	s.view.Name, s.view.underline = nameLinkView, true // `<a class="acc">`: the anchor's default underline

	lead := kit.Muted(e.T("coach.lead"), coachLeadSize)
	lead.Name, lead.Wrap = "p.muted", true
	hint := kit.Muted(e.T("coach.socialsHint"), coachSocHintSize)
	hint.Name = nameMutedDiv
	s.body = &coachBody{
		top:      kit.NewTopBar(e.T("account.coach"), e.T("a11y.back"), e.Act.Back),
		lead:     lead,
		link:     s.view,
		bioLabel: fieldLabel(e.T("coach.bio")),
		bio:      s.bio,
		subjLbl:  fieldLabel(e.T("coach.subjects")),
		subjects: s.subjects,
		lock:     s.lock,
		socLbl:   fieldLabel(e.T("coach.socials")),
		socBox:   s.socBox,
		addSoc:   s.addSoc,
		socHint:  hint,
		placeLbl: fieldLabel(e.T("coach.places")),
		placeBox: s.placeBox,
		addPlace: s.addPlace,
	}
	s.body.Add(s.body.top, s.body.lead, s.body.link, s.body.bioLabel, s.body.bio, s.body.subjLbl, s.body.subjects,
		s.body.lock, s.body.socLbl, s.body.socBox, s.body.addSoc, s.body.socHint, s.body.placeLbl, s.body.placeBox, s.body.addPlace)
	s.page = nav.NewPage(s.body, -1)

	s.save = kit.NewButton(kit.ButtonP, e.T("coach.save"), icon.Check)
	s.save.Name, s.save.IconSize, s.save.IconStroke = nameSaveCoch, 20, 2.4 // `ic('check',20,2.4)` (:493)
	s.save.OnPress = s.submit
	s.bar = &coachBar{button: s.save}
	s.bar.Add(s.save)
	s.Add(s.page, s.bar)

	if v := e.AccountView(); v.HasCoach {
		s.fill(v.Coach)
	}
	s.touched = false

	return s
}

// fieldLabel returns `.field label` (index.html:105): 11/700 upper case with 1.2
// of spacing in --tx2.
func fieldLabel(text string) *kit.Label {
	l := &kit.Label{Text: text, Role: theme.RoleBody, Size: 11, Weight: 700, Spacing: 1.2, Upper: true, Ink: theme.InkText2}
	l.Name = "label"

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tab is -1: no tab bar.
func (s *Coach) Tab() int { return -1 }

// Scroll returns the page's scroll.
func (s *Coach) Scroll() *ui.Scroll { return s.page.Scroll() }

// Bio returns the text area "Giới thiệu bản thân".
func (s *Coach) Bio() *kit.TextArea { return s.bio }

// Save returns the button "Cập nhật hồ sơ HLV" (`saveCoach`).
func (s *Coach) Save() *kit.Button { return s.save }

// Busy tells whether a save is under way: a second tap is ignored.
func (s *Coach) Busy() bool { return s.busy }

// Socials returns the input of each link, in order.
func (s *Coach) Socials() []*kit.Input { return inputsOf(s.socials) }

// Places returns the input of each place, in order.
func (s *Coach) Places() []*kit.Input { return inputsOf(s.places) }

// Platform returns the platform shown in front of the link at i.
func (s *Coach) Platform(i int) string {
	if i < 0 || i >= len(s.socials) {
		return ""
	}

	return s.socials[i].plat.Text
}

// Subjects returns the texts of the chips of the subjects.
func (s *Coach) Subjects() []string {
	var out []string
	for ch := range s.subjects.Children() {
		if c, ok := ch.(*kit.Chip); ok {
			out = append(out, c.Text)
		}
	}

	return out
}

// Mounted reads GET /me/coach-profile with the next turn: a view being mounted has no
// host, and ui.Go would run the read on the UI thread then.
func (s *Coach) Mounted(c *ui.Context) {
	c.AfterFunc(0, s.load)
}

// Unmounted ends the read and the save under way: leaving the screen drops them.
func (s *Coach) Unmounted(*ui.Context) {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// Layout fills the window with the page and puts the bar of the button over its
// bottom.
func (s *Coach) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(s.page, ui.Tight(cs.Max))
	c.Place(s.page, geom.Point{})
	bs := c.Layout(s.bar, ui.Constraints{Min: geom.Sz(cs.Max.W, 0), Max: geom.Sz(cs.Max.W, ui.Unbounded)})
	c.Place(s.bar, geom.Pt(0, cs.Max.H-bs.H))

	return cs.Max
}

// Layout puts the parts one under the other by the line boxes of the prototype's
// document: the margins and the offsets of the inline buttons are the numbers of
// layout.json (M15C), each in its constant.
func (b *coachBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	row := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
	loose := ui.Loose(geom.Sz(w, ui.Unbounded))
	y := float32(0)
	put := func(wd ui.Widget, cons ui.Constraints, at float32) geom.Size {
		sz := c.Layout(wd, cons)
		c.Place(wd, geom.Pt(0, at))

		return sz
	}

	y += put(b.top, row, y).H
	y += coachLeadTop
	y += put(b.lead, row, y).H
	put(b.link, loose, y+coachLinkTop)
	y += coachLinkLine

	// "Giới thiệu bản thân"
	y += put(b.bioLabel, row, y).H + 8
	put(b.bio, row, y)
	y += 80 + coachGapAfterArea

	// "Bộ môn giảng dạy"
	y += put(b.subjLbl, row, y).H + 8
	y += put(b.subjects, row, y).H
	if !b.lock.Hidden {
		y += coachLockTop
		y += put(b.lock, loose, y).H
	}
	y += kit.FieldGap

	// "Trang mạng xã hội"
	y += put(b.socLbl, row, y).H + 8
	y += put(b.socBox, row, y).H
	put(b.addSoc, loose, y+coachSocTop)
	put(b.socHint, row, y+coachSocHintTop)
	y += coachSocHintTop + b.socHint.Node().Frame().H + kit.FieldGap

	// "Địa điểm dạy"
	y += put(b.placeLbl, row, y).H + 8
	y += put(b.placeBox, row, y).H
	put(b.addPlace, loose, y+coachPlaceTop)
	y += coachPlaceLine + kit.FieldGap

	y += coachEnd

	return cs.Constrain(geom.Sz(w, y))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// inputsOf returns the inputs of rows.
func inputsOf(rows []*entryRow) []*kit.Input {
	out := make([]*kit.Input, len(rows))
	for i, r := range rows {
		out[i] = r.input
	}

	return out
}

// fill shows the profile: the bio, the chips of the subjects, the links and the
// places (each as many rows as there are), with the limits and platforms of the API.
func (s *Coach) fill(c model.CoachProfile) {
	s.bio.SetText(c.Bio)
	s.publicURL = c.PublicURL
	s.allowed = defaultPlatforms
	if len(c.AllowedPlatforms) > 0 {
		s.allowed = c.AllowedPlatforms
	}
	s.maxSocials, s.maxPlaces = defaultMaxSocials, defaultMaxPlaces
	if c.MaxSocials > 0 {
		s.maxSocials = c.MaxSocials
	}
	if c.MaxPlaces > 0 {
		s.maxPlaces = c.MaxPlaces
	}

	s.subjects.Clear()
	for _, name := range c.Subjects {
		s.subjects.Add(newSubjectChip(name))
	}
	kit.SetHidden(s.lock, c.SubjectsEditable) // the administrator's note is for a list the coach cannot edit

	s.socBox.Clear()
	s.socials = s.socials[:0]
	for _, sc := range c.Socials {
		s.appendRow(true, sc.URL)
	}
	s.placeBox.Clear()
	s.places = s.places[:0]
	for _, p := range c.Places {
		s.appendRow(false, p)
	}
	s.syncPlatforms()
	s.body.Relayout()
}

// newSubjectChip returns the chip of a subject (index.html:489): 28 tall, padded 14,
// #2F6FE0 with white 800 text, 0.8 of spacing, in upper case.
func newSubjectChip(name string) *kit.Chip {
	c := kit.NewChip(kit.ChipKindUnknown, strings.ToUpper(name))
	c.Height, c.PadX, c.Weight, c.Spacing = coachChipHeight, coachChipPad, 800, coachChipSpacing
	c.Bg, c.Fg = chipBlue, color.White

	return c
}

// appendRow adds a row of a link (social) or a place holding text, and returns it.
func (s *Coach) appendRow(social bool, text string) *entryRow {
	r := &entryRow{}
	in := kit.NewInput(icon.NameUnknown, "")
	in.Search = true // `.inp` at `height:44px;border-radius:22px`
	in.OnChange = func(string) {
		s.touched = true
		in.SetError(false) // typing takes back what the server refused
	}
	r.input = in
	if social {
		in.Placeholder, in.Label = s.e.T("coach.socialPh"), s.e.T("coach.socials")
		in.Keyboard = platform.KeyboardURL
		r.plat = &kit.Label{Size: coachPlatSize, Weight: 700, Upper: true, Ink: theme.InkPrimary} // `<b class="acc" style="font-size:11px;text-transform:uppercase">`
		in.Leading = r.plat
		r.rm = newRmButton(nameRmSoc, s.e.T("a11y.removeLink"), func() { s.removeRow(r, true) })
	} else {
		in.Icon, in.IconSize = icon.Pin, coachPinIcon // `ic('pin',18)`
		in.Placeholder, in.Label = s.e.T("coach.placePh"), s.e.T("coach.places")
		in.Keyboard = platform.KeyboardText
		r.rm = newRmButton(nameRmPlace, s.e.T("a11y.removePlace"), func() { s.removeRow(r, false) })
	}
	in.EnterAction = platform.EnterDone
	in.OnSubmit = func(string) { s.blur() }
	in.SetText(text)
	in.Field().Select(0, 0) // a `value` shows from its start, not scrolled to its end
	r.Add(in, r.rm)
	if social {
		s.socials = append(s.socials, r)
		s.socBox.Add(r)
	} else {
		s.places = append(s.places, r)
		s.placeBox.Add(r)
	}

	return r
}

// syncPlatforms sets the platform in front of each link from the text it holds now:
// the first of the platforms the text names, else "link" (`plat`, :486). The
// prototype does it each time the screen is drawn again, which an add or a remove
// is; typing changes nothing until then (Q-P6-18).
func (s *Coach) syncPlatforms() {
	for _, r := range s.socials {
		plat := s.platformOf(r.input.Text())
		r.plat.SetText(plat)
		// each link is told apart by its platform for a screen reader: the field
		// and its remove button would all read the same otherwise
		r.input.Label = s.e.T("coach.socials") + ", " + plat
		r.rm.label = s.e.T("a11y.removeLink") + ", " + plat
	}
}

// platformOf returns the platform a link names, "link" for none.
func (s *Coach) platformOf(url string) string {
	if p, ok := s.platformNamed(url); ok {
		return p
	}

	return s.e.T("coach.linkLabel")
}

// platformNamed returns the first platform the link names (its text, lower-cased,
// holds it), as `saveCoach`'s pattern does.
func (s *Coach) platformNamed(url string) (string, bool) {
	low := strings.ToLower(url)
	for _, p := range s.allowed {
		if strings.Contains(low, p) {
			return p, true
		}
	}

	return "", false
}

// addSocial is `addSoc` (`$P:615`): a new empty link, or at the limit the toast
// "Tối đa {n} liên kết".
func (s *Coach) addSocial() {
	if len(s.socials) >= s.maxSocials {
		s.e.Act.Toast(s.e.Fill("toast.maxSocialsTpl", "n", strconv.Itoa(s.maxSocials)))

		return
	}
	s.touched = true
	s.appendRow(true, "")
	s.syncPlatforms()
	s.body.Relayout()
}

// addPlaceRow is `addPlace` (`$P:616`): a new empty place, or at the limit of the API
// the toast "Tối đa {n} địa điểm".
func (s *Coach) addPlaceRow() {
	if len(s.places) >= s.maxPlaces {
		s.e.Act.Toast(s.e.Fill("toast.maxPlacesTpl", "n", strconv.Itoa(s.maxPlaces)))

		return
	}
	s.touched = true
	s.appendRow(false, "")
	s.body.Relayout()
}

// removeRow is `rmSoc` and `rmPlace` (`$P:615-616`): the row goes.
func (s *Coach) removeRow(r *entryRow, social bool) {
	s.touched = true
	if social {
		s.socBox.Remove(r)
		s.socials = removeRow(s.socials, r)
		s.syncPlatforms()
	} else {
		s.placeBox.Remove(r)
		s.places = removeRow(s.places, r)
	}
	s.body.Relayout()
}

// removeRow returns rows without r.
func removeRow(rows []*entryRow, r *entryRow) []*entryRow {
	out := rows[:0]
	for _, x := range rows {
		if x != r {
			out = append(out, x)
		}
	}
	clear(rows[len(out):])

	return out
}

// openPage opens the public page of the coach in the browser (`<a href>`).
func (s *Coach) openPage() {
	if s.publicURL != "" {
		s.e.OpenURL(s.publicURL)
	}
}

// blur takes the focus away, which hides the keyboard.
func (s *Coach) blur() {
	if v := s.View(); v != nil {
		v.Focus(nil)
	}
}

// load reads GET /me/coach-profile and fills the form if the user has changed
// nothing meanwhile.
func (s *Coach) load() {
	s.cancel = ui.Go(s, func(ctx context.Context) (model.CoachProfile, error) {
		return s.e.Account().Coach(ctx)
	}, func(_ *ui.Context, c model.CoachProfile, err error) {
		s.cancel = nil
		switch {
		case err == nil:
			if !s.touched && !s.busy {
				s.fill(c)
				s.touched = false
			}
		case errors.Is(err, context.Canceled):
		case s.e.Refused(err):
		default: // the form stays as it is
		}
	})
}

// submit is `saveCoach` (`$P:617`): a link that names none of the platforms is the
// toast "Chỉ hỗ trợ Facebook, Instagram, YouTube, TikTok, Zalo, Threads" and nothing
// is sent (the input does not go red: only the server's `invalid_social` marks
// one); else the empty links and places are dropped and the rest goes to the
// server, one request at a time.
func (s *Coach) submit() {
	if s.busy {
		return
	}
	req := model.CoachUpdateRequest{Bio: strings.TrimSpace(s.bio.Text()), Socials: []model.SocialURL{}, Places: []string{}}
	bad := false
	for _, r := range s.socials {
		u := strings.TrimSpace(r.input.Text())
		if u == "" {
			continue
		}
		if _, ok := s.platformNamed(u); !ok {
			bad = true // `saveCoach` only shows the toast: no red border for the local check
		}
		req.Socials = append(req.Socials, model.SocialURL{URL: u})
	}
	if bad {
		s.e.Act.Toast(s.e.T("toast.badSocial"))

		return
	}
	for _, r := range s.places {
		if p := strings.TrimSpace(r.input.Text()); p != "" {
			req.Places = append(req.Places, p)
		}
	}
	sent := s.sentRows()
	s.busy = true
	s.blur()
	if s.cancel != nil {
		s.cancel() // a read that is still out must not overwrite what is saved
	}
	s.cancel = ui.Go(s, func(ctx context.Context) (model.CoachProfile, error) {
		return s.e.Account().UpdateCoach(ctx, req)
	}, func(_ *ui.Context, c model.CoachProfile, err error) { s.saved(sent, c, err) })
}

// sentRows returns the rows of the links that go in the request, in order: the
// server names a link by its place in the request, with the empty ones dropped.
func (s *Coach) sentRows() []*entryRow {
	var out []*entryRow
	for _, r := range s.socials {
		if strings.TrimSpace(r.input.Text()) != "" {
			out = append(out, r)
		}
	}

	return out
}

// saved takes the answer of PUT /me/coach-profile: success goes back (`back()`) with
// the toast of the prototype; a refusal of a link marks the input the server names.
func (s *Coach) saved(sent []*entryRow, _ model.CoachProfile, err error) {
	s.busy = false
	s.cancel = nil
	if err == nil {
		s.e.Act.Back()
		s.e.Act.Toast(s.e.T("toast.coachSaved"))

		return
	}
	if errors.Is(err, context.Canceled) || s.e.Refused(err) {
		return
	}
	fail := env.FailureOf(err)
	if fail.Kind == env.FailureInvalidSocial {
		for key := range fail.Fields {
			if i, ok := socialIndex(key); ok && i < len(sent) {
				sent[i].input.SetError(true)
			}
		}
	}
	if msg := s.e.FailureToast(fail); msg != "" {
		s.e.Act.Toast(msg)
	}
}

// socialIndex returns i of the server's field `socials[i].url`.
func socialIndex(key string) (int, bool) {
	rest, ok := strings.CutPrefix(key, socialFieldPrefix)
	if !ok {
		return 0, false
	}
	num, ok := strings.CutSuffix(rest, socialFieldSuffix)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(num)

	return i, err == nil && i >= 0
}
