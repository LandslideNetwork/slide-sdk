package saferand

import (
	"crypto/rand"
	"encoding/binary"
)

func CryptoRandFloat64() float64 {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		panic(err)
	}
	u := binary.BigEndian.Uint64(b[:])
	return float64(u>>11) / (1 << 53)
}

func CryptoRandInt(max int) int {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		panic(err)
	}
	return int(binary.LittleEndian.Uint64(b[:])) % max
}
