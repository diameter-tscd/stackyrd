package cache

import (
	"testing"
)

func BenchmarkCacheGet(b *testing.B) {
	c := NewShardedCache[string]()
	
	for i := 0; i < 1000; i++ {
		c.Set(string(rune(i)), "value"+string(rune(i)), 0)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			_, _ = c.Get(string(rune(j)))
		}
	}
}