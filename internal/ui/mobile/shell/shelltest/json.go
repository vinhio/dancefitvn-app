//go:build !tvos

package shelltest

import "encoding/json/v2"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// jsonUnmarshal decodes data into v with the json/v2 the model's tags are
// written for.
func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
