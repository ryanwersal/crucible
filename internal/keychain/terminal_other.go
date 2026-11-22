//go:build !darwin

package keychain

import "io"

func terminalInput(io.Reader) bool { return false }
