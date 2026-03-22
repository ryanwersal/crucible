package ui

import (
	"os"
	"syscall"
	"unsafe"
)

// IsTerminal reports whether the given file is connected to a terminal.
func IsTerminal(f *os.File) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TIOCGETA,
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0,
	)
	return err == 0
}

// terminalWidth returns the width of the terminal attached to f, or 80 as fallback.
func terminalWidth(f *os.File) int {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	var ws winsize
	_, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(&ws)),
		0, 0, 0,
	)
	if err != 0 || ws.Col == 0 {
		return 80
	}
	return int(ws.Col)
}
