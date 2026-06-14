package terminal

import (
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/term"
)

var ErrNotTerminal = errors.New("stdin is not a terminal")

var osExit = os.Exit

type Guard struct {
	mu       sync.Mutex
	fd       int
	oldState *term.State
	restored bool
}

func NewGuard() (*Guard, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, ErrNotTerminal
	}
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	return &Guard{fd: fd, oldState: oldState}, nil
}

func (g *Guard) Restore() {
	if g == nil || g.restored || g.oldState == nil {
		return
	}
	g.mu.Lock()
	if g.restored {
		g.mu.Unlock()
		return
	}
	g.restored = true
	g.mu.Unlock()
	_ = term.Restore(g.fd, g.oldState)
}

func (g *Guard) Restored() bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.restored
}

func GuardWithSignal() (*Guard, chan os.Signal) {
	g, err := NewGuard()
	if err != nil {
		os.Stderr.WriteString("raw mode: " + err.Error() + "\n")
		osExit(1)
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		osExit(signalExitCode(g, <-sigCh))
	}()
	return g, sigCh
}

func signalExitCode(g *Guard, sig os.Signal) int {
	g.Restore()
	switch sig {
	case syscall.SIGINT:
		return 130
	case syscall.SIGTERM:
		return 143
	default:
		return 1
	}
}

func (g *Guard) HandlePanic() {
	if r := recover(); r != nil {
		g.Restore()
		panic(r)
	}
}
