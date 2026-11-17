package api

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// multipartFile encodes image as the form field "file" and returns the body
// and its Content-Type.
func multipartFile(image []byte, name string) ([]byte, string, error) {
	if len(image) == 0 {
		return nil, "", fmt.Errorf("api: empty image: %w", core.ErrInvalid)
	}
	if name == "" {
		name = "avatar.jpg"
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`,
		strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\r", "", "\n", "").Replace(name)))
	h.Set("Content-Type", http.DetectContentType(image))

	part, err := w.CreatePart(h)
	if err != nil {
		return nil, "", fmt.Errorf("api: multipart: %w", err)
	}
	if _, err := part.Write(image); err != nil {
		return nil, "", fmt.Errorf("api: multipart: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("api: multipart: %w", err)
	}

	return buf.Bytes(), w.FormDataContentType(), nil
}
