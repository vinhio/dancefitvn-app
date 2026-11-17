//go:build tvos

package app

import "dancefitvn-app/internal/store"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// devPrefs applies DFV_PREFS of a developer's build through the calls of
// TV12 (CycleQualityTV until the quality asked, SetAutoNext), then logs the
// preferences read: the probe of scripts/tvos-store-check.sh app. The
// build that ships does nothing. UI thread, once the preferences are open.
func (p *tvPlayback) devPrefs() {
	if !devBuild {
		return
	}
	if spec := devEnv(prefsEnv); spec != "" {
		set, err := parseDevPrefs(spec)
		if err != nil {
			p.w.deps.Logf("dfv: prefs: %v", err)
		} else {
			p.applyDevPrefs(set)
		}
	}
	p.w.deps.Logf("dfv: prefs quality=%s autonext=%t", p.prefs.Quality(), p.prefs.AutoNext())
}

// applyDevPrefs sets what set asks for, as the settings screen does.
func (p *tvPlayback) applyDevPrefs(set devPrefSet) {
	for range 4 {
		if set.quality == store.QualityUnknown || p.prefs.Quality() == set.quality {
			break
		}
		p.prefs.CycleQualityTV()
	}
	if set.setAutoNext && p.prefs.AutoNext() != set.autoNext {
		p.prefs.SetAutoNext(set.autoNext)
	}
}
