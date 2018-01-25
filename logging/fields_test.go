package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeFields(t *testing.T) {
	lhs := Fields{"key": "value"}
	rhs := Fields{"key2": "value2"}

	lhs = MergeFields(lhs, rhs)
	assert.Equal(t, "value", lhs["key"])
	assert.Equal(t, "value2", lhs["key2"])
}

func TestMergeEmptyFields(t *testing.T) {
	var lhs Fields
	rhs := Fields{"key": "value"}

	lhs = MergeFields(lhs, rhs)
	assert.Equal(t, "value", lhs["key"])
}

func TestDupeFields(t *testing.T) {
	lhs := Fields{"key": "value"}
	duped := lhs.Dupe()
	lhs["key"] = "value2"
	assert.Equal(t, "value", duped["key"])
}

type testError struct {
	PublicField string
	message     string
}

func (te testError) Error() string {
	return te.message
}

func TestWithError(t *testing.T) {
	fields := Fields{"key": "value"}
	fields = fields.WithError(testError{"Public", "message"})
	assert.Equal(t, "message", fields["error_message"])
}

func TestLocalhostFields(t *testing.T) {
	assert.NotNil(t, localhostFields)
	assert.NotNil(t, localhostFields["pid"])
	assert.NotNil(t, localhostFields["executable"])
	assert.NotNil(t, localhostFields["hostname"])
}
