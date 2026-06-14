package main

import (
	"fmt"
	"os"

	"stackyrd/pkg/terminal"
)

func main() {
	g, _ := terminal.GuardWithSignal()
	defer g.Restore()
	defer g.HandlePanic()

	fmt.Print("\x1b[?1049h")
	defer fmt.Print("\x1b[?1049l")

	fmt.Print("\x1b[2J\x1b[H")
	fmt.Println("=== Raw Mode TUI Demo ===")
	fmt.Println("Press any key to see its code.")
	fmt.Println("Press 'q' or Ctrl+C to exit.")
	fmt.Println()

	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		b := buf[0]
		switch {
		case b == 'q':
			fmt.Println("\nExiting...")
			return
		case b == 0x03:
			fmt.Println("\nCtrl+C detected. Exiting...")
			return
		case b == 0x1b:
			fmt.Print("ESC")
		case b == 0x0d:
			fmt.Print("ENTER")
		case b == 0x7f || b == 0x08:
			fmt.Print("BACKSPACE")
		case b == 0x09:
			fmt.Print("TAB")
		case b >= 0x20 && b <= 0x7e:
			fmt.Printf("'%c'", b)
		default:
			fmt.Printf("0x%02x", b)
		}
		fmt.Printf(" (0x%02x)\n", b)
	}
}
