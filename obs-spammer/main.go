package main

import (
	"context"
	"log"
	"obs"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fr, done := obs.InitGCP(context.Background(), "obs-spammer")
	defer done()
	tick := time.After(0)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sig:
			return
		case <-tick:
			ctx := context.Background()
			spam(ctx, fr)
			tick = time.After(60 * time.Second)
		}
	}
}

func spam(ctx context.Context, fr obs.FlightRecorder) {
	fs, ctx, done := fr.WithNewSpan(ctx, "spam")
	defer done()
	fs.Incr("test_counter", 1)
	sw := fs.StartStopwatch("latency")
	defer sw.Stop()
	fs.Info("some info message", obs.Vals{"field": "a"})
	fs.Debug("some info message", obs.Vals{"field": "a"})
	fs.Trace("some info message", obs.Vals{"field": "a"})

	fs.Warn("a_warning", "warning message", obs.Vals{"field": "b"})
	fs.Critical("a_crtical", "critcal message", obs.Vals{"field": "c"})

	fs.SetGauge("test_gauge", 1.2345)
	log.Println("test message to stdlib log")
}
