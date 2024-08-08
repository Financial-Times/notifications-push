package hash

import (
	"crypto/md5"
	"encoding/hex"
)

// ComputeMD5Hash takes a string and returns its MD5 hash as a hexadecimal string
func ComputeMD5Hash(input string) string {
	hash := md5.New()
	hash.Write([]byte(input))
	hashBytes := hash.Sum(nil)
	return hex.EncodeToString(hashBytes)
}
