package entity

import (
	"encoding/hex"
	"errors"
	"strings"
)

// ParseID accepts the existing 24-character public identifier format.
func ParseID(value string) (string, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 12 {
		return "", errors.New("invalid identifier")
	}
	return strings.ToLower(value), nil
}
