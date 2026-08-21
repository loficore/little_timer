// Package crypto 封装对称加密辅助函数，用于静态加密的 secret，
// 基于标准库 `crypto/aes` + `crypto/cipher` GCM 和
// `golang.org/x/crypto/pbkdf2`。
//
// Wire format（EncryptWithPassword 产出的加密 blob）：
//
//	salt (16) || nonce (12) || ciphertext (N) || gcm_tag (16)
//
// API：
//
//   - Encrypt(plaintext, key, nonce) / Decrypt(blob, key) —— 裸 AES-GCM
//     往返。Encrypt 的输出为 nonce(12) || ciphertext || tag(16)。
//   - EncryptWithPassword(plaintext, password) / DecryptWithPassword —— 追加
//     PBKDF2-HMAC-SHA256 密钥派生，使用 16 字节 salt。
//   - DeriveKey(password, salt) → 32 字节密钥（为测试而暴露）。
//   - GenerateKey / GenerateNonce / GenerateSalt —— 随机字节生成。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// AES-GCM / PBKDF2 尺寸常量。

const (
	AES256GCMKeySize   = 32
	AES256GCMNonceSize = 12
	AES256GCMTagSize   = 16
	SaltSize           = 16
	PBKDF2Iterations   = 100_000
)

// CryptoError 是类型化哨兵错误，可通过 errors.Is / errors.As 进行匹配。
type CryptoError string

const (
	ErrInvalidKeyLength     CryptoError = "invalid key length"
	ErrInvalidNonceLength   CryptoError = "invalid nonce length"
	ErrAuthenticationFailed CryptoError = "authentication failed"
	ErrOutOfMemory          CryptoError = "out of memory"
)

func (e CryptoError) Error() string { return string(e) }

// 随机数辅助函数。

// GenerateKey 返回一个新生成的 32 字节 AES-256 密钥。
func GenerateKey() []byte { return randomBytes(AES256GCMKeySize) }

// GenerateNonce 返回一个新生成的 12 字节 GCM nonce。
func GenerateNonce() []byte { return randomBytes(AES256GCMNonceSize) }

// GenerateSalt 返回一个新生成的 16 字节 PBKDF2 salt。
func GenerateSalt() []byte { return randomBytes(SaltSize) }

// randomBytes 从 crypto/rand 读取 n 字节；出错时（极其罕见 ——
// 仅当操作系统 RNG 故障）panic，把 RNG 失败视为致命错误。
func randomBytes(n int) []byte {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		panic(fmt.Errorf("crypto: read random bytes: %w", err))
	}
	return buf
}

// DeriveKey 使用 PBKDF2-HMAC-SHA256(password, salt, 100_000) 派生 32 字节密钥。
func DeriveKey(password, salt []byte) ([]byte, error) {
	if len(salt) != SaltSize {
		return nil, fmt.Errorf("%w: salt must be %d bytes, got %d",
			ErrInvalidKeyLength, SaltSize, len(salt))
	}
	return pbkdf2.Key(password, salt, PBKDF2Iterations, AES256GCMKeySize, sha256.New), nil
}

// 裸 AES-256-GCM（调用方提供 32 字节 key 和 12 字节 nonce）。
// Wire format：nonce || ciphertext || tag。Encrypt 的输出为
// len(plaintext)+NonceSize+TagSize 字节。

// Encrypt 使用 AES-256-GCM 与给定的 key/nonce 加密明文。返回的二进制数据
// 布局为 nonce || ciphertext || tag。
func Encrypt(plaintext, key, nonce []byte) ([]byte, error) {
	if len(key) != AES256GCMKeySize {
		return nil, fmt.Errorf("%w: key must be %d bytes, got %d",
			ErrInvalidKeyLength, AES256GCMKeySize, len(key))
	}
	if len(nonce) != AES256GCMNonceSize {
		return nil, fmt.Errorf("%w: nonce must be %d bytes, got %d",
			ErrInvalidNonceLength, AES256GCMNonceSize, len(nonce))
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	// Seal 把 ciphertext + tag 追加到第一个参数后面；这里预先把目标
	// 容量设为 nonce+ct+tag，使结果只需一次分配。
	out := make([]byte, 0, len(nonce)+len(plaintext)+AES256GCMTagSize)
	out = append(out, nonce...)
	sealed := gcm.Seal(out, nonce, plaintext, nil)
	return sealed, nil
}

// Decrypt 解密由 Encrypt 产生的二进制数据。当 GCM tag 校验失败时返回
// ErrAuthenticationFailed。
func Decrypt(blob, key []byte) ([]byte, error) {
	if len(key) != AES256GCMKeySize {
		return nil, fmt.Errorf("%w: key must be %d bytes, got %d",
			ErrInvalidKeyLength, AES256GCMKeySize, len(key))
	}
	if len(blob) < AES256GCMNonceSize+AES256GCMTagSize {
		return nil, fmt.Errorf("%w: blob too short (%d bytes)",
			ErrAuthenticationFailed, len(blob))
	}
	nonce := blob[:AES256GCMNonceSize]
	body := blob[AES256GCMNonceSize:]
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	pt, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuthenticationFailed, err)
	}
	return pt, nil
}

// 基于口令的辅助函数 —— DeriveKey + Encrypt/Decrypt 打包组合。
// Wire format：salt(16) || nonce(12) || ciphertext(N) || tag(16)。

// EncryptWithPassword 从 password+salt 派生 32 字节密钥，再加密明文。
// salt 在内部生成，调用方需持久化返回的 blob 以便后续恢复明文。
func EncryptWithPassword(plaintext, password []byte) ([]byte, error) {
	salt := GenerateSalt()
	key, err := DeriveKey(password, salt)
	if err != nil {
		return nil, err
	}
	nonce := GenerateNonce()
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	// Wire 布局：salt || nonce || sealed（sealed = ct || tag）
	out := make([]byte, 0, SaltSize+AES256GCMNonceSize+len(sealed))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

// DecryptWithPassword 解密由 EncryptWithPassword 产生的二进制数据。
func DecryptWithPassword(blob, password []byte) ([]byte, error) {
	if len(blob) < SaltSize+AES256GCMNonceSize+AES256GCMTagSize {
		return nil, fmt.Errorf("%w: blob too short (%d bytes)",
			ErrAuthenticationFailed, len(blob))
	}
	salt := blob[:SaltSize]
	nonce := blob[SaltSize : SaltSize+AES256GCMNonceSize]
	body := blob[SaltSize+AES256GCMNonceSize:]
	key, err := DeriveKey(password, salt)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	pt, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuthenticationFailed, err)
	}
	return pt, nil
}

// 内部辅助函数。

// newGCM 用给定的 32 字节 key 构造 AES-256-GCM 密码器。
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes.NewCipher: %w", err)
	}
	return cipher.NewGCM(block)
}
