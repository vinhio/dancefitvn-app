//go:build ios && !tvos

package sys

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// audioBase is DataDir, Library/Application Support/<AppID>: the audio
// folder stays where it was, PrepareAudioDir flags it out of backups.
func audioBase() (string, error) { return DataDir() }
