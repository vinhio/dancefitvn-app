package model

import "time"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// AppConfig is the answer of GET /app/config. The version maps are keyed by
// platform text ("ios", "android", "tvos").
type AppConfig struct {
	MinVersion    map[string]string `json:"min_version"`
	LatestVersion map[string]string `json:"latest_version"`
	ForceUpdate   bool              `json:"force_update"`
	Features      Features          `json:"features"`
	Download      DownloadConfig    `json:"download"`
	Progress      ProgressConfig    `json:"progress"`
	Support       Support           `json:"support"`
	Links         Links             `json:"links"`
	ServerTime    time.Time         `json:"server_time,omitzero"`
}

// Features are the switches the server can turn.
type Features struct {
	BeatCounter bool `json:"beat_counter"`
	DownloadAll bool `json:"download_all"`
	TVPairing   bool `json:"tv_pairing"`
	FaceIDLogin bool `json:"face_id_login"`
}

// DownloadConfig holds the download defaults.
type DownloadConfig struct {
	WiFiOnlyDefault bool `json:"wifi_only_default"`
	MaxConcurrent   int  `json:"max_concurrent"`
}

// ProgressConfig holds how progress is saved.
type ProgressConfig struct {
	SaveIntervalSec  int     `json:"save_interval_sec"`
	WatchedThreshold float64 `json:"watched_threshold"`
}

// Support holds the support contacts.
type Support struct {
	HotlineName string `json:"hotline_name"`
	Hotline     string `json:"hotline"`
	Email       string `json:"email"`
	Website     string `json:"website"`
}

// Links holds the web pages the app opens.
type Links struct {
	Register       string `json:"register"`
	ForgotPassword string `json:"forgot_password"`
	Privacy        string `json:"privacy"`
	// Terms is the terms of use (K34); optional: the mock lacks it and the app
	// hides its row while it is empty.
	Terms         string `json:"terms"`
	DeleteAccount string `json:"delete_account"`
}
