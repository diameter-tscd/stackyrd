package services

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain enforces goroutine-leak detection for this integration subpackage.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}