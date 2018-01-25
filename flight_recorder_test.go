package obs

import "testing"

func BenchmarkGetCallerContext(b *testing.B) {
	for i := 0; i < b.N; i++ {
		getCallerContext(1)
	}
}
