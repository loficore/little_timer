//go:build test_keystore

package keystore

import (
	"errors"
	"testing"
)

// TestMemKeystore_RoundTrip 内存 keystore 的基本契约：Seal/Unseal 往返。
func TestMemKeystore_RoundTrip(t *testing.T) {
	ks := Current()
	if ks == nil {
		t.Fatal("Current() = nil")
	}
	if ks.Name() != "test-memory" {
		t.Errorf("Name() = %q, want test-memory", ks.Name())
	}
	if err := ks.Available(); err != nil {
		t.Fatalf("Available() = %v, want nil（内存实现无外部通道）", err)
	}

	// 未建立 master key 时 Seal 报 ErrNotFound
	if _, err := ks.Seal([]byte("x")); !errors.Is(err, ErrNotFound) {
		t.Errorf("未建 key 时 Seal err = %v, want ErrNotFound", err)
	}

	if err := ks.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey: %v", err)
	}
	has, err := ks.HasMasterKey()
	if err != nil || !has {
		t.Fatalf("HasMasterKey = %v, %v; want true", has, err)
	}

	secret := []byte("sk-legacy-key")
	blob, err := ks.Seal(secret)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	// 关键安全属性：明文绝不能出现在密文里。
	if string(blob) == string(secret) || contains(blob, secret) {
		t.Error("Seal 产物包含明文，内存 keystore 泄露")
	}
	got, err := ks.Unseal(blob)
	if err != nil || string(got) != string(secret) {
		t.Errorf("Unseal = %q, %v; want %q", got, err, secret)
	}

	// 篡改 1 字节 → ErrUnseal
	bad := append([]byte(nil), blob...)
	bad[len(bad)-1] ^= 0xff
	if _, err := ks.Unseal(bad); !errors.Is(err, ErrUnseal) {
		t.Errorf("篡改后 Unseal err = %v, want ErrUnseal", err)
	}

	// 跨实例不可解：模拟"换进程/换 keyring"
	other := newMemKeystore()
	_ = other.EnsureMasterKey()
	if _, err := other.Unseal(blob); !errors.Is(err, ErrUnseal) {
		t.Errorf("别的实例 Unseal err = %v, want ErrUnseal", err)
	}
}

// TestMemKeystore_DeleteMasterKey 删除后既有密文不可解，且回到未建立态。
func TestMemKeystore_DeleteMasterKey(t *testing.T) {
	ks := newMemKeystore()
	if err := ks.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey: %v", err)
	}
	blob, err := ks.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if err := ks.DeleteMasterKey(); err != nil {
		t.Fatalf("DeleteMasterKey: %v", err)
	}
	if _, err := ks.Unseal(blob); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除 key 后 Unseal err = %v, want ErrNotFound", err)
	}
	has, _ := ks.HasMasterKey()
	if has {
		t.Error("删除后 HasMasterKey 仍为 true")
	}
}

func contains(haystack, needle []byte) bool {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
