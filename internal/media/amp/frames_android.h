// Adapted from jplayer-mobile/internal/media/amp/frames_android.h at 49de3c2.

// The frame path of the Android engine: MediaPlayer -> AImageReader
// (YUV_420_888, CPU_READ_OFTEN) -> AImage -> RGBA in Go memory.
// Declarations only: frames_android.go uses //export, so its preamble may
// hold no definitions. NDK only, no JNI call but the bridge of
// amp_reader_surface (ANativeWindow_toSurface with the JNIEnv* of
// jni.Env.Native).

#ifndef JPLAYER_AMP_FRAMES_H
#define JPLAYER_AMP_FRAMES_H

#include <jni.h>
#include <media/NdkImage.h>
#include <media/NdkImageReader.h>
#include <stdint.h>

// amp_frame describes the AImage the reader's thread holds while Go
// converts it. It lives in C memory, inside amp_reader.
typedef struct amp_frame {
	int32_t format, width, height;
	int32_t crop[4]; // left, top, right, bottom
	int32_t planes;
	int32_t pix_stride[3], row_stride[3], data_len[3];
	uint8_t* data[3];
	int32_t data_space; // AImage_getDataSpace (API 34), -1 when unknown
	AImage* img;
} amp_frame;

// amp_reader is one AImageReader with its window and the frame its
// callback hands to Go.
typedef struct amp_reader amp_reader;

// amp_reader_new makes a YUV_420_888 reader of w x h for the CPU to read,
// whose frames go to Go's ampOnFrame with ctx. NULL with *status set on
// failure.
amp_reader* amp_reader_new(int32_t w, int32_t h, uintptr_t ctx, int32_t* status);

// amp_reader_surface returns, as a local reference of env (a jobject as
// an integer), the android.view.Surface of the reader's window
// (ANativeWindow_toSurface), or 0, with no exception left pending. env is
// jni.Env.Native of the calling thread.
uintptr_t amp_reader_surface(uintptr_t env, amp_reader* rd);

// amp_reader_close stops the callback: it waits for one in flight, and no
// frame reaches Go after it returns.
void amp_reader_close(amp_reader* rd);

// amp_reader_sync waits for a callback in flight, if any: one that starts
// after it returns sees what the caller stored before.
void amp_reader_sync(amp_reader* rd);

// amp_reader_delete removes the listener, deletes the reader and frees rd.
// Call it after amp_reader_close, the player released, and the Surface
// released.
void amp_reader_delete(amp_reader* rd);

// amp_reader_stats writes the callbacks, the failed acquires and the last
// failure's media_status_t into out[0..2].
void amp_reader_stats(amp_reader* rd, int64_t* out);

// amp_convert writes f's pixels, cropped and scaled to dw x dh, as opaque
// RGBA into dst (rows stride bytes apart), limited range, BT.709 when
// bt709 else BT.601. It returns the time it took in ns, or -1 for an
// image it cannot read.
int64_t amp_convert(amp_frame* f, uint8_t* dst, int32_t stride, int32_t dw, int32_t dh, int32_t bt709);

// amp_convert_planes is amp_convert of a 4:2:0 picture given by its
// planes: w x h, y rows ys apart, u and v rows uvs apart with ps bytes
// between samples. It is for the device tests and the benchmark.
int64_t amp_convert_planes(const uint8_t* y, int32_t ys, const uint8_t* u, const uint8_t* v, int32_t uvs,
	int32_t ps, int32_t w, int32_t h, uint8_t* dst, int32_t stride, int32_t dw, int32_t dh, int32_t bt709);

// amp_now is CLOCK_MONOTONIC in ns.
int64_t amp_now(void);

#endif
