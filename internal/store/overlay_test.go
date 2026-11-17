package store_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/store"
)

const mockDir = "../../mock/dancefitvn-mock-api"

// staticTokens is a TokenSource with one token.
type staticTokens struct{}

func (staticTokens) Token() string { return "tok" }
func (staticTokens) Refresh(context.Context, string) (string, error) {
	return "", api.ErrSignedOut
}

// rig is a client over an Overlay over the mock API.
type rig struct {
	mock    *mock.Transport
	overlay *store.Overlay
	client  *api.Client
}

func newRig(t *testing.T, b store.Backend) *rig {
	t.Helper()
	m, err := mock.New(os.DirFS(mockDir), mock.Options{
		Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close() })
	ov := store.NewOverlay(m, b)
	c, err := api.New(api.Options{
		Transport: ov, Version: "1.0.0", Platform: "ios", Tokens: staticTokens{}, Cache: api.NewMemoryCache(),
	})
	require.NoError(t, err)

	return &rig{mock: m, overlay: ov, client: c}
}

func TestOverlayPassesUnrelatedRoutesThrough(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	cfg, err := r.client.Config(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.MinVersion)
}

func TestOverlayProfileUpdate(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	before, err := r.client.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, "0905123456", before.User.Phone)

	_, err = r.client.UpdateMe(ctx, model.MeUpdateRequest{
		FirstName: "Vy", LastName: "Trần", Email: "thaovy.hlv@gmail.com", Phone: "0999000111",
	})
	require.NoError(t, err)

	me, err := r.client.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, "0999000111", me.User.Phone)
	assert.Equal(t, "Vy", me.User.FirstName)
	assert.Equal(t, "Trần Vy", me.User.FullName)
	assert.Equal(t, "u_412", me.User.ID, "other fields stay")

	home, err := r.client.Home(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Vy", home.GreetingName)
}

func TestOverlayEmailChangePendingKeepsEmail(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	// The mock answers with email_change_pending null, so the new e-mail applies.
	_, err := r.client.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "A", LastName: "B", Email: "new@example.com", Phone: "1"})
	require.NoError(t, err)
	me, err := r.client.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", me.User.Email)
}

func TestOverlayAvatar(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	_, err := r.client.UploadAvatar(ctx, []byte("img"), "a.jpg")
	require.NoError(t, err)
	me, err := r.client.Me(ctx)
	require.NoError(t, err)
	assert.Contains(t, me.User.AvatarURL, "?v=2")
}

func TestOverlayCoachProfile(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	_, err := r.client.UpdateCoachProfile(ctx, model.CoachUpdateRequest{
		Bio:     "Bio mới",
		Socials: []model.SocialURL{{URL: "https://www.instagram.com/vy"}, {URL: "https://youtu.be/xyz"}},
		Places:  []string{"Phòng tập A", "Phòng tập B"},
	})
	require.NoError(t, err)
	p, err := r.client.CoachProfile(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Bio mới", p.Bio)
	assert.Equal(t, []string{"Phòng tập A", "Phòng tập B"}, p.Places)
	assert.Equal(t, []model.Social{
		{Platform: "instagram", URL: "https://www.instagram.com/vy"},
		{Platform: "youtube", URL: "https://youtu.be/xyz"},
	}, p.Socials)
	assert.Equal(t, 6, p.MaxSocials, "other fields stay")
}

func TestOverlayProgressShowsInCourseAndHome(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	course, err := r.client.Course(ctx, "set-35")
	require.NoError(t, err)
	require.Equal(t, 2, course.Progress.WatchedCount)

	at := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	_, err = r.client.SaveProgress(ctx, "l_3505", model.ProgressRequest{PositionSec: 60, DurationSec: 200, ClientTime: at})
	require.NoError(t, err)

	course, err = r.client.Course(ctx, "set-35")
	require.NoError(t, err)
	l := lessonOf(t, course, "l_3505")
	assert.Equal(t, 60, l.Progress.PositionSec)
	assert.False(t, l.Progress.Watched)
	assert.Equal(t, 2, course.Progress.WatchedCount)
	require.NotNil(t, course.Resume)
	assert.Equal(t, "l_3505", course.Resume.LessonID)
	assert.Equal(t, 60, course.Resume.PositionSec)

	home, err := r.client.Home(ctx)
	require.NoError(t, err)
	require.NotNil(t, home.Continue)
	assert.Equal(t, "l_3505", home.Continue.Lesson.ID)
	assert.Equal(t, "set-35", home.Continue.Course.ID)
	assert.Equal(t, 60, home.Continue.PositionSec)
	assert.Equal(t, 140, home.Continue.RemainingSec)
}

func TestOverlayWatchedCountsOnce(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	_, err := r.client.Course(ctx, "set-35")
	require.NoError(t, err)

	// 198 of 220 is watched (90 %); saving it twice counts once.
	for range 2 {
		_, err = r.client.SaveProgress(ctx, "l_3503", model.ProgressRequest{PositionSec: 198, DurationSec: 220, ClientTime: time.Now()})
		require.NoError(t, err)
	}
	course, err := r.client.Course(ctx, "set-35")
	require.NoError(t, err)
	assert.True(t, lessonOf(t, course, "l_3503").Progress.Watched)
	assert.Equal(t, 3, course.Progress.WatchedCount)

	home, err := r.client.Home(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, home.MyCourses[0].Progress.WatchedCount)
	assert.Equal(t, 3, home.Stats.WatchedInCurrentCourse)
	courses, err := r.client.MyCourses(ctx, "", 0)
	require.NoError(t, err)
	assert.Equal(t, 3, courses.Items[0].Progress.WatchedCount)

	// 197 of 220 is not.
	r2 := newRig(t, store.NewMemory())
	_, err = r2.client.Course(ctx, "set-35")
	require.NoError(t, err)
	_, err = r2.client.SaveProgress(ctx, "l_3503", model.ProgressRequest{PositionSec: 197, DurationSec: 220, ClientTime: time.Now()})
	require.NoError(t, err)
	course, err = r2.client.Course(ctx, "set-35")
	require.NoError(t, err)
	assert.False(t, lessonOf(t, course, "l_3503").Progress.Watched)
	assert.Equal(t, 2, course.Progress.WatchedCount)
}

func TestOverlayWatchedGoesOnToNextLesson(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	require.NoError(t, r.mock.SetScenario("lessons.progress", "watched"))
	_, err := r.client.Course(ctx, "set-35")
	require.NoError(t, err)
	_, err = r.client.SaveProgress(ctx, "l_3503", model.ProgressRequest{PositionSec: 220, DurationSec: 220, ClientTime: time.Now()})
	require.NoError(t, err)

	home, err := r.client.Home(ctx)
	require.NoError(t, err)
	require.NotNil(t, home.Continue)
	assert.Equal(t, "l_3504", home.Continue.Lesson.ID)
	assert.Equal(t, 0, home.Continue.PositionSec)
	assert.Equal(t, 250, home.Continue.RemainingSec)
	assert.Equal(t, "set-35", home.Continue.Course.ID)

	course, err := r.client.Course(ctx, "set-35")
	require.NoError(t, err)
	require.NotNil(t, course.Resume)
	assert.Equal(t, "l_3504", course.Resume.LessonID)
}

func TestOverlayBatchKeepsNewestPerLesson(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	_, err := r.client.Course(ctx, "set-35")
	require.NoError(t, err)

	t1 := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	_, err = r.client.SaveProgress(ctx, "l_3506", model.ProgressRequest{PositionSec: 100, DurationSec: 235, ClientTime: t1.Add(time.Hour)})
	require.NoError(t, err)
	_, err = r.client.SendProgressBatch(ctx, model.BatchRequest{Items: []model.BatchItem{
		{LessonID: "l_3506", PositionSec: 30, DurationSec: 235, ClientTime: t1},                // older: ignored
		{LessonID: "l_3507", PositionSec: 235, DurationSec: 235, ClientTime: t1},               // watched
		{LessonID: "l_3508", PositionSec: 10, DurationSec: 200, ClientTime: t1.Add(time.Hour)}, // new
	}})
	require.NoError(t, err)

	course, err := r.client.Course(ctx, "set-35")
	require.NoError(t, err)
	assert.Equal(t, 100, lessonOf(t, course, "l_3506").Progress.PositionSec)
	assert.True(t, lessonOf(t, course, "l_3507").Progress.Watched)
	assert.Equal(t, 10, lessonOf(t, course, "l_3508").Progress.PositionSec)
	assert.Equal(t, 3, course.Progress.WatchedCount)
}

func TestOverlayDeviceDelete(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	before, err := r.client.Devices(ctx)
	require.NoError(t, err)
	require.Len(t, before.Items, 3)
	meBefore, err := r.client.Me(ctx)
	require.NoError(t, err)

	require.NoError(t, r.client.DeleteDevice(ctx, "d_102"))
	require.NoError(t, r.client.DeleteDevice(ctx, "d_102"))

	after, err := r.client.Devices(ctx)
	require.NoError(t, err)
	require.Len(t, after.Items, 2)
	for _, d := range after.Items {
		assert.NotEqual(t, "d_102", d.ID)
	}
	me, err := r.client.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, meBefore.Devices.Active-1, me.Devices.Active)
}

// TestOverlayLoginReplacingDevice checks K31 in the mock's world: a login that replaced a device
// takes it out of the device list, as the server's revocation does; a login that did not, or one the
// mock refuses, leaves the list as it was.
func TestOverlayLoginReplacingDevice(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	_, err := r.client.Login(ctx, "a@b.c", "x", model.DeviceInfo{})
	require.NoError(t, err)
	devs, err := r.client.Devices(ctx)
	require.NoError(t, err)
	assert.Len(t, devs.Items, 3, "a plain login revokes nothing")

	require.NoError(t, r.mock.SetScenario("auth.login", "device_limit"))
	_, err = r.client.LoginReplacing(ctx, "a@b.c", "x", model.DeviceInfo{}, "")
	require.Error(t, err)
	require.NoError(t, r.mock.SetScenario("auth.login", "invalid_credentials"))
	_, err = r.client.LoginReplacing(ctx, "a@b.c", "x", model.DeviceInfo{}, "d_102")
	require.Error(t, err)
	devs, err = r.client.Devices(ctx)
	require.NoError(t, err)
	assert.Len(t, devs.Items, 3, "a refused login revokes nothing")

	require.NoError(t, r.mock.SetScenario("auth.login", "device_limit"))
	_, err = r.client.LoginReplacing(ctx, "a@b.c", "x", model.DeviceInfo{}, "d_102")
	require.NoError(t, err)
	devs, err = r.client.Devices(ctx)
	require.NoError(t, err)
	require.Len(t, devs.Items, 2)
	for _, d := range devs.Items {
		assert.NotEqual(t, "d_102", d.ID)
	}
}

func TestOverlayFailedWriteIsNotRecorded(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	require.NoError(t, r.mock.SetScenario("me.update", "validation_failed"))
	_, err := r.client.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "X", LastName: "Y", Email: "x@y.z", Phone: "7"})
	require.Error(t, err)
	me, err := r.client.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, "0905123456", me.User.Phone)

	require.NoError(t, r.mock.SetScenario("devices.delete", "not_found"))
	require.Error(t, r.client.DeleteDevice(ctx, "d_102"))
	devs, err := r.client.Devices(ctx)
	require.NoError(t, err)
	assert.Len(t, devs.Items, 3)
}

func TestOverlayNoStaleETag(t *testing.T) {
	t.Parallel()

	// The client caches by ETag; an overlaid answer must not be served from
	// that cache after the record changes.
	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	for i := range 3 {
		_, err := r.client.Me(ctx)
		require.NoError(t, err)
		_, err = r.client.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "N", LastName: "L", Email: "e@x.y", Phone: string(rune('1' + i))})
		require.NoError(t, err)
		me, err := r.client.Me(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(rune('1'+i)), me.User.Phone)
	}
}

func TestOverlayDropsETag(t *testing.T) {
	t.Parallel()

	m, err := mock.New(os.DirFS(mockDir), mock.Options{Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close() })
	ov := store.NewOverlay(m, store.NewMemory())
	h := http.Header{}
	h.Set("Authorization", "Bearer t")
	h.Set("If-None-Match", `"anything"`)
	resp, err := ov.Do(t.Context(), &api.Request{RouteID: "me.get", Method: http.MethodGet, Path: "/me", Header: h})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.Status, "If-None-Match is not passed on")
	assert.Empty(t, resp.Header.Get("ETag"))
}

func TestOverlayPersists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := t.Context()
	r := newRig(t, store.NewDir(dir))
	_, err := r.client.Course(ctx, "set-35")
	require.NoError(t, err)
	_, err = r.client.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "Vy", LastName: "Lê", Email: "e@x.y", Phone: "0123"})
	require.NoError(t, err)
	_, err = r.client.SaveProgress(ctx, "l_3503", model.ProgressRequest{PositionSec: 100, DurationSec: 220, ClientTime: time.Now()})
	require.NoError(t, err)
	require.NoError(t, r.client.DeleteDevice(ctx, "d_201"))
	require.NoError(t, r.overlay.SaveErr())
	assert.FileExists(t, dir+"/mock-overlay.json")

	// a new process: new overlay, same folder
	r2 := newRig(t, store.NewDir(dir))
	me, err := r2.client.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, "0123", me.User.Phone)
	assert.Equal(t, "Lê Vy", me.User.FullName)
	course, err := r2.client.Course(ctx, "set-35")
	require.NoError(t, err)
	assert.Equal(t, 100, lessonOf(t, course, "l_3503").Progress.PositionSec)
	devs, err := r2.client.Devices(ctx)
	require.NoError(t, err)
	assert.Len(t, devs.Items, 2)

	// Reset forgets everything, on disk too
	require.NoError(t, r2.overlay.Reset())
	r3 := newRig(t, store.NewDir(dir))
	me, err = r3.client.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, "0905123456", me.User.Phone)
	devs, err = r3.client.Devices(ctx)
	require.NoError(t, err)
	assert.Len(t, devs.Items, 3)
}

func TestOverlayCorruptRecordStartsEmpty(t *testing.T) {
	t.Parallel()

	b := store.NewMemory()
	b.Put("mock-overlay", []byte("not json"))
	r := newRig(t, b)
	me, err := r.client.Me(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "0905123456", me.User.Phone)
}

func TestOverlaySaveFailureDoesNotFailTheCall(t *testing.T) {
	t.Parallel()

	b := store.NewMemory()
	b.SaveErr = assert.AnError
	r := newRig(t, b)
	ctx := t.Context()
	_, err := r.client.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "A", LastName: "B", Email: "e@x.y", Phone: "42"})
	require.NoError(t, err)
	assert.ErrorIs(t, r.overlay.SaveErr(), assert.AnError)
	me, err := r.client.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, "42", me.User.Phone, "kept in memory")
}

func TestOverlayConcurrent(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	done := make(chan struct{})
	for i := range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = r.client.Course(ctx, "set-35")
			_, _ = r.client.SaveProgress(ctx, "l_3503", model.ProgressRequest{PositionSec: i, DurationSec: 220, ClientTime: time.Now()})
			_, _ = r.client.Home(ctx)
			_, _ = r.client.Me(ctx)
		}()
	}
	for range 8 {
		<-done
	}
}

// lessonOf returns the lesson id of c.
func lessonOf(t *testing.T, c model.Course, id string) model.Lesson {
	t.Helper()
	for _, l := range c.Lessons {
		if l.ID == id {
			return l
		}
	}
	require.Failf(t, "lesson not found", id)

	return model.Lesson{}
}

func TestOverlayPairConfirmRestoresDevice(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	ctx := t.Context()
	meBefore, err := r.client.Me(ctx)
	require.NoError(t, err)

	hasTV := func() bool {
		devs, err := r.client.Devices(ctx)
		require.NoError(t, err)
		for _, d := range devs.Items {
			if d.Platform == model.PlatformTVOS {
				return true
			}
		}

		return false
	}
	require.True(t, hasTV())

	require.NoError(t, r.client.DeleteDevice(ctx, "d_201"))
	assert.False(t, hasTV())
	me, err := r.client.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, meBefore.Devices.Active-1, me.Devices.Active)

	got, err := r.client.ConfirmPairing(ctx, "7K4M2P")
	require.NoError(t, err)
	assert.Equal(t, "d_201", got.Device.ID)
	assert.True(t, hasTV())
	me, err = r.client.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, meBefore.Devices.Active, me.Devices.Active)

	// Delete and pair again, and a failed confirm changes nothing.
	require.NoError(t, r.client.DeleteDevice(ctx, "d_201"))
	require.NoError(t, r.mock.SetScenario("tv.pair.confirm", "invalid_code"))
	_, err = r.client.ConfirmPairing(ctx, "7K4M2P")
	require.Error(t, err)
	assert.False(t, hasTV())
}

func TestOverlayPairConfirmSurvivesRestart(t *testing.T) {
	t.Parallel()

	b := store.NewMemory()
	r := newRig(t, b)
	ctx := t.Context()
	require.NoError(t, r.client.DeleteDevice(ctx, "d_201"))
	r2 := newRig(t, b)
	devs, err := r2.client.Devices(ctx)
	require.NoError(t, err)
	assert.Len(t, devs.Items, 2)
	_, err = r2.client.ConfirmPairing(ctx, "7K4M2P")
	require.NoError(t, err)
	r3 := newRig(t, b)
	devs, err = r3.client.Devices(ctx)
	require.NoError(t, err)
	assert.Len(t, devs.Items, 3)
}
