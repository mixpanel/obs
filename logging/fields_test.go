package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateFields(t *testing.T) {
	lhs := Fields{"key": "value"}
	rhs := Fields{"key2": "value2"}

	merged := lhs.Update(rhs)
	assert.Equal(t, "value", merged["key"])
	assert.Equal(t, "value2", merged["key2"])
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

func TestPopulateStandardFields(t *testing.T) {
	fields := Fields{}
	savedLocalFields := localhostFields
	defer func() {
		localhostFields = savedLocalFields
	}()
	localhostFields = Fields{"localhostKey": "value"}
	fields.populateStandardFields(levelDebug, "test logger")
	assert.Equal(t, "DEBUG", fields["level"])
	assert.Equal(t, "test logger", fields["logger"])
	assert.Equal(t, "value", fields["localhostKey"])
}

func TestLocalhostFields(t *testing.T) {
	assert.NotNil(t, localhostFields)
	assert.NotNil(t, localhostFields["pid"])
	assert.NotNil(t, localhostFields["executable"])
	assert.NotNil(t, localhostFields["hostname"])
	assert.NotNil(t, localhostFields["role"])
}
