package fishhist

import "bytes"

// escape encodes a value for the history file, mirroring fish's
// escape_yaml_fish_2_0. Order matters: backslashes are doubled first, then
// newlines become a backslash followed by a literal 'n'.
func escape(s []byte) []byte {
	if bytes.IndexByte(s, '\\') < 0 && bytes.IndexByte(s, '\n') < 0 {
		return s
	}
	out := make([]byte, 0, len(s)+8)
	for _, b := range s {
		switch b {
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		default:
			out = append(out, b)
		}
	}
	return out
}

// unescape decodes a value from the history file, mirroring fish's
// unescape_yaml_fish_2_0.
//
// Fish's decoder recognises exactly two escapes, `\\` and `\n`. On any other
// escape sequence -- including a backslash at the very end of the value -- it
// breaks out of its loop, discarding the remainder of the input. That
// truncation is faithfully reproduced here: diverging from it would make us
// re-serialize an entry differently than fish would, silently rewriting the
// user's history.
func unescape(s []byte) []byte {
	i := bytes.IndexByte(s, '\\')
	if i < 0 {
		return s
	}

	out := make([]byte, 0, len(s))
	out = append(out, s[:i]...)
	for i < len(s) {
		// s[i] is a backslash; decode the sequence it introduces.
		if i+1 >= len(s) {
			// Trailing lone backslash: fish stops here.
			return out
		}
		switch s[i+1] {
		case '\\':
			out = append(out, '\\')
		case 'n':
			out = append(out, '\n')
		default:
			// Unrecognised escape: fish discards the rest.
			return out
		}
		i += 2

		next := bytes.IndexByte(s[i:], '\\')
		if next < 0 {
			out = append(out, s[i:]...)
			return out
		}
		out = append(out, s[i:i+next]...)
		i += next
	}
	return out
}
