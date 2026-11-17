// Adapted from jplayer-tv/internal/media/avf/resolve_tvos.go at 781bee5.

//go:build ios && tvos

package avftv

import (
	"fmt"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/cf"
	"github.com/gdeskdev/desk/foundation/ffi/dl"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// observerClass is the Objective-C class whose instances receive the
// engine's key-value observations and notifications. A class belongs to
// the process, so it is defined once, under a name only this package
// uses.
const observerClass = "DFVTVPlaybackObserver"

// The Objective-C values the engine needs, resolved once per process by
// resolve: classes, selectors (kept in variables so that the drain and
// the tick never look one up by name), the constants of AVFoundation,
// AVFAudio and Foundation, the key paths, and the observer class.
var av struct {
	once sync.Once
	err  error

	player, item, url, session, center, number, transaction objc.ID // classes

	observer objc.Class

	// Key paths observed or set: owned NSStrings, kept for the process.
	// keyScaleX is the layer's mirror (transform.scale.x, 04 §2.3);
	// keyReadyForDisplay the layer's first frame shown (P7.5a).
	keyStatus, keyTimeControl, keyScaleX, keyReadyForDisplay objc.ID

	// Notification names (AVPlayerItem.h:28-31) and NSUnderlyingErrorKey.
	noteFailed, noteStalled, noteErrorLog, noteEnded, underlyingKey objc.ID

	// AVAudioTimePitchAlgorithmTimeDomain (AVAudioProcessingSettings.h):
	// the pitch kept at any speed (04 §2.2).
	pitchTimeDomain objc.ID

	// AVAudioSessionCategoryPlayback and AVAudioSessionModeDefault.
	category, mode objc.ID

	// AVAudioSessionInterruptionNotification and its userInfo keys
	// (AVAudioSession.h, AVAudioSessionTypes.h).
	noteInterruption, keyInterruptionType, keyInterruptionOption objc.ID
}

// sel holds the selectors the engine sends.
var sel struct {
	alloc, play, pause, status, timeControlStatus, replaceItem, errorOf       objc.SEL
	setVolume, setMuted, volume, isMuted                                      objc.SEL
	addKVO, removeKVO, addNote, removeNote, defaultCenter                     objc.SEL
	urlWithString, itemWithURL                                                objc.SEL
	setMaxResolution, setPeakBitRate, maxResolution, peakBitRate              objc.SEL
	domain, code, userInfo, objectForKey, errorLog, events, lastObject, count objc.SEL
	errorStatusCode, errorDomain, sharedInstance, setCategory, setActive      objc.SEL
	numberWithUnsignedInt, numberWithInt, reasonForWaiting, isEqualToString   objc.SEL
	itemFailed, itemStalled, itemErrorLog, itemEnded                          objc.SEL
	sessionInterrupted, unsignedIntegerValue                                  objc.SEL
	// Added for DanceFITVN: the tick's buffered ranges, the pitch, the
	// mirror on the layer (key-value coding) in a transaction without
	// animation.
	loadedTimeRanges, objectAtIndex, setPitchAlgorithm, pitchAlgorithm    objc.SEL
	setValueForKeyPath, valueForKeyPath, begin, commit, setDisableActions objc.SEL
	// Added for DanceFITVN (P7.5a): the first frame on the layer and the
	// size of the picture.
	isReadyForDisplay, presentationSize objc.SEL
}

// cgSize is CoreGraphics' CGSize (CGFloat is a float64 on arm64).
type cgSize struct{ W, H float64 }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// resolved runs resolve once and returns its error, the same every time.
func resolved() error {
	av.once.Do(func() { av.err = resolve() })

	return av.err
}

// resolve loads the frameworks, reads the constants, finds the classes,
// registers the selectors and defines the observer class.
func resolve() error {
	if err := objc.Init(); err != nil {
		return fmt.Errorf("avf: resolve: %w: %w", core.ErrUnsupported, err)
	}
	for _, f := range []string{"Foundation", "AVFoundation", "CoreMedia", "QuartzCore"} {
		if err := objc.LoadFramework(f); err != nil {
			return fmt.Errorf("avf: resolve: %w: %w", core.ErrUnsupported, err)
		}
	}
	for _, c := range []struct {
		dst  *objc.ID
		name string
	}{
		{&av.player, "AVPlayer"}, {&av.item, "AVPlayerItem"}, {&av.url, "NSURL"},
		{&av.session, "AVAudioSession"}, {&av.center, "NSNotificationCenter"},
		{&av.number, "NSNumber"}, {&av.transaction, "CATransaction"},
	} {
		if *c.dst = objc.ID(objc.GetClass(c.name)); *c.dst == 0 {
			return fmt.Errorf("avf: resolve: no class %s: %w", c.name, core.ErrUnsupported)
		}
	}
	if err := resolveConstants(); err != nil {
		return err
	}
	resolveSelectors()
	av.keyStatus, av.keyTimeControl = objc.NSString("status"), objc.NSString("timeControlStatus")
	av.keyScaleX = objc.NSString("transform.scale.x")
	av.keyReadyForDisplay = objc.NSString("readyForDisplay")
	cls, err := objc.DefineClass(observerClass, "NSObject", nil,
		objc.Method{Selector: "observeValueForKeyPath:ofObject:change:context:", Fn: onChanged},
		objc.Method{Selector: "itemFailed:", Fn: onFailed},
		objc.Method{Selector: "itemStalled:", Fn: onStalled},
		objc.Method{Selector: "itemErrorLog:", Fn: onErrorLog},
		objc.Method{Selector: "itemEnded:", Fn: onEnded},
		objc.Method{Selector: "sessionInterrupted:", Fn: onInterruption},
	)
	if err != nil {
		return fmt.Errorf("avf: resolve: %w", err)
	}
	av.observer = cls

	return nil
}

// resolveConstants reads the notification names, the audio session's
// category and mode, the pitch algorithm and NSUnderlyingErrorKey from
// their frameworks' symbols.
func resolveConstants() error {
	libs := map[string]string{
		"AVFoundation": "/System/Library/Frameworks/AVFoundation.framework/AVFoundation",
		// AVAudioSession lives in AVFAudio (AVAudioSessionTypes.h), which
		// AVFoundation re-exports.
		"AVFAudio":   "/System/Library/Frameworks/AVFAudio.framework/AVFAudio",
		"Foundation": "/System/Library/Frameworks/Foundation.framework/Foundation",
	}
	handles := make(map[string]uintptr, len(libs))
	for name, path := range libs {
		lib, err := dl.Open(path)
		if err != nil {
			return fmt.Errorf("avf: resolve: %w: %w", core.ErrUnsupported, err)
		}
		handles[name] = lib.Handle()
	}
	for _, k := range []struct {
		dst  *cf.Ref
		lib  string
		name string
	}{
		{(*cf.Ref)(&av.noteFailed), "AVFoundation", "AVPlayerItemFailedToPlayToEndTimeNotification"},
		{(*cf.Ref)(&av.noteStalled), "AVFoundation", "AVPlayerItemPlaybackStalledNotification"},
		{(*cf.Ref)(&av.noteErrorLog), "AVFoundation", "AVPlayerItemNewErrorLogEntryNotification"},
		{(*cf.Ref)(&av.noteEnded), "AVFoundation", "AVPlayerItemDidPlayToEndTimeNotification"},
		{(*cf.Ref)(&av.pitchTimeDomain), "AVFoundation", "AVAudioTimePitchAlgorithmTimeDomain"},
		{(*cf.Ref)(&av.category), "AVFAudio", "AVAudioSessionCategoryPlayback"},
		{(*cf.Ref)(&av.mode), "AVFAudio", "AVAudioSessionModeDefault"},
		{(*cf.Ref)(&av.noteInterruption), "AVFAudio", "AVAudioSessionInterruptionNotification"},
		{(*cf.Ref)(&av.keyInterruptionType), "AVFAudio", "AVAudioSessionInterruptionTypeKey"},
		{(*cf.Ref)(&av.keyInterruptionOption), "AVFAudio", "AVAudioSessionInterruptionOptionKey"},
		{(*cf.Ref)(&av.underlyingKey), "Foundation", "NSUnderlyingErrorKey"},
	} {
		v, err := cf.Constant(handles[k.lib], k.name)
		if err != nil || v == 0 {
			return fmt.Errorf("avf: resolve: constant %s: %w", k.name, core.ErrUnsupported)
		}
		*k.dst = v
	}

	return nil
}

// resolveSelectors registers every selector the engine sends.
func resolveSelectors() {
	s := &sel
	for _, x := range []struct {
		dst  *objc.SEL
		name string
	}{
		{&s.alloc, "alloc"}, {&s.play, "play"}, {&s.pause, "pause"}, {&s.status, "status"},
		{&s.timeControlStatus, "timeControlStatus"}, {&s.replaceItem, "replaceCurrentItemWithPlayerItem:"},
		{&s.errorOf, "error"}, {&s.setVolume, "setVolume:"}, {&s.setMuted, "setMuted:"},
		{&s.volume, "volume"}, {&s.isMuted, "isMuted"},
		{&s.addKVO, "addObserver:forKeyPath:options:context:"},
		{&s.removeKVO, "removeObserver:forKeyPath:context:"},
		{&s.addNote, "addObserver:selector:name:object:"},
		{&s.removeNote, "removeObserver:name:object:"}, {&s.defaultCenter, "defaultCenter"},
		{&s.urlWithString, "URLWithString:"}, {&s.itemWithURL, "playerItemWithURL:"},
		{&s.setMaxResolution, "setPreferredMaximumResolution:"}, {&s.setPeakBitRate, "setPreferredPeakBitRate:"},
		{&s.maxResolution, "preferredMaximumResolution"}, {&s.peakBitRate, "preferredPeakBitRate"},
		{&s.domain, "domain"}, {&s.code, "code"}, {&s.userInfo, "userInfo"}, {&s.objectForKey, "objectForKey:"},
		{&s.errorLog, "errorLog"}, {&s.events, "events"}, {&s.lastObject, "lastObject"}, {&s.count, "count"},
		{&s.errorStatusCode, "errorStatusCode"}, {&s.errorDomain, "errorDomain"},
		{&s.sharedInstance, "sharedInstance"}, {&s.setCategory, "setCategory:mode:options:error:"},
		{&s.setActive, "setActive:error:"},
		{&s.numberWithUnsignedInt, "numberWithUnsignedInt:"}, {&s.numberWithInt, "numberWithInt:"},
		{&s.reasonForWaiting, "reasonForWaitingToPlay"},
		{&s.isEqualToString, "isEqualToString:"},
		{&s.itemFailed, "itemFailed:"}, {&s.itemStalled, "itemStalled:"}, {&s.itemErrorLog, "itemErrorLog:"},
		{&s.itemEnded, "itemEnded:"},
		{&s.loadedTimeRanges, "loadedTimeRanges"}, {&s.objectAtIndex, "objectAtIndex:"},
		{&s.setPitchAlgorithm, "setAudioTimePitchAlgorithm:"}, {&s.pitchAlgorithm, "audioTimePitchAlgorithm"},
		{&s.setValueForKeyPath, "setValue:forKeyPath:"}, {&s.valueForKeyPath, "valueForKeyPath:"},
		{&s.begin, "begin"}, {&s.commit, "commit"}, {&s.setDisableActions, "setDisableActions:"},
		{&s.sessionInterrupted, "sessionInterrupted:"}, {&s.unsignedIntegerValue, "unsignedIntegerValue"},
		{&s.isReadyForDisplay, "isReadyForDisplay"}, {&s.presentationSize, "presentationSize"},
	} {
		*x.dst = objc.Sel(x.name)
	}
}
