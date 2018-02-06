# obs [![GoDoc][doc-img]][doc] [![Build Status][ci-img]][ci]
Fast and buffered stats, logging and tracing in Go.

## Installation
```go get -u github.com/mixpanel/obs```

## Abstract
Mixpanel obs provides a unified interface for emitting metrics, logging as well as tracing.

```FlightRecorder``` is the unified interface. Make an instance of flight recorder or
use one of the utility methods. 

```
fr := obs.InitGCP(ctx, "my-service", "INFO")
```

or 

```
fr := NewFlightRecorder("my-service", mr, logger, tracer)
```


After obtaining a `FlightRecorder`, for telemetry you must obtain a span. 

```
fs := fr.WithSpan(ctx)

fs.Info("my info log")
fs.Incr("my.counter")
```
<hr/>
Released under the [MIT License](LICENSE).

[doc-img]: https://godoc.org/github.com/mixpanel/obs?status.svg
[doc]: https://godoc.org/github.com/mixpanel/obs
[ci-img]: https://api.travis-ci.org/mixpanel/obs.svg?branch=master
[ci]: https://travis-ci.org/mixpanel/obs
