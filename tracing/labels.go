package tracing

var Label = struct {
	HTTPMethod, HTTPStatusCode, HTTPResponseSize, ServiceName, ServiceVersion, ErrorName, ErrorMessage string
}{
	HTTPMethod:       "trace.cloud.google.com/http/method",
	HTTPStatusCode:   "trace.cloud.google.com/http/status_code",
	HTTPResponseSize: "trace.cloud.google.com/http/response/size",
	ServiceName:      "trace.cloud.google.com/gae/app/module",
	ServiceVersion:   "trace.cloud.google.com/gae/app/version",
	ErrorName:        "trace.cloud.google.com/error/name",
	ErrorMessage:     "trace.cloud.google.com/error/message",
}
