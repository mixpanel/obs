# obs
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
