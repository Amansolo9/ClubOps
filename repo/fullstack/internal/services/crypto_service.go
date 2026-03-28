package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
)

type CryptoService struct {
	gcm cipher.AEAD
}

func NewCryptoService() (*CryptoService, error) {
	keyMaterial := os.Getenv("APP_ENCRYPTION_KEY")
	if keyMaterial == "" {
		return nil, errors.New("APP_ENCRYPTION_KEY is required")
	}
	sum := sha256.Sum256([]byte(keyMaterial))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &CryptoService{gcm: gcm}, nil
}

func (s *CryptoService) Encrypt(plain string) string {
	if plain == "" {
		return ""
	}
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}
	ciphertext := s.gcm.Seal(nil, nonce, []byte(plain), nil)
	out := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(out)
}

func (s *CryptoService) Decrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	ns := s.gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("invalid encrypted payload")
	}
	plain, err := s.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
