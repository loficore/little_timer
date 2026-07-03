// Package crypto — tests for the encryption + secret-storage helpers.
//
// We test the public surface: round-trip equality, tamper detection,
// wrong-key rejection, key-derivation determinism, and the lockout /
// persistence behaviour of SecretStorage.
package crypto

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// -----------------------------------------------------------------------------
// Encrypt / Decrypt.
// -----------------------------------------------------------------------------

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
	// Flip a byte in the ciphertext region (skip the nonce prefix).
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

// -----------------------------------------------------------------------------
// EncryptWithPassword / DecryptWithPassword.
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// DeriveKey.
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Random helpers.
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// SecretStorage.
// -----------------------------------------------------------------------------

func TestSecretStorageSetStoreRetrieveUnlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.enc")
	store := New(path)

	if store.HasMasterPassword() {
		t.Fatal("expected HasMasterPassword=false on fresh store")
	}
	if !store.IsLocked() {
		t.Fatal("expected IsLocked=true on fresh store")
	}

	if err := store.SetMasterPassword([]byte("master-pw")); err != nil {
		t.Fatalf("SetMasterPassword: %v", err)
	}
	if !store.HasMasterPassword() {
		t.Fatal("expected HasMasterPassword=true after SetMasterPassword")
	}
	if store.IsLocked() {
		t.Fatal("expected IsLocked=false after SetMasterPassword")
	}

	if err := store.Store([]byte("api_key"), []byte("deadbeef")); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := store.Retrieve([]byte("api_key"))
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if string(got) != "deadbeef" {
		t.Fatalf("Retrieve = %q, want %q", got, "deadbeef")
	}

	// Round-trip via Unlock on a fresh instance.
	store2 := New(path)
	if err := store2.Unlock([]byte("master-pw")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	got2, err := store2.Retrieve([]byte("api_key"))
	if err != nil {
		t.Fatalf("Retrieve after Unlock: %v", err)
	}
	if string(got2) != "deadbeef" {
		t.Fatalf("Retrieve after Unlock = %q, want %q", got2, "deadbeef")
	}
}

func TestSecretStorageWrongPasswordDoesNotUnlock(t *testing.T) {
	dir := t.TempDir()
	store := New(filepath.Join(dir, "secrets.enc"))
	if err := store.SetMasterPassword([]byte("right")); err != nil {
		t.Fatalf("SetMasterPassword: %v", err)
	}
	if err := store.Store([]byte("k"), []byte("v")); err != nil {
		t.Fatalf("Store: %v", err)
	}

	store2 := New(filepath.Join(dir, "secrets.enc"))
	if err := store2.Unlock([]byte("wrong")); err == nil {
		t.Fatal("expected error for wrong password")
	} else if !errors.Is(err, ErrAuthenticationFailed) && !errors.Is(err, ErrSecretDecryptionFailed) {
		t.Fatalf("Unlock wrong pw = %v, want ErrAuthenticationFailed or ErrSecretDecryptionFailed", err)
	}
}

func TestSecretStorageLockoutAfterRepeatedFailures(t *testing.T) {
	dir := t.TempDir()
	store := New(filepath.Join(dir, "secrets.enc"))
	if err := store.SetMasterPassword([]byte("right")); err != nil {
		t.Fatalf("SetMasterPassword: %v", err)
	}

	for i := 0; i < MaxUnlockAttempts; i++ {
		_ = store.Unlock([]byte("wrong"))
	}
	if store.LockoutUntil() == 0 {
		t.Fatal("expected LockoutUntil > 0 after repeated failures")
	}
	if err := store.Unlock([]byte("right")); !errors.Is(err, ErrSecretLocked) {
		t.Fatalf("Unlock during lockout = %v, want ErrSecretLocked", err)
	}
}

func TestSecretStorageDeleteAndClear(t *testing.T) {
	dir := t.TempDir()
	store := New(filepath.Join(dir, "secrets.enc"))
	if err := store.SetMasterPassword([]byte("pw")); err != nil {
		t.Fatalf("SetMasterPassword: %v", err)
	}
	for _, k := range []string{"a", "b", "c"} {
		if err := store.Store([]byte(k), []byte("v-"+k)); err != nil {
			t.Fatalf("Store %s: %v", k, err)
		}
	}
	if err := store.Delete([]byte("b")); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Retrieve([]byte("b")); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("Retrieve after Delete = %v, want ErrSecretNotFound", err)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := store.Retrieve([]byte("a")); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("Retrieve after Clear = %v, want ErrSecretNotFound", err)
	}
}

func TestSecretStorageLockedStoreFails(t *testing.T) {
	dir := t.TempDir()
	store := New(filepath.Join(dir, "secrets.enc"))
	if err := store.Store([]byte("k"), []byte("v")); !errors.Is(err, ErrSecretLocked) {
		t.Fatalf("Store when locked = %v, want ErrSecretLocked", err)
	}
}

func TestSecretStorageInMemoryOnly(t *testing.T) {
	store := New("")
	if err := store.SetMasterPassword([]byte("pw")); err != nil {
		t.Fatalf("SetMasterPassword: %v", err)
	}
	if err := store.Store([]byte("k"), []byte("v")); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if store.HasMasterPassword() {
		t.Fatal("HasMasterPassword should be false for in-memory-only store")
	}
}

// Belt-and-braces: confirm the on-disk file has the magic prefix.
func TestSecretStorageDiskBlobHasMagic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.enc")
	store := New(path)
	if err := store.SetMasterPassword([]byte("pw")); err != nil {
		t.Fatalf("SetMasterPassword: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.HasPrefix(data, secretMagic) {
		t.Fatalf("on-disk blob missing magic prefix")
	}
}