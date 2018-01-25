package logging

import (
	"fmt"
	"os"
)

// Fields are used to add additional context to your log message
type Fields map[string]interface{}

var localhostFields Fields

func init() {
	localhostFields = getLocalhostFields()
}

// MergeFields creates a new Fields set by merging a and b.
func MergeFields(a, b Fields) Fields {
	merged := make(Fields, len(a)+len(b))
	for k, v := range a {
		merged[k] = v
	}
	for k, v := range b {
		merged[k] = v
	}
	return merged
}

// Dupe makes a copy of the fields
func (fields Fields) Dupe() Fields {
	dupe := make(Fields, len(fields))
	for k, v := range fields {
		dupe[k] = v
	}
	return dupe
}

// WithError is used to add the full error
// string to the context. This is useful
// especially with the obserr package
func (fields Fields) WithError(err error) Fields {
	res := fields.Dupe()
	res["error_message"] = fmt.Sprintf("%v", err)
	return res
}

func getLocalhostFields() Fields {
	fields := make(Fields)
	fields["pid"] = os.Getpid()
	fields["executable"] = os.Args[0]
	fields["argv"] = os.Args

	localhost, err := os.Hostname()
	if err != nil {
		initError(fmt.Sprintf("Unable to lookup localhost hostname.\n"))
		return fields
	}
	fields["hostname"] = localhost

	return fields
}
