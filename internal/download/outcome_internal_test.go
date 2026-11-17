package download

import "testing"

import "github.com/stretchr/testify/assert"

// TestOutcomeZeroIsUnknown checks the enum convention: the zero value is
// outcomeUnknown and every named value is distinct from it.
func TestOutcomeZeroIsUnknown(t *testing.T) {
	t.Parallel()

	var zero outcome
	assert.Equal(t, outcomeUnknown, zero)
	for _, o := range []outcome{outcomeDone, outcomeRefetch, outcomeRestart} {
		assert.NotEqual(t, outcomeUnknown, o)
	}
}
