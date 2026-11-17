// Adapted from jplayer-mobile/internal/media/avf/classes_ios.go at 49de3c2.

//go:build ios && !tvos && cgo

package avf

import (
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/cf"
	"github.com/gdeskdev/desk/foundation/ffi/dl"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The names of the classes the package defines, once per process.
const (
	observerClassName = "JPAVFObserver"
	sessionClassName  = "JPAVFSessionObserver"
)

// The selectors of the observers' methods for notifications.
const (
	selEnded        = "jpEnded:"
	selFailed       = "jpFailed:"
	selInterruption = "jpInterruption:"
	selRouteChange  = "jpRouteChange:"
	selMediaReset   = "jpMediaReset:"
	selForeground   = "jpWillEnterForeground:"
)

// Values of the SDK, each checked against its header.
const (
	itemStatusReady  = 1 // AVPlayerItemStatusReadyToPlay, AVPlayerItem.h:48
	itemStatusFailed = 2 // AVPlayerItemStatusFailed, AVPlayerItem.h:50

	controlPaused  = 0 // AVPlayerTimeControlStatusPaused, AVPlayer.h:190
	controlWaiting = 1 // AVPlayerTimeControlStatusWaitingToPlayAtSpecifiedRate, AVPlayer.h:192
	controlPlaying = 2 // AVPlayerTimeControlStatusPlaying, AVPlayer.h:194

	kvoOptionNew = 1 // NSKeyValueObservingOptionNew, NSKeyValueObserving.h:22

	keyValueLoaded = 2 // AVKeyValueStatusLoaded, AVAsynchronousKeyValueLoading.h:19

	interruptionBegan    = 1 // AVAudioSessionInterruptionTypeBegan, AVAudioSessionTypes.h:608
	interruptionResume   = 1 // AVAudioSessionInterruptionOptionShouldResume, AVAudioSessionTypes.h:616
	reasonAppSuspended   = 1 // AVAudioSessionInterruptionReasonAppWasSuspended, AVAudioSessionTypes.h:642
	routeOldDeviceAbsent = 2 // AVAudioSessionRouteChangeReasonOldDeviceUnavailable, AVAudioSessionTypes.h:397

	pixelFormatBGRA = 'B'<<24 | 'G'<<16 | 'R'<<8 | 'A' // kCVPixelFormatType_32BGRA, CVPixelBuffer.h

	taskVMInfo      = 22  // TASK_VM_INFO, mach/task_info.h:333
	physFootprintAt = 18  // phys_footprint, the 19th 64-bit word of task_vm_info (task_info.h:358)
	taskVMInfoWords = 128 // room for every revision of task_vm_info
)

// The frameworks the package reads constants and functions from, by path.
const (
	pathAVFoundation = "/System/Library/Frameworks/AVFoundation.framework/AVFoundation"
	pathAVFAudio     = "/System/Library/Frameworks/AVFAudio.framework/AVFAudio"
	pathFoundation   = "/System/Library/Frameworks/Foundation.framework/Foundation"
	pathCoreVideo    = "/System/Library/Frameworks/CoreVideo.framework/CoreVideo"
	pathLibSystem    = "/usr/lib/libSystem.B.dylib"
	pathUIKit        = "/System/Library/Frameworks/UIKit.framework/UIKit"
)

// The formats of the errors of initAVF.
const (
	frameworkMissing = "avf: load %s: %w"
	constantMissing  = "avf: constant %s: %w"
	classMissing     = "avf: class %s: %w"
)

// assetKeyCount is how many asset keys the item loads.
const assetKeyCount = 2

// av holds what the package resolves once per process (initAVF): the
// classes, the NSString constants of the frameworks (not owned), the
// strings and the array it makes (owned for the life of the process), the C
// functions, and the selectors. Read-only after initAVF.
var av struct {
	cls struct {
		player, item, asset, url, output, center, session, array objc.ID
		observer, sessionObserver                                objc.Class
	}

	// AVFoundation (AVPlayerItem.h:27-37, AVMediaFormat.h:17-18).
	didPlayToEnd, failedToPlayToEnd, failedErrorKey, mediaTypeVideo objc.ID
	mediaTypeAudio                                                  objc.ID
	// AVFoundation (AVAudioProcessingSettings.h): the pitch kept at any
	// speed (docs/plan/04-media-engine.md §2.2).
	pitchTimeDomain objc.ID
	// AVFAudio (AVAudioSessionTypes.h:100-339): the reason of an
	// interruption (:320) and its older flag of a suspended app (:335,
	// deprecated from iOS 14.5 but still exported), and the notification
	// of the media services' reset (:251).
	category, mode, interruption, interruptionType, interruptionOption objc.ID
	interruptionReason, interruptionSuspended                          objc.ID
	routeChange, routeReason, mediaReset                               objc.ID
	// Foundation (NSError.h:24).
	underlyingErrorKey objc.ID
	// UIKit (UIScene.h:110): a scene leaves the background.
	willEnterForeground objc.ID
	// CoreVideo (CVPixelBuffer.h).
	keyFormat, keyIOSurface, keyWidth, keyHeight cf.Ref

	// Made once: the key paths observed and the asset keys loaded.
	keyStatus, keyControl, keyDuration, keyTracks objc.ID
	assetKeys                                     objc.ID // @[@"duration", @"tracks"]

	taskInfo dl.Fn // task_info
	taskSelf uint32

	sel selectors
}

// selectors are the messages the package sends. Each is in the SDK's
// headers: NSURL.h, AVAsset.h, AVAssetTrack.h, AVPlayerItemTrack.h,
// AVPlayerItem.h, AVPlayer.h, AVPlayerItemOutput.h,
// AVAsynchronousKeyValueLoading.h, AVAudioSession.h, NSArray.h, NSString.h,
// NSKeyValueObserving.h, NSNotification.h, NSDictionary.h, NSError.h.
type selectors struct {
	alloc, init                                       objc.SEL
	urlWithString, fileURLWithPath                    objc.SEL
	assetWithURL, cancelLoading, loadValues, statusOf objc.SEL
	duration, tracks, mediaType, assetTrack           objc.SEL
	setEnabled, isEnabled                             objc.SEL
	itemWithAsset, status, errorOf, loadedTimeRanges  objc.SEL
	addOutput, removeOutput, initWithAttributes       objc.SEL
	replaceItem, play, pause, timeControlStatus       objc.SEL
	setMuted                                          objc.SEL
	count, objectAtIndex, arrayWithObjects, isEqualTo objc.SEL
	addKVO, removeKVO                                 objc.SEL
	defaultCenter, addObserver, removeObserver, post  objc.SEL
	object, userInfo, objectForKey                    objc.SEL
	domain, code, localizedDescription                objc.SEL
	sharedInstance, setCategory, setActive            objc.SEL
	ended, failed, interrupted, routeChanged          objc.SEL
	mediaServicesReset, enteringForeground            objc.SEL
	// The pitch at any speed: AVPlayerItem.h, audioTimePitchAlgorithm.
	setPitchAlgorithm, pitchAlgorithm objc.SEL
	// The variants of an HLS stream (followSize): AVAsset.h:749,
	// AVAssetVariant.h:40.
	variants, videoAttributes objc.SEL
}

// assetKeyNames backs the array of asset keys while it is made: a package
// variable, off any goroutine's stack, as objc.Call wants its pointers.
var assetKeyNames [assetKeyCount]objc.ID

// blockType is the type a block's function takes first (purego's objc.Block,
// which gDesk's objc returns from NewBlock but does not name).
var blockType = reflect.TypeOf(objc.NewBlock).Out(0)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// initAVF loads the frameworks and resolves what the package uses, once;
// its failure is kept and returned every time.
var initAVF = sync.OnceValue(func() error {
	if err := objc.Init(); err != nil {
		return err
	}
	for _, fw := range []string{"Foundation", "AVFoundation", "AVFAudio", "CoreMedia", "CoreVideo", "QuartzCore", "UIKit"} {
		if err := objc.LoadFramework(fw); err != nil {
			return err
		}
	}
	if err := resolveConstants(); err != nil {
		return err
	}
	if err := resolveFunctions(); err != nil {
		return err
	}
	resolveSelectors()
	if err := resolveClasses(); err != nil {
		return err
	}
	makeStrings()

	return defineClasses()
})

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// resolveConstants reads the NSString and CFString constants, each from its
// framework.
func resolveConstants() error {
	for _, group := range []struct {
		path  string
		names []string
		dst   []*objc.ID
	}{
		{pathAVFoundation, []string{
			"AVPlayerItemDidPlayToEndTimeNotification", "AVPlayerItemFailedToPlayToEndTimeNotification",
			"AVPlayerItemFailedToPlayToEndTimeErrorKey", "AVMediaTypeVideo", "AVMediaTypeAudio",
			"AVAudioTimePitchAlgorithmTimeDomain",
		}, []*objc.ID{
			&av.didPlayToEnd, &av.failedToPlayToEnd, &av.failedErrorKey, &av.mediaTypeVideo, &av.mediaTypeAudio,
			&av.pitchTimeDomain,
		}},
		{pathAVFAudio, []string{
			"AVAudioSessionCategoryPlayback", "AVAudioSessionModeDefault", "AVAudioSessionInterruptionNotification",
			"AVAudioSessionInterruptionTypeKey", "AVAudioSessionInterruptionOptionKey",
			"AVAudioSessionInterruptionReasonKey", "AVAudioSessionInterruptionWasSuspendedKey",
			"AVAudioSessionRouteChangeNotification", "AVAudioSessionRouteChangeReasonKey",
			"AVAudioSessionMediaServicesWereResetNotification",
		}, []*objc.ID{
			&av.category, &av.mode, &av.interruption, &av.interruptionType, &av.interruptionOption,
			&av.interruptionReason, &av.interruptionSuspended,
			&av.routeChange, &av.routeReason, &av.mediaReset,
		}},
		{pathFoundation, []string{"NSUnderlyingErrorKey"}, []*objc.ID{&av.underlyingErrorKey}},
		{pathUIKit, []string{"UISceneWillEnterForegroundNotification"}, []*objc.ID{&av.willEnterForeground}},
	} {
		lib, err := dl.Open(group.path)
		if err != nil {
			return fmt.Errorf(frameworkMissing, group.path, err)
		}
		for i, name := range group.names {
			ref, err := cf.Constant(lib.Handle(), name)
			if err != nil || ref == 0 {
				return fmt.Errorf(constantMissing, name, core.ErrNotFound)
			}
			*group.dst[i] = objc.ID(ref)
		}
	}
	cv, err := dl.Open(pathCoreVideo)
	if err != nil {
		return fmt.Errorf(frameworkMissing, pathCoreVideo, err)
	}
	for _, k := range []struct {
		dst  *cf.Ref
		name string
	}{
		{&av.keyFormat, "kCVPixelBufferPixelFormatTypeKey"},
		{&av.keyIOSurface, "kCVPixelBufferIOSurfacePropertiesKey"},
		{&av.keyWidth, "kCVPixelBufferWidthKey"},
		{&av.keyHeight, "kCVPixelBufferHeightKey"},
	} {
		if *k.dst, err = cf.Constant(cv.Handle(), k.name); err != nil || *k.dst == 0 {
			return fmt.Errorf(constantMissing, k.name, core.ErrNotFound)
		}
	}

	return nil
}

// resolveFunctions binds task_info of libSystem and reads the task's
// port.
func resolveFunctions() error {
	sys, err := dl.Open(pathLibSystem)
	if err != nil {
		return fmt.Errorf(frameworkMissing, pathLibSystem, err)
	}
	if err := sys.Bind(dl.Sym{Fn: &av.taskInfo, Name: "task_info"}); err != nil {
		return err
	}
	// mach_task_self() is a macro reading the global mach_task_self_
	// (mach/mach_init.h): dlsym gives its address.
	var self dl.Fn
	if err := sys.Bind(dl.Sym{Fn: &self, Name: "mach_task_self_"}); err != nil {
		return err
	}
	av.taskSelf = *(*uint32)(unsafe.Pointer(dl.Bytes(uintptr(self)))) //nolint:gosec // G103: a global of libSystem, read through its address

	return nil
}

// resolveSelectors fills av.sel.
func resolveSelectors() {
	s := &av.sel
	s.alloc, s.init = objc.Sel("alloc"), objc.Sel("init")
	s.urlWithString, s.fileURLWithPath = objc.Sel("URLWithString:"), objc.Sel("fileURLWithPath:")
	s.assetWithURL, s.cancelLoading = objc.Sel("URLAssetWithURL:options:"), objc.Sel("cancelLoading")
	s.loadValues = objc.Sel("loadValuesAsynchronouslyForKeys:completionHandler:")
	s.statusOf = objc.Sel("statusOfValueForKey:error:")
	s.duration, s.tracks, s.mediaType, s.assetTrack = objc.Sel("duration"), objc.Sel("tracks"), objc.Sel("mediaType"), objc.Sel("assetTrack")
	s.setEnabled, s.isEnabled = objc.Sel("setEnabled:"), objc.Sel("isEnabled")
	s.itemWithAsset = objc.Sel("playerItemWithAsset:automaticallyLoadedAssetKeys:")
	s.status, s.errorOf, s.loadedTimeRanges = objc.Sel("status"), objc.Sel("error"), objc.Sel("loadedTimeRanges")
	s.addOutput, s.removeOutput = objc.Sel("addOutput:"), objc.Sel("removeOutput:")
	s.initWithAttributes = objc.Sel("initWithPixelBufferAttributes:")
	s.replaceItem = objc.Sel("replaceCurrentItemWithPlayerItem:")
	s.play, s.pause, s.timeControlStatus = objc.Sel("play"), objc.Sel("pause"), objc.Sel("timeControlStatus")
	s.setMuted = objc.Sel("setMuted:")
	s.count, s.objectAtIndex = objc.Sel("count"), objc.Sel("objectAtIndex:")
	s.arrayWithObjects, s.isEqualTo = objc.Sel("arrayWithObjects:count:"), objc.Sel("isEqualToString:")
	s.addKVO = objc.Sel("addObserver:forKeyPath:options:context:")
	s.removeKVO = objc.Sel("removeObserver:forKeyPath:")
	s.defaultCenter = objc.Sel("defaultCenter")
	s.addObserver, s.removeObserver = objc.Sel("addObserver:selector:name:object:"), objc.Sel("removeObserver:")
	s.post = objc.Sel("postNotificationName:object:userInfo:")
	s.object, s.userInfo, s.objectForKey = objc.Sel("object"), objc.Sel("userInfo"), objc.Sel("objectForKey:")
	s.domain, s.code, s.localizedDescription = objc.Sel("domain"), objc.Sel("code"), objc.Sel("localizedDescription")
	s.sharedInstance = objc.Sel("sharedInstance")
	s.setCategory, s.setActive = objc.Sel("setCategory:mode:options:error:"), objc.Sel("setActive:error:")
	s.ended, s.failed = objc.Sel(selEnded), objc.Sel(selFailed)
	s.interrupted, s.routeChanged = objc.Sel(selInterruption), objc.Sel(selRouteChange)
	s.variants, s.videoAttributes = objc.Sel("variants"), objc.Sel("videoAttributes")
	s.mediaServicesReset, s.enteringForeground = objc.Sel(selMediaReset), objc.Sel(selForeground)
	s.setPitchAlgorithm, s.pitchAlgorithm = objc.Sel("setAudioTimePitchAlgorithm:"), objc.Sel("audioTimePitchAlgorithm")
}

// resolveClasses looks the system's classes up.
func resolveClasses() error {
	for _, c := range []struct {
		dst  *objc.ID
		name string
	}{
		{&av.cls.player, "AVPlayer"},
		{&av.cls.item, "AVPlayerItem"},
		{&av.cls.asset, "AVURLAsset"},
		{&av.cls.url, "NSURL"},
		{&av.cls.output, "AVPlayerItemVideoOutput"},
		{&av.cls.center, "NSNotificationCenter"},
		{&av.cls.session, "AVAudioSession"},
		{&av.cls.array, "NSArray"},
	} {
		if *c.dst = objc.ID(objc.GetClass(c.name)); *c.dst == 0 {
			return fmt.Errorf(classMissing, c.name, core.ErrNotFound)
		}
	}

	return nil
}

// makeStrings makes the key paths and the array of asset keys, owned for the
// life of the process.
func makeStrings() {
	av.keyStatus, av.keyControl = objc.NSString("status"), objc.NSString("timeControlStatus")
	av.keyDuration, av.keyTracks = objc.NSString("duration"), objc.NSString("tracks")
	assetKeyNames = [assetKeyCount]objc.ID{av.keyDuration, av.keyTracks}
	objc.WithPool(func() {
		arr := objc.Call(av.cls.array, av.sel.arrayWithObjects, uintptr(unsafe.Pointer(&assetKeyNames[0])), assetKeyCount) //nolint:gosec // G103: a package variable, read during the call
		av.assetKeys = objc.Retain(arr)
	})
}

// defineClasses defines the two observer classes of the package: the
// engine's, for key-value observing of the item and the player, the
// notifications of the item's end and of the scene leaving the
// background; the session's, for the audio session's notifications.
func defineClasses() error {
	var err error
	av.cls.observer, err = objc.DefineClass(observerClassName, "NSObject", nil,
		objc.Method{Selector: "observeValueForKeyPath:ofObject:change:context:", Fn: observed},
		objc.Method{Selector: selEnded, Fn: func(self objc.ID, _ objc.SEL, note objc.ID) { notified(self, note, true) }},
		objc.Method{Selector: selFailed, Fn: func(self objc.ID, _ objc.SEL, note objc.ID) { notified(self, note, false) }},
		objc.Method{Selector: selForeground, Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { enteringForeground(self) }},
	)
	if err != nil {
		return err
	}
	av.cls.sessionObserver, err = objc.DefineClass(sessionClassName, "NSObject", nil,
		objc.Method{Selector: selInterruption, Fn: func(self objc.ID, _ objc.SEL, note objc.ID) { sessionNotified(self, note, true) }},
		objc.Method{Selector: selRouteChange, Fn: func(self objc.ID, _ objc.SEL, note objc.ID) { sessionNotified(self, note, false) }},
		objc.Method{Selector: selMediaReset, Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { sessionReset(self) }},
	)

	return err
}

// newBlock makes an owned Objective-C block of a function taking args after
// the block itself and answering nothing; fn gets those arguments. Its C
// callback is made once per list of argument types (purego caches it by the
// function's type, which reflect.FuncOf makes the same each time), so blocks
// made for each player cost no new callback. Release it (objc.Release) once
// the API it is given has its copy.
func newBlock(args []reflect.Type, fn func(args []reflect.Value)) objc.ID {
	in := append([]reflect.Type{blockType}, args...)
	typ := reflect.FuncOf(in, nil, false)
	impl := reflect.MakeFunc(typ, func(vals []reflect.Value) []reflect.Value {
		fn(vals[1:])

		return nil
	})

	return objc.ID(objc.NewBlock(impl.Interface()))
}

// center returns [NSNotificationCenter defaultCenter] (not owned).
func center() objc.ID { return objc.Call(av.cls.center, av.sel.defaultCenter) }
