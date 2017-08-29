package logging

import (
	"fmt"
	"os"

	"github.com/mixpanel/obs/util"
)

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

func (fields Fields) Dupe() Fields {
	dupe := make(Fields, len(fields))
	for k, v := range fields {
		dupe[k] = v
	}
	return dupe
}

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
	hostInfo := util.GetHostInfo(localhost)
	if hostInfo == nil {
		initError(fmt.Sprintf("Unable to extract host info from %v.\n", localhost))
		return fields
	}

	for k, v := range hostInfo.Map() {
		fields[k] = v
	}
	return fields
}
