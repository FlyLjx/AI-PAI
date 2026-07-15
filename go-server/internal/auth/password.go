package auth

import (
	"crypto/pbkdf2"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const keyLength = 64

func VerifyPassword(password string, passwordHash string) bool {
	if strings.HasPrefix(passwordHash, "$2a$") ||
		strings.HasPrefix(passwordHash, "$2b$") ||
		strings.HasPrefix(passwordHash, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil
	}
	ok, err := verifyPBKDF2(password, passwordHash)
	return err == nil && ok
}

func HashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hash)
}

func verifyPBKDF2(password string, passwordHash string) (bool, error) {
	parts := strings.Split(passwordHash, ":")
	if len(parts) != 3 {
		return false, errors.New("invalid password hash")
	}
	iterations, err := strconv.Atoi(parts[0])
	if err != nil || iterations <= 0 {
		return false, errors.New("invalid iterations")
	}
	salt := []byte(parts[1])
	stored, err := hex.DecodeString(parts[2])
	if err != nil {
		return false, err
	}
	hash, err := pbkdf2.Key(sha512.New, password, salt, iterations, keyLength)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(stored, hash) == 1, nil
}
