package cache

import (
	"testing"
	"time"
)

func BenchmarkGet(b *testing.B) {
	c := New[string]()
	defer c.Close()

	for i := 0; i < 1000; i++ {
		c.Set(string(rune(i)), "value", 0)
	}

	b.ResetTimer()
	for b.Loop() {
		for i := 0; i < 1000; i++ {
			_, _ = c.Get(string(rune(i)))
		}
	}
}

func BenchmarkGetWithExpired(b *testing.B) {
	c := New[string]()
	defer c.Close()

	for i := 0; i < 1000; i++ {
		c.Set(string(rune(i)), "value", 100*time.Millisecond)
	}

	b.ResetTimer()
	for b.Loop() {
		for i := 0; i < 1000; i++ {
			_, _ = c.Get(string(rune(i)))
		}
	}
}