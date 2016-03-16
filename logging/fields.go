package logging

import (
	"fmt"
	"os"
	"util"
)

type Fields map[string]interface{}

var localhostFields Fields

func init() {
	localhostFields = getLocalhostFields()
}

func (fields Fields) Update(updates Fields) Fields {
	for k, v := range updates {
		fields[k] = v
	}
	return fields
}

func (fields Fields) Dupe() Fields {
	dupe := make(map[string]interface{}, len(fields))
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
	/*
		TODO: these are present in python but not yet implemented here:
			path to file (available from runtime.Caller)
			function name (available from runtime.Stack)
			line number (available from runtime.Caller)
			error defaults to filepath:function
			exception (passed in as an arg)
				- type
				- trace
				- message
	*/
	fields := make(map[string]interface{})
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
