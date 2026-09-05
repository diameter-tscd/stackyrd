package main_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain runs every test in this integration package under goleak so a
// leaked goroutine (idle cron tickers, kafka pollers, redis reconnections,
// websocket read loops, infra Close() paths) fails the whole package instead of
// silently lingering in the test process.
//
// This is the single entry point for the integration suite, so one guard
// covers all packages that share the main_test package declaration.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}