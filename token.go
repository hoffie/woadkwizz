package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
)

func generateToken() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("rand.Read failed: %s", err))
	}
	return hex.EncodeToString(b)
}
