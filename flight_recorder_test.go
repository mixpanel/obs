package obs

import (
	"context"
	"testing"
)

func BenchmarkGetCallerContext(b *testing.B) {
	for i := 0; i < b.N; i++ {
		getCallerContext(1)
	}
}

func BenchmarkIncr(b *testing.B) {
	fs := NullFlightRecorder.WithSpan(context.Background())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fs.Incr("test")
	}
}
