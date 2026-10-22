//go:build !linux && !android

// keystore_other_test.go —— other 占位实现的行为测试。
//
// 仅在非 linux/android 平台构建运行(在其他平台执行本测试;Linux 本机
// 因 build tag 跳过,仅断言包可编译)。

package keystore

import (
	"errors"
	"strings"
	"testing"
)

// TestOtherCurrentName 断言 Current() 返回的占位实例 Name() == "other"。
func TestOtherCurrentName(t *testing.T) {
	ks := Current()
	if ks == nil {
		t.Fatal("Current() = nil, want other placeholder")
	}
	if got := ks.Name(); got != "other" {
		t.Fatalf("Name() = %q, want %q", got, "other")
	}
}

// TestOtherAvailable 断言 Available() 返回 errors.Is(ErrUnavailable) 的错误,
// 且错误文案含 "Windows/macOS 见 issue #13"。
func TestOtherAvailable(t *testing.T) {
	err := Current().Available()
	if err == nil {
		t.Fatal("Available() = nil, want ErrUnavailable")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Available() err = %v, want errors.Is(ErrUnavailable)", err)
	}
	const wantSub = "Windows/macOS 见 issue #13"
	if !strings.Contains(err.Error(), wantSub) {
		t.Fatalf("err text = %q, want contains %q", err, wantSub)
	}
}

// TestOtherMethodsUnavailable 断言除 Name 外全部方法返回 ErrUnavailable。
func TestOtherMethodsUnavailable(t *testing.T) {
	ks := Current()

	if err := ks.EnsureMasterKey(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("EnsureMasterKey err = %v, want errors.Is(ErrUnavailable)", err)
	}
	if has, err := ks.HasMasterKey(); !errors.Is(err, ErrUnavailable) || has {
		t.Fatalf("HasMasterKey = (%v, %v), want (false, ErrUnavailable)", has, err)
	}
	if blob, err := ks.Seal([]byte("plaintext")); blob != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Seal = (%v, %v), want (nil, ErrUnavailable)", blob, err)
	}
	if plain, err := ks.Unseal([]byte("blob")); plain != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Unseal = (%v, %v), want (nil, ErrUnavailable)", plain, err)
	}
	if err := ks.DeleteMasterKey(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("DeleteMasterKey err = %v, want errors.Is(ErrUnavailable)", err)
	}
}
