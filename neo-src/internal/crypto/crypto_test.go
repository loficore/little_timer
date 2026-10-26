// Package crypto —— 加密原语测试(encryption.go)。
//
// 只测公开接口:往返一致性、篡改检测、错误密钥拒绝。
// v1 口令派生原语已随迁移链整体删除,对应测试一并移除;SecretStorage v2
// 的行为测试见同目录 secret_v2_test.go(外部测试包)。
package crypto

import (
	"bytes"
	"errors"
	"testing"
)

// Encrypt / Decrypt。

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := GenerateKey()
	nonce := GenerateNonce()
	plaintext := []byte("hello, little-timer world!")

	blob, err := Encrypt(plaintext, key, nonce)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if len(blob) != len(plaintext)+AES256GCMNonceSize+AES256GCMTagSize {
		t.Fatalf("blob length = %d, want %d",
			len(blob), len(plaintext)+AES256GCMNonceSize+AES256GCMTagSize)
	}
	if !bytes.Equal(blob[:AES256GCMNonceSize], nonce) {
		t.Fatalf("nonce not preserved as prefix")
	}

	got, err := Decrypt(blob, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: got %q want %q", got, plaintext)
	}
}

func TestEncryptRejectsWrongKeySize(t *testing.T) {
	_, err := Encrypt([]byte("x"), make([]byte, 16), GenerateNonce())
	if err == nil {
		t.Fatal("expected error for 16-byte key")
	}
	if !errors.Is(err, ErrInvalidKeyLength) {
		t.Fatalf("err = %v, want ErrInvalidKeyLength", err)
	}
}

func TestEncryptRejectsWrongNonceSize(t *testing.T) {
	_, err := Encrypt([]byte("x"), GenerateKey(), make([]byte, 8))
	if err == nil {
		t.Fatal("expected error for 8-byte nonce")
	}
	if !errors.Is(err, ErrInvalidNonceLength) {
		t.Fatalf("err = %v, want ErrInvalidNonceLength", err)
	}
}

func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	key := GenerateKey()
	blob, err := Encrypt([]byte("payload"), key, GenerateNonce())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	// 翻转 ciphertext 区域的一个字节（跳过 nonce 前缀）。
	blob[AES256GCMNonceSize] ^= 0x01
	_, err = Decrypt(blob, key)
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("err = %v, want ErrAuthenticationFailed", err)
	}
}

func TestDecryptRejectsWrongKey(t *testing.T) {
	blob, err := Encrypt([]byte("payload"), GenerateKey(), GenerateNonce())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	_, err = Decrypt(blob, GenerateKey())
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("err = %v, want ErrAuthenticationFailed", err)
	}
}

func TestDecryptRejectsShortBlob(t *testing.T) {
	_, err := Decrypt([]byte{0x01, 0x02}, GenerateKey())
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("err = %v, want ErrAuthenticationFailed", err)
	}
}

// 随机数辅助函数。

func TestGenerateKeyNonceSaltLengths(t *testing.T) {
	if l := len(GenerateKey()); l != AES256GCMKeySize {
		t.Fatalf("GenerateKey length = %d, want %d", l, AES256GCMKeySize)
	}
	if l := len(GenerateNonce()); l != AES256GCMNonceSize {
		t.Fatalf("GenerateNonce length = %d, want %d", l, AES256GCMNonceSize)
	}
	if l := len(GenerateSalt()); l != SaltSize {
		t.Fatalf("GenerateSalt length = %d, want %d", l, SaltSize)
	}
}

func TestGenerateProducesDistinctValues(t *testing.T) {
	a, b := GenerateKey(), GenerateKey()
	if bytes.Equal(a, b) {
		t.Fatal("two consecutive GenerateKey() calls returned identical bytes")
	}
}
