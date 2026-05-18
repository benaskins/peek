package main

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

func blockID(headingPath []string, text string) string {
	h := sha1.New()
	for _, hd := range headingPath {
		h.Write([]byte(normalize(hd)))
		h.Write([]byte{0x1f})
	}
	h.Write([]byte{0x1e})
	h.Write([]byte(normalize(text)))
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
