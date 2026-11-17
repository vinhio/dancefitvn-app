//go:build android

package app

import (
	"fmt"
	"time"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// localZone returns the zone of java.util.TimeZone.getDefault(), which is
// the zone persist.sys.timezone names, loaded from the system's tzdata
// (which Go on Android reads). It asks the VM through JNI, so the app
// needs no cgo here.
func localZone() (*time.Location, error) {
	var name string
	err := jni.Do(func(env jni.Env) error {
		cls, err := env.FindClass("java/util/TimeZone")
		if err != nil {
			return err
		}
		m, err := env.StaticMethod(cls, "getDefault", "()Ljava/util/TimeZone;")
		if err != nil {
			return err
		}
		tz, err := env.CallStaticObject(cls, m)
		if err != nil {
			return err
		}
		if tz == 0 {
			return fmt.Errorf("default zone is null: %w", core.ErrNotFound)
		}
		getID, err := env.Method(cls, "getID", "()Ljava/lang/String;")
		if err != nil {
			return err
		}
		id, err := env.CallObject(tz, getID)
		if err != nil {
			return err
		}
		name, err = env.GoString(id)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("local zone: %w", err)
	}
	if name == "" {
		return nil, fmt.Errorf("local zone: empty id: %w", core.ErrNotFound)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("local zone %q: %w: %w", name, core.ErrNotFound, err)
	}

	return loc, nil
}
