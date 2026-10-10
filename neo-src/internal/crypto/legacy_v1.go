// 本文件收纳 v1 主口令时代的密钥派生原语(从 encryption.go 移出,无 build
// tag)。SecretStorage v2 (secret.go) 走 keystore envelope,默认构建不再引用
// 这些函数;保留它们仅供两个消费方使用 ——
//   - keystore 迁移器(Task 8,解密 v1 "LTMSECv1" blob 用);
//   - keep_master_pwd 回退构建(保留旧口令生命周期时引用)。
//
// 迁移完成并跨过两个 release 后整体删除(spec §5)。同包内 encryption.go
// 仍提供裸 AES-GCM 与 Generate* 原语。

package crypto

import (
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

// PBKDF2 迭代次数(v1 口令路径专用)。
const PBKDF2Iterations = 100_000

// DeriveKey 使用 PBKDF2-HMAC-SHA256(password, salt, 100_000) 派生 32 字节密钥。
//
// Deprecated: v1 主口令派生路径。新代码一律经 keystore.Keystore.Seal/Unseal
// 用托管 master key 加解密;本函数仅供 keystore 迁移器与 keep_master_pwd
// 回退构建引用。
func DeriveKey(password, salt []byte) ([]byte, error) {
	if len(salt) != SaltSize {
		return nil, fmt.Errorf("%w: salt must be %d bytes, got %d",
			ErrInvalidKeyLength, SaltSize, len(salt))
	}
	return pbkdf2.Key(password, salt, PBKDF2Iterations, AES256GCMKeySize, sha256.New), nil
}

// 基于口令的辅助函数 —— DeriveKey + Encrypt/Decrypt 打包组合。
// Wire format:salt(16) || nonce(12) || ciphertext(N) || tag(16)。

// EncryptWithPassword 从 password+salt 派生 32 字节密钥,再加密明文。
// salt 在内部生成,调用方需持久化返回的 blob 以便后续恢复明文。
//
// Deprecated: v1 主口令加密路径(见文件头说明)。默认构建中 SecretStorage
// 已改用 keystore envelope v2;仅迁移器与 keep_master_pwd 回退构建引用。
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
//
// Deprecated: v1 主口令解密路径(见文件头说明)。新数据流请用
// keystore.Keystore.Unseal;本函数仅迁移器与 keep_master_pwd 回退构建引用。
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
