// Adapted from jplayer-mobile/internal/media/amp/ids_android.go at 49de3c2.

//go:build android && cgo

package amp

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The constants of android.jar (platform 35), read with javap -constants;
// each exists from API 26, the app's oldest.
const (
	usageMedia       = 1   // AudioAttributes.USAGE_MEDIA
	contentMusic     = 2   // AudioAttributes.CONTENT_TYPE_MUSIC
	contentMovie     = 3   // AudioAttributes.CONTENT_TYPE_MOVIE
	partialWakeLock  = 1   // PowerManager.PARTIAL_WAKE_LOCK
	seekClosest      = 3   // MediaPlayer.SEEK_CLOSEST
	trackAudio       = 2   // MediaPlayer.TrackInfo.MEDIA_TRACK_TYPE_AUDIO
	infoBufferStart  = 701 // MediaPlayer.MEDIA_INFO_BUFFERING_START
	infoBufferEnd    = 702 // MediaPlayer.MEDIA_INFO_BUFFERING_END
	errInvalidOp     = -38 // what of onError for a call in a wrong state (INVALID_OPERATION)
	focusGranted     = 1   // AudioManager.AUDIOFOCUS_REQUEST_GRANTED (the changes: enginekit.Focus*)
	keyDuration      = 9   // MediaMetadataRetriever.METADATA_KEY_DURATION
	keyHasVideo      = 17  // METADATA_KEY_HAS_VIDEO
	keyVideoWidth    = 18  // METADATA_KEY_VIDEO_WIDTH
	keyVideoHeight   = 19  // METADATA_KEY_VIDEO_HEIGHT
	keyVideoRotation = 24  // METADATA_KEY_VIDEO_ROTATION
	optionClosest    = 3   // MediaMetadataRetriever.OPTION_CLOSEST
	jpegQuality      = 72  // toDataURL's 0.72, sources.js:84
	scaledFrameAPI   = 27  // MediaMetadataRetriever.getScaledFrameAtTime
	wifiHighPerf     = 3   // WifiManager.WIFI_MODE_FULL_HIGH_PERF (deprecated from API 34, still taken)
)

// The names of MediaPlayer's listeners the proxy of a load implements,
// and of their methods, in the order the proxy's function gets them.
var (
	listenerInterfaces = []string{
		"android/media/MediaPlayer$OnPreparedListener",
		"android/media/MediaPlayer$OnCompletionListener",
		"android/media/MediaPlayer$OnErrorListener",
		"android/media/MediaPlayer$OnInfoListener",
		"android/media/MediaPlayer$OnBufferingUpdateListener",
		"android/media/MediaPlayer$OnSeekCompleteListener",
		"android/media/MediaPlayer$OnVideoSizeChangedListener",
	}
	listenerMethods = []string{
		"onPrepared", "onCompletion", "onError", "onInfo", "onBufferingUpdate", "onSeekComplete", "onVideoSizeChanged",
	}
)

// The indexes of listenerMethods.
const (
	onPrepared = iota
	onCompletion
	onError
	onInfo
	onBufferingUpdate
	onSeekComplete
	onVideoSizeChanged
)

// ids are the classes (global references, kept for the life of the
// process) and methods the engine, the session and the prober call,
// looked up once. Every signature was checked with javap -s on
// platforms/android-35/android.jar.
var ids struct {
	mu sync.Mutex
	ok bool

	player                                                     jni.Class
	playerNew, setAudioAttributes, setWakeMode                 jni.MethodID
	setDataSource, prepareAsync, start, pause, release         jni.MethodID
	setDataSourceURI                                           jni.MethodID // (Context, Uri): a content:// document
	uri                                                        jni.Class
	uriParse                                                   jni.MethodID
	seekTo, getCurrentPosition, getDuration, setVolume         jni.MethodID
	setPlaybackParams, setSurface, getTrackInfo                jni.MethodID
	trackType                                                  jni.MethodID // MediaPlayer.TrackInfo.getTrackType
	listenerSetters                                            [len(listenerSetterNames)]jni.MethodID
	params                                                     jni.Class
	paramsNew, setSpeed                                        jni.MethodID
	attrsBuilder                                               jni.Class
	attrsBuilderNew, setUsage, setContentType, attrsBuild      jni.MethodID
	surfaceRelease                                             jni.MethodID
	policy                                                     jni.Class
	policyInstance, cleartextPermitted                         jni.MethodID
	context                                                    jni.Class
	getSystemService                                           jni.MethodID
	audioManager                                               jni.Class
	requestFocus, abandonFocus                                 jni.MethodID
	focusBuilder                                               jni.Class
	focusBuilderNew, focusAttrs, focusListener, focusBuild     jni.MethodID
	mmr                                                        jni.Class
	mmrNew, mmrSetPath, mmrSetURL, mmrSetURI, mmrExtract       jni.MethodID
	mmrFrame, mmrScaledFrame, mmrRelease                       jni.MethodID
	bitmap                                                     jni.Class
	createScaled, compress, bitmapWidth, bitmapHeight, recycle jni.MethodID
	jpeg                                                       jni.Object // Bitmap.CompressFormat.JPEG, global
	stream                                                     jni.Class
	streamNew, toByteArray                                     jni.MethodID
	hashMap                                                    jni.Class
	hashMapNew                                                 jni.MethodID
	debug                                                      jni.Class
	nativeHeap                                                 jni.MethodID
	wifiManager                                                jni.Class
	createWifiLock                                             jni.MethodID
	wifiLock                                                   jni.Class
	wifiCounted, wifiAcquire, wifiRelease, wifiHeld            jni.MethodID
	powerManager                                               jni.Class
	newWakeLock                                                jni.MethodID
	wakeLock                                                   jni.Class
	wakeCounted, wakeAcquireFor, wakeRelease, wakeHeld         jni.MethodID
	sdk                                                        int32
}

// listenerSetterNames are MediaPlayer's setters of the listeners, in the
// order of listenerInterfaces.
var listenerSetterNames = [...]string{
	"setOnPreparedListener", "setOnCompletionListener", "setOnErrorListener", "setOnInfoListener",
	"setOnBufferingUpdateListener", "setOnSeekCompleteListener", "setOnVideoSizeChangedListener",
}

// liveGlobals counts the global references the engine made and did not
// delete, the classes of ids aside (Stats).
var liveGlobals atomic.Int64

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// lookup finds ids, once per process, on any thread attached to the VM.
func lookup(env jni.Env) error {
	ids.mu.Lock()
	defer ids.mu.Unlock()
	if ids.ok {
		return nil
	}
	l := looker{env: env}
	l.class(&ids.player, "android/media/MediaPlayer")
	l.method(&ids.playerNew, ids.player, "<init>", "()V")
	l.method(&ids.setAudioAttributes, ids.player, "setAudioAttributes", "(Landroid/media/AudioAttributes;)V")
	l.method(&ids.setWakeMode, ids.player, "setWakeMode", "(Landroid/content/Context;I)V")
	l.method(&ids.setDataSource, ids.player, "setDataSource", "(Ljava/lang/String;)V")
	l.method(&ids.setDataSourceURI, ids.player, "setDataSource", "(Landroid/content/Context;Landroid/net/Uri;)V")
	l.class(&ids.uri, "android/net/Uri")
	l.static(&ids.uriParse, ids.uri, "parse", "(Ljava/lang/String;)Landroid/net/Uri;")
	l.method(&ids.prepareAsync, ids.player, "prepareAsync", "()V")
	l.method(&ids.start, ids.player, "start", "()V")
	l.method(&ids.pause, ids.player, "pause", "()V")
	l.method(&ids.release, ids.player, "release", "()V")
	l.method(&ids.seekTo, ids.player, "seekTo", "(JI)V")
	l.method(&ids.getCurrentPosition, ids.player, "getCurrentPosition", "()I")
	l.method(&ids.getDuration, ids.player, "getDuration", "()I")
	l.method(&ids.setVolume, ids.player, "setVolume", "(FF)V")
	l.method(&ids.setPlaybackParams, ids.player, "setPlaybackParams", "(Landroid/media/PlaybackParams;)V")
	l.method(&ids.setSurface, ids.player, "setSurface", "(Landroid/view/Surface;)V")
	l.method(&ids.getTrackInfo, ids.player, "getTrackInfo", "()[Landroid/media/MediaPlayer$TrackInfo;")
	var trackInfo jni.Class
	l.class(&trackInfo, "android/media/MediaPlayer$TrackInfo")
	l.method(&ids.trackType, trackInfo, "getTrackType", "()I")
	for i, name := range listenerSetterNames {
		l.method(&ids.listenerSetters[i], ids.player, name, "(L"+listenerInterfaces[i]+";)V")
	}
	l.class(&ids.params, "android/media/PlaybackParams")
	l.method(&ids.paramsNew, ids.params, "<init>", "()V")
	l.method(&ids.setSpeed, ids.params, "setSpeed", "(F)Landroid/media/PlaybackParams;")
	l.class(&ids.attrsBuilder, "android/media/AudioAttributes$Builder")
	l.method(&ids.attrsBuilderNew, ids.attrsBuilder, "<init>", "()V")
	l.method(&ids.setUsage, ids.attrsBuilder, "setUsage", "(I)Landroid/media/AudioAttributes$Builder;")
	l.method(&ids.setContentType, ids.attrsBuilder, "setContentType", "(I)Landroid/media/AudioAttributes$Builder;")
	l.method(&ids.attrsBuild, ids.attrsBuilder, "build", "()Landroid/media/AudioAttributes;")
	var surface jni.Class
	l.class(&surface, "android/view/Surface")
	l.method(&ids.surfaceRelease, surface, "release", "()V")
	l.class(&ids.policy, "android/security/NetworkSecurityPolicy")
	l.static(&ids.policyInstance, ids.policy, "getInstance", "()Landroid/security/NetworkSecurityPolicy;")
	l.method(&ids.cleartextPermitted, ids.policy, "isCleartextTrafficPermitted", "(Ljava/lang/String;)Z")
	l.class(&ids.context, "android/content/Context")
	l.method(&ids.getSystemService, ids.context, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;")
	l.class(&ids.audioManager, "android/media/AudioManager")
	l.method(&ids.requestFocus, ids.audioManager, "requestAudioFocus", "(Landroid/media/AudioFocusRequest;)I")
	l.method(&ids.abandonFocus, ids.audioManager, "abandonAudioFocusRequest", "(Landroid/media/AudioFocusRequest;)I")
	l.class(&ids.focusBuilder, "android/media/AudioFocusRequest$Builder")
	l.method(&ids.focusBuilderNew, ids.focusBuilder, "<init>", "(I)V")
	l.method(&ids.focusAttrs, ids.focusBuilder, "setAudioAttributes", "(Landroid/media/AudioAttributes;)Landroid/media/AudioFocusRequest$Builder;")
	l.method(&ids.focusListener, ids.focusBuilder, "setOnAudioFocusChangeListener", "(Landroid/media/AudioManager$OnAudioFocusChangeListener;)Landroid/media/AudioFocusRequest$Builder;")
	l.method(&ids.focusBuild, ids.focusBuilder, "build", "()Landroid/media/AudioFocusRequest;")
	l.class(&ids.mmr, "android/media/MediaMetadataRetriever")
	l.method(&ids.mmrNew, ids.mmr, "<init>", "()V")
	l.method(&ids.mmrSetPath, ids.mmr, "setDataSource", "(Ljava/lang/String;)V")
	l.method(&ids.mmrSetURL, ids.mmr, "setDataSource", "(Ljava/lang/String;Ljava/util/Map;)V")
	l.method(&ids.mmrSetURI, ids.mmr, "setDataSource", "(Landroid/content/Context;Landroid/net/Uri;)V")
	l.method(&ids.mmrExtract, ids.mmr, "extractMetadata", "(I)Ljava/lang/String;")
	l.method(&ids.mmrFrame, ids.mmr, "getFrameAtTime", "(JI)Landroid/graphics/Bitmap;")
	l.method(&ids.mmrRelease, ids.mmr, "release", "()V")
	l.class(&ids.bitmap, "android/graphics/Bitmap")
	l.static(&ids.createScaled, ids.bitmap, "createScaledBitmap", "(Landroid/graphics/Bitmap;IIZ)Landroid/graphics/Bitmap;")
	l.method(&ids.compress, ids.bitmap, "compress", "(Landroid/graphics/Bitmap$CompressFormat;ILjava/io/OutputStream;)Z")
	l.method(&ids.bitmapWidth, ids.bitmap, "getWidth", "()I")
	l.method(&ids.bitmapHeight, ids.bitmap, "getHeight", "()I")
	l.method(&ids.recycle, ids.bitmap, "recycle", "()V")
	var format jni.Class
	l.class(&format, "android/graphics/Bitmap$CompressFormat")
	if l.err == nil {
		var f jni.FieldID
		f, l.err = env.StaticField(format, "JPEG", "Landroid/graphics/Bitmap$CompressFormat;")
		if l.err == nil {
			ids.jpeg = env.NewGlobalRef(env.StaticObjectField(format, f))
		}
	}
	l.class(&ids.stream, "java/io/ByteArrayOutputStream")
	l.method(&ids.streamNew, ids.stream, "<init>", "()V")
	l.method(&ids.toByteArray, ids.stream, "toByteArray", "()[B")
	l.class(&ids.hashMap, "java/util/HashMap")
	l.method(&ids.hashMapNew, ids.hashMap, "<init>", "()V")
	l.class(&ids.debug, "android/os/Debug")
	l.static(&ids.nativeHeap, ids.debug, "getNativeHeapAllocatedSize", "()J")
	l.class(&ids.wifiManager, "android/net/wifi/WifiManager")
	l.method(&ids.createWifiLock, ids.wifiManager, "createWifiLock", "(ILjava/lang/String;)Landroid/net/wifi/WifiManager$WifiLock;")
	l.class(&ids.wifiLock, "android/net/wifi/WifiManager$WifiLock")
	l.method(&ids.wifiCounted, ids.wifiLock, "setReferenceCounted", "(Z)V")
	l.method(&ids.wifiAcquire, ids.wifiLock, "acquire", "()V")
	l.method(&ids.wifiRelease, ids.wifiLock, "release", "()V")
	l.method(&ids.wifiHeld, ids.wifiLock, "isHeld", "()Z")
	l.class(&ids.powerManager, "android/os/PowerManager")
	l.method(&ids.newWakeLock, ids.powerManager, "newWakeLock", "(ILjava/lang/String;)Landroid/os/PowerManager$WakeLock;")
	l.class(&ids.wakeLock, "android/os/PowerManager$WakeLock")
	l.method(&ids.wakeCounted, ids.wakeLock, "setReferenceCounted", "(Z)V")
	l.method(&ids.wakeAcquireFor, ids.wakeLock, "acquire", "(J)V")
	l.method(&ids.wakeRelease, ids.wakeLock, "release", "()V")
	l.method(&ids.wakeHeld, ids.wakeLock, "isHeld", "()Z")
	var version jni.Class
	l.class(&version, "android/os/Build$VERSION")
	if l.err == nil {
		var f jni.FieldID
		f, l.err = env.StaticField(version, "SDK_INT", "I")
		if l.err == nil {
			ids.sdk = env.StaticIntField(version, f)
		}
	}
	if l.err == nil && ids.sdk >= scaledFrameAPI {
		l.method(&ids.mmrScaledFrame, ids.mmr, "getScaledFrameAtTime", "(JIII)Landroid/graphics/Bitmap;")
	}
	if l.err != nil {
		return l.err
	}
	ids.ok = true

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// looker looks classes and methods up until the first failure.
type looker struct {
	env jni.Env
	err error
}

// class finds a class and keeps it as a global reference.
func (l *looker) class(dst *jni.Class, name string) {
	if l.err != nil {
		return
	}
	c, err := l.env.FindClass(name)
	if err != nil {
		l.err = err

		return
	}
	*dst = jni.Class(l.env.NewGlobalRef(jni.Object(c)))
	l.env.DeleteLocalRef(jni.Object(c))
}

// method finds a method of c.
func (l *looker) method(dst *jni.MethodID, c jni.Class, name, sig string) {
	if l.err != nil {
		return
	}
	*dst, l.err = l.env.Method(c, name, sig)
}

// static finds a static method of c.
func (l *looker) static(dst *jni.MethodID, c jni.Class, name, sig string) {
	if l.err != nil {
		return
	}
	*dst, l.err = l.env.StaticMethod(c, name, sig)
}

// newGlobal makes a global reference of o, counted in liveGlobals; 0 for 0.
func newGlobal(env jni.Env, o jni.Object) jni.Object {
	if o == 0 {
		return 0
	}
	g := env.NewGlobalRef(o)
	if g != 0 {
		liveGlobals.Add(1)
	}

	return g
}

// deleteGlobal deletes a global reference newGlobal made.
func deleteGlobal(env jni.Env, o jni.Object) {
	if o == 0 {
		return
	}
	env.DeleteGlobalRef(o)
	liveGlobals.Add(-1)
}

// logf logs what has nobody to return to.
func logf(format string, args ...any) { log.Printf(format, args...) }
