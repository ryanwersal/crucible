package script

import "bytes"

// stripShebang blanks out a shebang line (#!) at the start of source,
// replacing its contents with spaces while preserving the trailing newline.
// This keeps line numbers intact for error messages. If no shebang is present,
// the original slice is returned unchanged (zero allocation).
func stripShebang(source []byte) []byte {
	if !bytes.HasPrefix(source, []byte("#!")) {
		return source
	}

	idx := bytes.IndexByte(source, '\n')
	if idx < 0 {
		// Shebang with no trailing newline — blank the entire content.
		out := make([]byte, len(source))
		for i := range out {
			out[i] = ' '
		}
		return out
	}

	// Copy before mutating to avoid modifying the caller's slice.
	out := make([]byte, len(source))
	copy(out, source)
	for i := range idx {
		out[i] = ' '
	}
	return out
}
