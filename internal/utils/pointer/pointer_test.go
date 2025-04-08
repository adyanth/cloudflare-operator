package pointer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTo(t *testing.T) {
	t.Parallel()

	const testValue = "test-value"

	ptr := To(testValue)
	assert.Equal(t, testValue, *ptr)
}
