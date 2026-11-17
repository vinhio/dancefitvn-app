package model

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ErrorBody is the body of every HTTP error of the API (errors/*.json).
type ErrorBody struct {
	Error APIError `json:"error"`
}

// APIError is the detail of an error; only the fields of its code are set.
type APIError struct {
	Code         string            `json:"code"`
	Message      string            `json:"message"`
	Fields       map[string]string `json:"fields,omitzero"`
	RetryAfter   int               `json:"retry_after,omitzero"`
	AttemptsLeft int               `json:"attempts_left,omitzero"`
	Devices      []Device          `json:"devices,omitzero"`
	EnrollURL    string            `json:"enroll_url,omitzero"`
	MaxBytes     int64             `json:"max_bytes,omitzero"`
	StoreURL     map[string]string `json:"store_url,omitzero"`
}
