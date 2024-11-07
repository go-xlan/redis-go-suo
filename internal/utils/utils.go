package utils

import (
	"encoding/hex"

	"github.com/google/uuid"
)

func NewUUID() string {
	uux := uuid.New()
	return hex.EncodeToString(uux[:])
}
