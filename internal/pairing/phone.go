package pairing

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// CodeLen is the length of a pairing code.
const CodeLen = 6

// Confirmer confirms a TV code; *api.Client implements it.
type Confirmer interface {
	ConfirmPairing(ctx context.Context, code string) (model.PairConfirm, error)
}

// Phone is the mobile side: it sends the code shown on the TV.
type Phone struct {
	client Confirmer
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewPhone returns a Phone that confirms through c.
func NewPhone(c Confirmer) *Phone { return &Phone{client: c} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Confirm normalises what the user typed and sends it by
// POST /tv/pair/confirm. An input that is not a code is core.ErrInvalid
// and sends nothing. The API's refusals come back as *api.Error:
// pair_code_invalid (404), device_limit (409).
func (p *Phone) Confirm(ctx context.Context, input string) (model.PairConfirm, error) {
	code, err := NormalizeCode(input)
	if err != nil {
		return model.PairConfirm{}, err
	}
	res, err := p.client.ConfirmPairing(ctx, code)
	if err != nil {
		return model.PairConfirm{}, fmt.Errorf("pairing: confirm: %w", err)
	}

	return res, nil
}

// NormalizeCode removes dashes and spaces and upper-cases: "7k4-m2p" is
// "7K4M2P". It accepts exactly CodeLen characters of [A-Z0-9].
func NormalizeCode(input string) (string, error) {
	var b strings.Builder
	for _, r := range input {
		switch {
		case r == '-' || r == ' ':
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 'a' + 'A')
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			return "", fmt.Errorf("pairing: code has a character outside A-Z0-9: %w", core.ErrInvalid)
		}
	}
	if b.Len() != CodeLen {
		return "", fmt.Errorf("pairing: code must have %d characters: %w", CodeLen, core.ErrInvalid)
	}

	return b.String(), nil
}
