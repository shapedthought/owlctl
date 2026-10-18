package utils

import "bytes"

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// StripUTF8BOM returns b without a leading UTF-8 byte order mark.
// Input without a BOM is returned unchanged.
func StripUTF8BOM(b []byte) []byte {
	return bytes.TrimPrefix(b, utf8BOM)
}
