// Package crypto —— 加密原语测试(encryption.go + legacy_v1.go)。
//
// 只测公开接口:往返一致性、篡改检测、错误密钥拒绝、密钥派生的确定性。
// v1 口令派生原语(DeriveKey / EncryptWithPassword / DecryptWithPassword)
// 的测试随函数迁移保留在此;SecretStorage v2 的行为测试见同目录
// secret_v2_test.go(外部测试包)。
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

// EncryptWithPassword / DecryptWithPassword。

func TestPasswordRoundTrip(t *testing.T) {
	plaintext := []byte("the cake is a lie")
	password := []byte("hunter2")

	blob, err := EncryptWithPassword(plaintext, password)
	if err != nil {
		t.Fatalf("EncryptWithPassword: %v", err)
	}
	got, err := DecryptWithPassword(blob, password)
	if err != nil {
		t.Fatalf("DecryptWithPassword: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch")
	}
}

func TestPasswordRoundTripWrongPassword(t *testing.T) {
	blob, err := EncryptWithPassword([]byte("x"), []byte("hunter2"))
	if err != nil {
		t.Fatalf("EncryptWithPassword: %v", err)
	}
	if _, err := DecryptWithPassword(blob, []byte("hunter3")); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("err = %v, want ErrAuthenticationFailed", err)
	}
}

// DeriveKey。

func TestDeriveKeyDeterministic(t *testing.T) {
	password := []byte("password")
	salt := GenerateSalt()

	k1, err := DeriveKey(password, salt)
	if err != nil {
		t.Fatalf("DeriveKey #1: %v", err)
	}
	k2, err := DeriveKey(password, salt)
	if err != nil {
		t.Fatalf("DeriveKey #2: %v", err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatalf("DeriveKey not deterministic: %x vs %x", k1, k2)
	}
	if len(k1) != AES256GCMKeySize {
		t.Fatalf("key length = %d, want %d", len(k1), AES256GCMKeySize)
	}
}

func TestDeriveKeyWrongSaltSize(t *testing.T) {
	if _, err := DeriveKey([]byte("x"), []byte("short")); !errors.Is(err, ErrInvalidKeyLength) {
		t.Fatalf("err = %v, want ErrInvalidKeyLength", err)
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
