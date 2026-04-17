package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/chenzanhong/zlog"
)

// Encrypt 使用 AES-256-GCM 加密字符串
func Encrypt(plaintext string, key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("encryption key is empty")
	}

	// 使用 AES-256-GCM 加密
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal 方法会加密并附加认证标签
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// 返回 base64 编码的密文
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 使用 AES-256-GCM 解密字符串
func Decrypt(ciphertext string, key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("decryption key is empty")
	}

	// 解码 base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// 检查密钥长度是否为 32 字节
	if len(key) < 32 {
		zlog.Warnw("key length is not 32 bytes, padding with zeros")
		// 在末尾不足 32 字节时填充 0
		key = key + strings.Repeat("0", 32-len(key))
	} else if len(key) > 32 {
		// 截取前 32 字节
		key = key[:32]
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, data := data[:nonceSize], data[nonceSize:]

	// Open 方法会解密并验证完整性
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (可能密钥错误或数据被篡改): %w", err)
	}

	return string(plaintext), nil
}
