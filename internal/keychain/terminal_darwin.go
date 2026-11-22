package keychain

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

func terminalInput(input io.Reader) bool {
	file, ok := input.(*os.File)
	if !ok {
		return false
	}
	var state syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, file.Fd(), syscall.TIOCGETA, uintptr(unsafe.Pointer(&state)), 0, 0, 0)
	return err == 0
}
