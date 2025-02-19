package utils

import (
	"encoding/hex"

	"github.com/google/uuid"
)

func NewUUID() string {
	newUUID := uuid.New()
	return hex.EncodeToString(newUUID[:])
}
