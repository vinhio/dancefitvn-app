// Adapted from jplayer-mobile/internal/media/amp/frames_android.c at 49de3c2.

//go:build android && cgo

// The frame path of the Android engine. Every NDK call was checked against
// the NDK 30 headers (media/NdkImageReader.h, media/NdkImage.h,
// android/native_window_jni.h, android/hardware_buffer.h,
// android/data_space.h). The conversion is that of the M4.2a spike
// (cmd/ampspike/video_android.c: yuv_row_neon, yuv_convert), with a
// scaled path for frames over 1280 pixels.

#include "frames_android.h"

#include <android/hardware_buffer.h>
#include <android/native_window_jni.h>
#if defined(__aarch64__)
#include <arm_neon.h>
#endif
#include <dlfcn.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "_cgo_export.h"

// maxImages is how many images the reader lets out at once: the callback
// holds one at a time, the decoder queues the others.
#define MAX_IMAGES 3

// AMP_FORMAT_YV12 is android.graphics.ImageFormat.YV12, which
// media/NdkImage.h does not name: what MediaTek decoders give a
// YUV_420_888 reader (enginekit.ImageYV12). AImage hands its planes
// out as Y, U, V, as it does those of YUV_420_888.
#define AMP_FORMAT_YV12 0x32315659

struct amp_reader {
	AImageReader* r;
	ANativeWindow* win; // the reader's: not ours to release
	pthread_mutex_t mu;
	int closed;
	uintptr_t ctx;
	int64_t callbacks, fails, last_fail;
	amp_frame f;
};

int64_t amp_now(void) {
	struct timespec t;
	clock_gettime(CLOCK_MONOTONIC, &t);
	return (int64_t)t.tv_sec * 1000000000LL + t.tv_nsec;
}

// get_data_space is AImage_getDataSpace when the system has it (API 34);
// the app builds for API 26, so it is looked up at run time, once.
typedef media_status_t (*data_space_fn)(const AImage*, int32_t*);
static data_space_fn get_data_space;
static pthread_once_t data_space_once = PTHREAD_ONCE_INIT;
static void find_data_space(void) {
	void* h = dlopen("libmediandk.so", RTLD_NOW);
	if (h != NULL) {
		get_data_space = (data_space_fn)dlsym(h, "AImage_getDataSpace");
	}
}

// on_image is the one listener of every reader of the process. It runs on
// the reader's own thread for every frame the decoder queues: it acquires
// the image, describes it in rd->f, lets Go convert it, and gives it back.
// The mutex lets amp_reader_close wait for it.
static void on_image(void* context, AImageReader* r) {
	amp_reader* rd = (amp_reader*)context;
	pthread_mutex_lock(&rd->mu);
	AImage* img = NULL;
	media_status_t st = AImageReader_acquireNextImage(r, &img);
	if (rd->closed) {
		if (st == AMEDIA_OK && img != NULL) AImage_delete(img);
		pthread_mutex_unlock(&rd->mu);
		return;
	}
	rd->callbacks++;
	if (st != AMEDIA_OK || img == NULL) {
		rd->fails++;
		rd->last_fail = st;
		pthread_mutex_unlock(&rd->mu);
		return;
	}
	amp_frame* f = &rd->f;
	memset(f, 0, sizeof *f);
	f->img = img;
	f->data_space = -1;
	AImage_getFormat(img, &f->format);
	AImage_getWidth(img, &f->width);
	AImage_getHeight(img, &f->height);
	AImageCropRect c;
	if (AImage_getCropRect(img, &c) == AMEDIA_OK && c.right > c.left && c.bottom > c.top) {
		f->crop[0] = c.left;
		f->crop[1] = c.top;
		f->crop[2] = c.right;
		f->crop[3] = c.bottom;
	} else {
		f->crop[2] = f->width;
		f->crop[3] = f->height;
	}
	AImage_getNumberOfPlanes(img, &f->planes);
	for (int i = 0; i < f->planes && i < 3; i++) {
		if (AImage_getPlanePixelStride(img, i, &f->pix_stride[i]) != AMEDIA_OK) f->pix_stride[i] = 0;
		if (AImage_getPlaneRowStride(img, i, &f->row_stride[i]) != AMEDIA_OK) f->row_stride[i] = 0;
		int len = 0;
		if (AImage_getPlaneData(img, i, &f->data[i], &len) != AMEDIA_OK) f->data[i] = NULL;
		f->data_len[i] = len;
	}
	pthread_once(&data_space_once, find_data_space);
	if (get_data_space != NULL) get_data_space(img, &f->data_space);
	ampOnFrame(rd->ctx, f);
	AImage_delete(img);
	f->img = NULL;
	pthread_mutex_unlock(&rd->mu);
}

amp_reader* amp_reader_new(int32_t w, int32_t h, uintptr_t ctx, int32_t* status) {
	amp_reader* rd = calloc(1, sizeof *rd);
	if (rd == NULL) {
		*status = AMEDIA_ERROR_UNKNOWN;
		return NULL;
	}
	pthread_mutex_init(&rd->mu, NULL);
	rd->ctx = ctx;
	media_status_t st = AImageReader_newWithUsage(w, h, AIMAGE_FORMAT_YUV_420_888,
		AHARDWAREBUFFER_USAGE_CPU_READ_OFTEN, MAX_IMAGES, &rd->r);
	if (st != AMEDIA_OK) {
		*status = st;
		pthread_mutex_destroy(&rd->mu);
		free(rd);
		return NULL;
	}
	AImageReader_ImageListener l = {.context = rd, .onImageAvailable = on_image};
	st = AImageReader_setImageListener(rd->r, &l);
	if (st == AMEDIA_OK) st = AImageReader_getWindow(rd->r, &rd->win);
	if (st != AMEDIA_OK) {
		*status = st;
		AImageReader_delete(rd->r);
		pthread_mutex_destroy(&rd->mu);
		free(rd);
		return NULL;
	}
	*status = AMEDIA_OK;
	return rd;
}

uintptr_t amp_reader_surface(uintptr_t envp, amp_reader* rd) {
	JNIEnv* env = (JNIEnv*)envp;
	if (env == NULL || rd == NULL || rd->win == NULL) return 0;
	jobject s = ANativeWindow_toSurface(env, rd->win);
	// jni.Env.Native's contract: leave no exception pending.
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
		return 0;
	}
	return (uintptr_t)s;
}

void amp_reader_close(amp_reader* rd) {
	pthread_mutex_lock(&rd->mu);
	rd->closed = 1;
	pthread_mutex_unlock(&rd->mu);
}

void amp_reader_sync(amp_reader* rd) {
	pthread_mutex_lock(&rd->mu);
	pthread_mutex_unlock(&rd->mu);
}

void amp_reader_delete(amp_reader* rd) {
	amp_reader_close(rd);
	AImageReader_setImageListener(rd->r, NULL);
	AImageReader_delete(rd->r);
	pthread_mutex_destroy(&rd->mu);
	free(rd);
}

void amp_reader_stats(amp_reader* rd, int64_t* out) {
	pthread_mutex_lock(&rd->mu);
	out[0] = rd->callbacks;
	out[1] = rd->fails;
	out[2] = rd->last_fail;
	pthread_mutex_unlock(&rd->mu);
}

// ===================================================================
// Conversion
// ===================================================================

// The limited-range matrices in Q16 (scalar, as enginekit.YUV420ToRGBA)
// and Q6 (NEON): 1.164 for Y, then V->R, U->G, V->G, U->B.
typedef struct {
	int32_t y, vr, ug, vg, ub;
} coefs;
static const coefs q16_601 = {76309, 104597, 25675, 53279, 132201};
static const coefs q16_709 = {76309, 117489, 13975, 34925, 138438};
static const coefs q6_601 = {75, 102, 25, 52, 129};
static const coefs q6_709 = {75, 115, 14, 34, 135};

static inline uint8_t clamp8(int32_t v) { return v < 0 ? 0 : v > 255 ? 255 : (uint8_t)v; }

static inline void yuv_px(int32_t y, int32_t u, int32_t v, uint8_t* p, const coefs* k) {
	int32_t yy = (y - 16) * k->y;
	u -= 128;
	v -= 128;
	p[0] = clamp8((yy + k->vr * v + 32768) >> 16);
	p[1] = clamp8((yy - k->ug * u - k->vg * v + 32768) >> 16);
	p[2] = clamp8((yy + k->ub * u + 32768) >> 16);
	p[3] = 255;
}

// yuv_row_scalar converts one row of w pixels starting at column x0.
static void yuv_row_scalar(const uint8_t* y, const uint8_t* u, const uint8_t* v, int32_t ps, int32_t x0,
	int32_t w, uint8_t* d, const coefs* k) {
	for (int32_t x = x0; x < w; x++) {
		yuv_px(y[x], u[(x >> 1) * ps], v[(x >> 1) * ps], d + 4 * x, k);
	}
}

#if defined(__aarch64__)
// yuv_half_neon converts 8 pixels: y8 luma, u8/v8 chroma already widened
// to one per pixel.
static inline uint8x8x3_t yuv_half_neon(uint8x8_t y8, uint8x8_t u8, uint8x8_t v8, const coefs* k) {
	int16x8_t yy = vmulq_n_s16(vsubq_s16(vreinterpretq_s16_u16(vmovl_u8(y8)), vdupq_n_s16(16)), (int16_t)k->y);
	int16x8_t uu = vsubq_s16(vreinterpretq_s16_u16(vmovl_u8(u8)), vdupq_n_s16(128));
	int16x8_t vv = vsubq_s16(vreinterpretq_s16_u16(vmovl_u8(v8)), vdupq_n_s16(128));
	int16x8_t r = vqaddq_s16(yy, vmulq_n_s16(vv, (int16_t)k->vr));
	int16x8_t g = vqsubq_s16(vqsubq_s16(yy, vmulq_n_s16(uu, (int16_t)k->ug)), vmulq_n_s16(vv, (int16_t)k->vg));
	int16x8_t b = vqaddq_s16(yy, vmulq_n_s16(uu, (int16_t)k->ub));
	uint8x8x3_t o;
	o.val[0] = vqrshrun_n_s16(r, 6);
	o.val[1] = vqrshrun_n_s16(g, 6);
	o.val[2] = vqrshrun_n_s16(b, 6);
	return o;
}

// yuv_row_neon converts one row, 16 pixels a step, and returns the column
// it stopped at. layout: 0 planar (pixel stride 1), 1 NV12 (u points at
// interleaved U,V), 2 NV21 (v points at interleaved V,U).
static int32_t yuv_row_neon(const uint8_t* y, const uint8_t* u, const uint8_t* v, int32_t layout, int32_t w,
	uint8_t* d, const coefs* k) {
	int32_t x = 0;
	for (; x + 16 <= w; x += 16) {
		uint8x16_t y16 = vld1q_u8(y + x);
		uint8x8_t u8, v8;
		if (layout == 0) {
			u8 = vld1_u8(u + x / 2);
			v8 = vld1_u8(v + x / 2);
		} else if (layout == 1) {
			uint8x8x2_t uv = vld2_u8(u + x);
			u8 = uv.val[0];
			v8 = uv.val[1];
		} else {
			uint8x8x2_t vu = vld2_u8(v + x);
			v8 = vu.val[0];
			u8 = vu.val[1];
		}
		uint8x8_t ulo = vzip1_u8(u8, u8), uhi = vzip2_u8(u8, u8);
		uint8x8_t vlo = vzip1_u8(v8, v8), vhi = vzip2_u8(v8, v8);
		uint8x8x3_t lo = yuv_half_neon(vget_low_u8(y16), ulo, vlo, k);
		uint8x8x3_t hi = yuv_half_neon(vget_high_u8(y16), uhi, vhi, k);
		uint8x16x4_t px;
		px.val[0] = vcombine_u8(lo.val[0], hi.val[0]);
		px.val[1] = vcombine_u8(lo.val[1], hi.val[1]);
		px.val[2] = vcombine_u8(lo.val[2], hi.val[2]);
		px.val[3] = vdupq_n_u8(255);
		vst4q_u8(d + 4 * x, px);
	}
	return x;
}
#endif

// yuv_convert converts a w x h 4:2:0 picture at its size. ps is the chroma
// pixel stride; the NEON loop runs for the planar and the two semi-planar
// layouts, the scalar one for the rest of each row and any other layout.
static void yuv_convert(const uint8_t* y, int32_t ys, const uint8_t* u, int32_t us, const uint8_t* v,
	int32_t vs, int32_t ps, int32_t w, int32_t h, uint8_t* dst, int32_t stride, int32_t bt709) {
	const coefs* k16 = bt709 ? &q16_709 : &q16_601;
	int32_t layout = -1;
	if (ps == 1) layout = 0;
	else if (ps == 2 && v == u + 1) layout = 1;
	else if (ps == 2 && u == v + 1) layout = 2;
#if defined(__aarch64__)
	const coefs* k6 = bt709 ? &q6_709 : &q6_601;
#endif
	for (int32_t row = 0; row < h; row++) {
		const uint8_t* yr = y + (size_t)row * ys;
		const uint8_t* ur = u + (size_t)(row >> 1) * us;
		const uint8_t* vr = v + (size_t)(row >> 1) * vs;
		uint8_t* d = dst + (size_t)row * stride;
		int32_t x0 = 0;
#if defined(__aarch64__)
		if (layout >= 0) x0 = yuv_row_neon(yr, ur, vr, layout, w, d, k6);
#endif
		yuv_row_scalar(yr, ur, vr, ps, x0, w, d, k16);
	}
}

// yuv_scale converts a w x h 4:2:0 picture to dw x dh, smaller, taking the
// nearest source pixel (16.16 fixed point, from pixel centres).
static void yuv_scale(const uint8_t* y, int32_t ys, const uint8_t* u, int32_t us, const uint8_t* v,
	int32_t vs, int32_t ps, int32_t w, int32_t h, uint8_t* dst, int32_t stride, int32_t dw, int32_t dh,
	int32_t bt709) {
	const coefs* k = bt709 ? &q16_709 : &q16_601;
	uint32_t sx = (uint32_t)(((uint64_t)w << 16) / (uint32_t)dw);
	uint32_t sy = (uint32_t)(((uint64_t)h << 16) / (uint32_t)dh);
	uint32_t fy = sy / 2;
	for (int32_t row = 0; row < dh; row++, fy += sy) {
		int32_t r = (int32_t)(fy >> 16);
		if (r >= h) r = h - 1;
		const uint8_t* yr = y + (size_t)r * ys;
		const uint8_t* ur = u + (size_t)(r >> 1) * us;
		const uint8_t* vr = v + (size_t)(r >> 1) * vs;
		uint8_t* d = dst + (size_t)row * stride;
		uint32_t fx = sx / 2;
		for (int32_t x = 0; x < dw; x++, fx += sx) {
			int32_t c = (int32_t)(fx >> 16);
			if (c >= w) c = w - 1;
			yuv_px(yr[c], ur[(c >> 1) * ps], vr[(c >> 1) * ps], d + 4 * x, k);
		}
	}
}

int64_t amp_convert_planes(const uint8_t* y, int32_t ys, const uint8_t* u, const uint8_t* v, int32_t uvs,
	int32_t ps, int32_t w, int32_t h, uint8_t* dst, int32_t stride, int32_t dw, int32_t dh, int32_t bt709) {
	if (w <= 0 || h <= 0 || dw <= 0 || dh <= 0 || dw > w || dh > h || ps <= 0 || y == NULL || u == NULL ||
		v == NULL || dst == NULL || stride < 4 * dw) {
		return -1;
	}
	int64_t t0 = amp_now();
	if (dw == w && dh == h) {
		yuv_convert(y, ys, u, uvs, v, uvs, ps, w, h, dst, stride, bt709);
	} else {
		yuv_scale(y, ys, u, uvs, v, uvs, ps, w, h, dst, stride, dw, dh, bt709);
	}
	return amp_now() - t0;
}

int64_t amp_convert(amp_frame* f, uint8_t* dst, int32_t stride, int32_t dw, int32_t dh, int32_t bt709) {
	int32_t l = f->crop[0], t = f->crop[1];
	int32_t w = f->crop[2] - l, h = f->crop[3] - t;
	if ((f->format != AIMAGE_FORMAT_YUV_420_888 && f->format != AMP_FORMAT_YV12) || f->planes < 3 || f->data[0] == NULL ||
		f->data[1] == NULL || f->data[2] == NULL || f->row_stride[1] != f->row_stride[2] ||
		f->pix_stride[1] != f->pix_stride[2]) {
		return -1;
	}
	int32_t ps = f->pix_stride[1];
	const uint8_t* y = f->data[0] + (size_t)t * f->row_stride[0] + l;
	const uint8_t* u = f->data[1] + (size_t)(t / 2) * f->row_stride[1] + (size_t)(l / 2) * ps;
	const uint8_t* v = f->data[2] + (size_t)(t / 2) * f->row_stride[2] + (size_t)(l / 2) * ps;
	return amp_convert_planes(y, f->row_stride[0], u, v, f->row_stride[1], ps, w, h, dst, stride, dw, dh, bt709);
}
