package flickr

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

type Cipher struct{ aead cipher.AEAD }

func NewCipher(encodedKey string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("FLICKR_TOKEN_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(value string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, []byte(value), nil), nil
}

func (c *Cipher) Decrypt(value []byte) (string, error) {
	if len(value) < c.aead.NonceSize() {
		return "", fmt.Errorf("encrypted token is truncated")
	}
	plain, err := c.aead.Open(nil, value[:c.aead.NonceSize()], value[c.aead.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt Flickr token: %w", err)
	}
	return string(plain), nil
}
