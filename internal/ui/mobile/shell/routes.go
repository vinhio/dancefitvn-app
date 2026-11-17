//go:build !tvos

package shell

import (
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// build is the route factory: it makes the screen of a route. The routes of P6
// are placeholders (screens.Stub) until their phases write them; a route nobody
// knows is a stub too, which is the prototype's `S.m.scr` falling back to a
// screen that is there.
func (s *Shell) build(r nav.Route) nav.Screen {
	e := s.e
	switch r.Screen {
	case env.RouteSplash:
		return screens.NewSplash(e, s.opts.HoldSplash)
	case env.RouteLogin:
		p, _ := r.Params.(env.LoginParams)

		return screens.NewLogin(e, p)
	case env.RouteUpgrade:
		p, _ := r.Params.(env.UpgradeParams)

		return screens.NewUpgrade(e, p)
	case env.RouteHome:
		return screens.NewHome(e)
	case env.RouteCourses:
		return screens.NewCourses(e)
	case env.RouteDetail:
		id, _ := r.Params.(string)

		return screens.NewDetail(e, id)
	case env.RouteOffline:
		id, _ := r.Params.(string)

		return screens.NewOffline(e, id)
	case env.RouteVideo:
		return screens.NewVideo(e)
	case env.RouteAudio:
		return screens.NewAudio(e)
	case env.RouteDone:
		return screens.NewDone(e)
	case env.RouteDownloads:
		return screens.NewDownloads(e)
	case env.RouteAccount:
		return screens.NewAccount(e)
	case env.RouteInfo:
		return screens.NewInfo(e)
	case env.RouteCoach:
		return screens.NewCoach(e)
	case env.RouteConnect:
		return screens.NewConnect(e)
	default:
		return screens.NewStub(e, r.Screen, -1)
	}
}
