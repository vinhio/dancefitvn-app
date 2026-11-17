// Adapted from jplayer-mobile/internal/sys/link_android.go at 49de3c2
// (ledgerFile, callString0) and jplayer-mobile/internal/sys/java_android.go
// at 49de3c2 (lookupJava, lookupMethod), trimmed to
// Context.getNoBackupFilesDir and File.getAbsolutePath.

package sys

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// java holds the methods paths_android.go calls, looked up once, and the
// no-backup folder once read: it does not change while the app runs.
var java struct {
	mu                        sync.Mutex
	ok                        bool
	noBackupDir, absolutePath jni.MethodID
	noBackup                  string
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// dataBase is os.UserConfigDir: files/.config, gDesk setting HOME to the
// app's files folder (bridge/backend/android/cgo_android.go).
func dataBase() (string, error) { return os.UserConfigDir() }

// secretBase is Context.getNoBackupFilesDir(): the system never backs it
// up, so another phone restored from this one takes neither the tokens
// nor the install id.
func secretBase() (string, error) {
	java.mu.Lock()
	defer java.mu.Unlock()
	if java.noBackup != "" {
		return java.noBackup, nil
	}
	var dir string
	err := jni.Do(func(env jni.Env) error {
		if err := lookupJava(env); err != nil {
			return err
		}
		ctx := jni.Context()
		if ctx == 0 {
			return fmt.Errorf("no context: %w", core.ErrUnsupported)
		}
		f, err := env.CallObject(ctx, java.noBackupDir)
		if err != nil || f == 0 {
			return fmt.Errorf("no no-backup folder: %w", errors.Join(err, core.ErrNotFound))
		}
		defer env.DeleteLocalRef(f)
		s, err := env.CallObject(f, java.absolutePath)
		if err != nil || s == 0 {
			return fmt.Errorf("no no-backup path: %w", errors.Join(err, core.ErrNotFound))
		}
		defer env.DeleteLocalRef(s)
		dir, err = env.GoString(s)

		return err
	})
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", fmt.Errorf("empty no-backup path: %w", core.ErrNotFound)
	}
	java.noBackup = dir

	return dir, nil
}

// lookupJava looks the two methods up, once; the caller holds java.mu.
func lookupJava(env jni.Env) error {
	if java.ok {
		return nil
	}
	methods := []struct {
		id               *jni.MethodID
		class, name, sig string
	}{
		{id: &java.noBackupDir, class: "android/content/Context", name: "getNoBackupFilesDir", sig: "()Ljava/io/File;"},
		{id: &java.absolutePath, class: "java/io/File", name: "getAbsolutePath", sig: "()Ljava/lang/String;"},
	}
	for _, m := range methods {
		if err := lookupMethod(env, m.id, m.class, m.name, m.sig); err != nil {
			return err
		}
	}
	java.ok = true

	return nil
}

// lookupMethod looks one instance method up into id.
func lookupMethod(env jni.Env, id *jni.MethodID, class, name, sig string) error {
	c, err := env.FindClass(class)
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(jni.Object(c))
	*id, err = env.Method(c, name, sig)

	return err
}
