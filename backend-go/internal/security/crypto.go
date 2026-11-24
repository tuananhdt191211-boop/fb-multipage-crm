package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

func deriveKey(masterSecret string) []byte {
	hash := sha256.Sum256([]byte(masterSecret))
	return hash[:]
}

// EncryptSecret mã hóa chuỗi nhạy cảm (như Page Access Token) bằng AES-256-GCM
func EncryptSecret(plainText, masterSecret string) (string, error) {
	if plainText == "" {
		return "", nil
	}
	block, err := aes.NewCipher(deriveKey(masterSecret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	cipherText := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

// DecryptSecret giải mã chuỗi đã mã hóa AES-256-GCM
func DecryptSecret(encodedCipher, masterSecret string) (string, error) {
	if encodedCipher == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(encodedCipher)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(deriveKey(masterSecret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, cipherText := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
