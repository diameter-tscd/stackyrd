package utils

import (
	"fmt"
	"os"
	"runtime/debug"

	"stackyrd/pkg/logger"
)

// GoSafe runs fn in the caller's goroutine, recovering any panic and logging
// the stack trace instead of crashing the process. log may be nil — the panic
// and stack then go to stderr.
func GoSafe(log *logger.Logger, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			if log != nil {
				log.Error("panic recovered", fmt.Errorf("%v", r), "stack", string(stack))
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "panic recovered: %v\n%s\n", r, stack)
			}
		}
	}()
	fn()
}
