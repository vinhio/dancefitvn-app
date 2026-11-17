package com.jivecode.dancefitvn;

import android.content.Context;
import android.content.DialogInterface;
import android.content.pm.PackageManager;
import android.hardware.biometrics.BiometricManager;
import android.hardware.biometrics.BiometricPrompt;
import android.os.Build;
import android.os.CancellationSignal;
import java.util.concurrent.Executor;

/**
 * The system's biometric sheet for internal/sys (F6, until gDesk's G3).
 *
 * <p>BiometricPrompt.AuthenticationCallback is an abstract class, which
 * jni.NewProxy (a java.lang.reflect.Proxy, interfaces only) cannot
 * implement: {@link Callback} extends it and forwards every answer, once,
 * to a {@link Listener}, an interface Go implements with jni.NewProxy.
 *
 * <p>This class itself uses nothing newer than API 26 at load time, so Go
 * may call {@link #availability} on any Android; {@link Callback}, which
 * needs API 28, is only loaded by {@link #authenticate} past the check.
 * Framework APIs only, Java 8's language (gDesk's java/README.md rules).
 */
public final class BiometricBridge {
    /** What Go implements: the sheet's one answer. */
    public interface Listener {
        /**
         * @param status SUCCEEDED, ERROR (code is BIOMETRIC_ERROR_*),
         *     FAILED_LIMIT or NOT_SHOWN (message is the exception)
         * @param code the BiometricPrompt error code, 0 otherwise
         * @param message the system's text, "" when none
         */
        void onResult(int status, int code, String message);
    }

    /** The user was recognized. */
    public static final int SUCCEEDED = 0;
    /** onAuthenticationError, or the negative button (BIOMETRIC_ERROR_NEGATIVE_BUTTON). */
    public static final int ERROR = 1;
    /** FAILED_TRIES tries were not recognized: the bridge dismissed the sheet. */
    public static final int FAILED_LIMIT = 2;
    /** The sheet could not be shown. */
    public static final int NOT_SHOWN = 3;

    /** How many unrecognized tries dismiss the sheet (03 §3 "sai 3 lần"); Go's failedLimit. */
    public static final int FAILED_TRIES = 3;

    /** availability's answer below API 28: no BiometricPrompt. */
    public static final int TOO_OLD = -1;

    /** BiometricPrompt.BIOMETRIC_ERROR_NEGATIVE_BUTTON (framework constant of API 28). */
    private static final int NEGATIVE_BUTTON = 13;
    /** BiometricManager.BIOMETRIC_SUCCESS. */
    private static final int SUCCESS = 0;
    /** BiometricManager.BIOMETRIC_ERROR_NO_HARDWARE. */
    private static final int NO_HARDWARE = 12;

    /** The bits of {@link #features}. */
    private static final int FINGERPRINT = 1;
    private static final int FACE = 2;
    private static final int IRIS = 4;

    private BiometricBridge() {}

    /**
     * Whether a biometry can check the user now: BiometricManager's
     * BIOMETRIC_SUCCESS (0) or BIOMETRIC_ERROR_* (API 30+:
     * canAuthenticate(BIOMETRIC_WEAK); API 29: canAuthenticate()); API 28,
     * which has no BiometricManager, a sensor declared (enrollment shows
     * when the sheet is asked for); TOO_OLD below API 28.
     */
    public static int availability(Context context) {
        if (Build.VERSION.SDK_INT < 28) {
            return TOO_OLD;
        }
        if (Build.VERSION.SDK_INT >= 29) {
            BiometricManager m = context.getSystemService(BiometricManager.class);
            if (m == null) {
                return NO_HARDWARE;
            }
            if (Build.VERSION.SDK_INT >= 30) {
                return m.canAuthenticate(BiometricManager.Authenticators.BIOMETRIC_WEAK);
            }
            return m.canAuthenticate();
        }
        return features(context) != 0 ? SUCCESS : NO_HARDWARE;
    }

    /** The biometric sensors the device declares: FINGERPRINT | FACE | IRIS bits. */
    public static int features(Context context) {
        PackageManager pm = context.getPackageManager();
        int f = 0;
        if (pm.hasSystemFeature(PackageManager.FEATURE_FINGERPRINT)) {
            f |= FINGERPRINT;
        }
        if (Build.VERSION.SDK_INT >= 29) {
            if (pm.hasSystemFeature(PackageManager.FEATURE_FACE)) {
                f |= FACE;
            }
            if (pm.hasSystemFeature(PackageManager.FEATURE_IRIS)) {
                f |= IRIS;
            }
        }
        return f;
    }

    /**
     * Shows the sheet; the listener gets one answer, on the main thread.
     * Call it on the main thread. The CancellationSignal dismisses the
     * sheet (the answer is then ERROR with BIOMETRIC_ERROR_CANCELED); it is
     * null when the sheet was not shown (the listener got NOT_SHOWN).
     *
     * @param context the activity (the sheet shows over it)
     * @param title the sheet's title
     * @param description the text under it, "" for none
     * @param negative the cancel button's text, "" for the system's "Cancel"
     */
    public static CancellationSignal authenticate(Context context, String title, String description,
            String negative, Listener listener) {
        if (Build.VERSION.SDK_INT < 28) {
            listener.onResult(NOT_SHOWN, 0, "BiometricPrompt needs API 28");
            return null;
        }
        return Callback.show(context, title, description, negative, listener);
    }

    /** The sheet's callback and its cancel button's listener, answering once. */
    static final class Callback extends BiometricPrompt.AuthenticationCallback
            implements DialogInterface.OnClickListener {
        private Listener listener;
        private CancellationSignal signal;
        private int failed;

        private Callback(Listener listener) {
            this.listener = listener;
        }

        static CancellationSignal show(Context context, String title, String description, String negative,
                Listener listener) {
            Callback cb = new Callback(listener);
            try {
                Executor main = context.getMainExecutor();
                if (negative == null || negative.isEmpty()) {
                    negative = context.getString(android.R.string.cancel);
                }
                BiometricPrompt.Builder b = new BiometricPrompt.Builder(context)
                        .setTitle(title)
                        .setNegativeButton(negative, main, cb);
                if (description != null && !description.isEmpty()) {
                    b.setDescription(description);
                }
                if (Build.VERSION.SDK_INT >= 29) {
                    b.setConfirmationRequired(false); // a recognized face signs in at once
                }
                cb.signal = new CancellationSignal();
                b.build().authenticate(cb.signal, main, cb);
                return cb.signal;
            } catch (RuntimeException e) {
                cb.finish(NOT_SHOWN, 0, String.valueOf(e));
                return null;
            }
        }

        @Override
        public void onAuthenticationSucceeded(BiometricPrompt.AuthenticationResult result) {
            finish(SUCCEEDED, 0, "");
        }

        @Override
        public void onAuthenticationError(int code, CharSequence message) {
            finish(ERROR, code, message == null ? "" : message.toString());
        }

        @Override
        public void onAuthenticationFailed() {
            failed++;
            if (failed >= FAILED_TRIES && listener != null) {
                finish(FAILED_LIMIT, 0, "");
                if (signal != null) {
                    signal.cancel(); // its BIOMETRIC_ERROR_CANCELED is not reported again
                }
            }
        }

        @Override
        public void onClick(DialogInterface dialog, int which) {
            finish(ERROR, NEGATIVE_BUTTON, "");
        }

        private void finish(int status, int code, String message) {
            Listener l = listener;
            if (l == null) {
                return;
            }
            listener = null;
            l.onResult(status, code, message);
        }
    }
}
