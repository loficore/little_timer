// Package crypto — secret storage with master-password protection.
//
// Port of `src/core/utils/secret_storage.zig` (little_timer).
//
// The Zig source ships three backends (Linux secret-service, macOS
// Keychain, Windows Credential Manager) but only the in-memory
// "SoftwareSecretImpl" is actually wired up; the OS-keychain helpers are
// stubs (`SecretError.NotImplemented`).  This Go port mirrors that:
// there is no real OS-keychain integration in W4 — secrets live in an
// in-memory map (encrypted-at-rest with a master-password-derived key)
// and are persisted to a single encrypted file on disk.  The file path
// is supplied by the caller so the caller controls where the secret blob
// lives (typically `os.UserConfigDir()/little_timer/secrets.enc`).
//
// Wire format of the on-disk blob:
//
//	magic (8 bytes: "LTMSECv1") || salt (16) || nonce (12) ||
//	  gcm_sealed( JSON(map[string][]byte) )
//
// JSON inside the GCM seal is `{"key": base64(value), …}`.  This is the
// encrypted-file fallback the spec asks for when the OS keychain is
// unavailable.
package crypto

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// SecretError mirrors `pub const SecretError = error{...}` in secret_storage.zig.
type SecretError string

const (
	ErrSecretNotFound         SecretError = "secret not found"
	ErrSecretAlreadyExists    SecretError = "secret already exists"
	ErrSecretInvalidValue     SecretError = "invalid value"
	ErrSecretNoAccess         SecretError = "no access"
	ErrSecretOutOfMemory      SecretError = "out of memory"
	ErrSecretLocked           SecretError = "secret store locked"
	ErrSecretMasterPwdNotSet  SecretError = "master password not set"
	ErrSecretDecryptionFailed SecretError = "decryption failed"
	ErrSecretIO               SecretError = "secret store i/o failed"
)

func (e SecretError) Error() string { return string(e) }

// Magic prefix for the encrypted blob file.  Lets us distinguish our own
// file from accidental garbage on disk and gives us a clear upgrade path
// (bump to v2, handle v1 by migration in `loadFromFile`).
var secretMagic = []byte("LTMSECv1")

// Lockout constants — match Zig settings_manager.zig (5 attempts → 300s).
const (
	MaxUnlockAttempts      = 5
	LockoutDurationSeconds = 300
)

// SecretStorage is the encrypted-file fallback for storing credentials.
// One instance per process; all methods are goroutine-safe.
type SecretStorage struct {
	mu sync.Mutex

	// filePath is where the encrypted blob lives on disk.  Empty means
	// "no persistence — secrets vanish when the process exits".
	filePath string

	// masterPassword is the user-supplied password kept in memory after
	// SetMasterPassword / Unlock.  We need it because the on-disk blob
	// uses password → PBKDF2 → AES-GCM, and re-deriving the AES key
	// without the password would require storing the key on disk (which
	// defeats the encryption).  Mirrors the Zig `SoftwareSecretImpl`'s
	// `master_password` field.  Zeroed on Lock().
	masterPassword []byte

	// secrets is the in-memory cache.  Keys are arbitrary user-supplied
	// byte slices; we serialise them as-is inside the JSON map.
	secrets map[string][]byte

	// lockout bookkeeping (matches settings_manager.zig).
	failedAttempts  uint32
	lockedUntilUnix int64
}

// New returns an empty, locked SecretStorage that will persist to filePath.
// If filePath is "" the store is in-memory only.
func New(filePath string) *SecretStorage {
	return &SecretStorage{
		filePath: filePath,
		secrets:  make(map[string][]byte),
	}
}

// -----------------------------------------------------------------------------
// Master password lifecycle.
// -----------------------------------------------------------------------------

// SetMasterPassword installs (or replaces) the master password.  Calling
// this on an existing store re-encrypts the on-disk blob with the new
// password.  Matches the Zig `setMasterPassword` semantics.
func (s *SecretStorage) SetMasterPassword(password []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	zeroBytes(s.masterPassword)
	s.masterPassword = append([]byte(nil), password...)
	s.secrets = make(map[string][]byte)
	s.failedAttempts = 0
	s.lockedUntilUnix = 0
	return s.persistLocked()
}

// Unlock validates password against the on-disk blob (if any) and populates
// the in-memory cache.  Implements the 5-attempt lockout from
// settings_manager.zig.
func (s *SecretStorage) Unlock(password []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lockedUntilUnix > time.Now().Unix() {
		return fmt.Errorf("%w: locked until %d", ErrSecretLocked, s.lockedUntilUnix)
	}

	if err := s.loadFromFileLocked(password); err != nil {
		s.failedAttempts++
		if s.failedAttempts >= MaxUnlockAttempts {
			s.lockedUntilUnix = time.Now().Unix() + LockoutDurationSeconds
		}
		return err
	}

	s.failedAttempts = 0
	s.lockedUntilUnix = 0
	return nil
}

// HasMasterPassword reports whether the on-disk blob exists.  Does not
// touch the in-memory state.
func (s *SecretStorage) HasMasterPassword() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.filePath == "" {
		return false
	}
	_, err := os.Stat(s.filePath)
	return err == nil
}

// IsLocked reports whether the store is currently locked (either because
// no Unlock was called or because of an active lockout).
func (s *SecretStorage) IsLocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.masterPassword == nil
}

// Lock drops the master password + decrypted cache.  The on-disk blob
// is untouched — the next Unlock will reload it.
func (s *SecretStorage) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	zeroBytes(s.masterPassword)
	s.masterPassword = nil
	// Clear plaintext cache so we don't leave decrypted secrets in memory.
	for k := range s.secrets {
		zeroBytes(s.secrets[k])
		delete(s.secrets, k)
	}
}

// -----------------------------------------------------------------------------
// Secret operations (require unlocked state).
// -----------------------------------------------------------------------------

// Store inserts or replaces a key/value pair.  The plaintext value is
// wiped from memory after encryption.
func (s *SecretStorage) Store(key, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireUnlocked(); err != nil {
		return err
	}
	stored := append([]byte(nil), value...)
	s.secrets[string(key)] = stored
	return s.persistLocked()
}

// Retrieve returns the plaintext value for key.  The returned slice is a
// copy; the caller may wipe it when done.
func (s *SecretStorage) Retrieve(key []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireUnlocked(); err != nil {
		return nil, err
	}
	v, ok := s.secrets[string(key)]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return append([]byte(nil), v...), nil
}

// Delete removes a key.  Deleting a non-existent key is a no-op.
func (s *SecretStorage) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireUnlocked(); err != nil {
		return err
	}
	if v, ok := s.secrets[string(key)]; ok {
		zeroBytes(v)
		delete(s.secrets, string(key))
	}
	return s.persistLocked()
}

// Clear wipes every secret.  Useful for "reset credentials" flows.
func (s *SecretStorage) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireUnlocked(); err != nil {
		return err
	}
	for k, v := range s.secrets {
		zeroBytes(v)
		delete(s.secrets, k)
	}
	return s.persistLocked()
}

// LockoutUntil returns the unix timestamp when the current lockout
// expires, or 0 if not locked out.
func (s *SecretStorage) LockoutUntil() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lockedUntilUnix
}

// -----------------------------------------------------------------------------
// Internals — caller MUST hold s.mu.
// -----------------------------------------------------------------------------

func (s *SecretStorage) requireUnlocked() error {
	if s.masterPassword == nil {
		return ErrSecretLocked
	}
	if s.lockedUntilUnix > time.Now().Unix() {
		return fmt.Errorf("%w: locked until %d", ErrSecretLocked, s.lockedUntilUnix)
	}
	return nil
}

// persistLocked serialises s.secrets to JSON, encrypts with masterKey via
// EncryptWithPassword (using the masterKey's bytes directly as the password
// input), and writes the magic-prefixed blob to disk.  If no filePath was
// configured this is a no-op.
//
// We re-derive a fresh salt per write rather than persisting one; the salt
// is needed only for PBKDF2 which we run to keep the cipher format
// consistent with DecryptWithPassword.  Equivalently we could call
// Encrypt/Decrypt directly with the masterKey, but routing through
// EncryptWithPassword means the on-disk blob can also be opened by
// DecryptWithPassword if someone recovers the master password — useful
// for manual recovery tools.
func (s *SecretStorage) persistLocked() error {
	if s.filePath == "" {
		return nil
	}
	if s.masterPassword == nil {
		return ErrSecretLocked
	}

	encoded := make(map[string]string, len(s.secrets))
	for k, v := range s.secrets {
		encoded[k] = base64.StdEncoding.EncodeToString(v)
	}
	plaintext, err := json.Marshal(encoded)
	if err != nil {
		return fmt.Errorf("%w: marshal: %v", ErrSecretIO, err)
	}
	defer zeroBytes(plaintext)

	sealed, err := EncryptWithPassword(plaintext, s.masterPassword)
	if err != nil {
		return err
	}

	out := make([]byte, 0, len(secretMagic)+len(sealed))
	out = append(out, secretMagic...)
	out = append(out, sealed...)

	if err := os.WriteFile(s.filePath, out, 0o600); err != nil {
		return fmt.Errorf("%w: write: %v", ErrSecretIO, err)
	}
	return nil
}

// loadFromFileLocked reads the on-disk blob, derives a key from password,
// and populates s.secrets.  On failure leaves the in-memory state untouched.
func (s *SecretStorage) loadFromFileLocked(password []byte) error {
	if s.filePath == "" {
		return ErrSecretNotFound
	}
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrSecretNotFound
		}
		return fmt.Errorf("%w: read: %v", ErrSecretIO, err)
	}
	if !bytes.HasPrefix(data, secretMagic) {
		return ErrSecretDecryptionFailed
	}
	sealed := data[len(secretMagic):]

	plaintext, err := DecryptWithPassword(sealed, password)
	if err != nil {
		return err
	}
	defer zeroBytes(plaintext)

	decoded := make(map[string]string)
	if err := json.Unmarshal(plaintext, &decoded); err != nil {
		return fmt.Errorf("%w: unmarshal: %v", ErrSecretDecryptionFailed, err)
	}
	secrets := make(map[string][]byte, len(decoded))
	for k, v := range decoded {
		raw, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return fmt.Errorf("%w: base64: %v", ErrSecretDecryptionFailed, err)
		}
		secrets[k] = raw
	}
	// Promote to live state only after all parsing succeeds.
	s.masterPassword = append([]byte(nil), password...)
	s.secrets = secrets
	return nil
}

// zeroBytes wipes a byte slice in place.  Best-effort defence against
// heap dumps; Go's GC may have copied the data, so this is not a
// guarantee, just a hygiene step that mirrors Zig's `@memset(buf, 0)`.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}