//go:build linux

// keystore_linux_test.go —— Linux Secret Service 后端的纯逻辑单元测试。
//
// 只覆盖不依赖 D-Bus 的部分:blob 布局(sealBlob/unsealBlob)、item 属性
// 常量 itemAttrs、master key 校验 decodeMasterKey,以及 Current() 注册。
// D-Bus 协议链路见 integration_dbus_test.go(tag `linux && integration_dbus`)。

package keystore

import (
	"bytes"
	"errors"
	"testing"
)

// testKey32 是固定 32 字节测试密钥(仅测试用途,非真实密钥材料)。
var testKey32 = []byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
	0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
}

// TestBlobLayout 断言 Seal 产物布局为 nonce(12)||ct||tag(16),
// 往返 Unseal(Seal(x))==x,篡改/空 blob 返回 ErrUnseal。
func TestBlobLayout(t *testing.T) {
	const nonceSize = 12 // AES-256-GCM nonce
	const tagSize = 16   // GCM 认证 tag

	cases := []struct {
		name      string
		plaintext []byte
	}{
		{"ascii", []byte("hello keystore")},
		{"中文", []byte("密钥托管主密钥")},
		{"空明文", []byte{}},
		{"大块", bytes.Repeat([]byte{0xa5}, 4096)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blob, err := sealBlob(tc.plaintext, testKey32)
			if err != nil {
				t.Fatalf("sealBlob = %v, want nil", err)
			}
			if got, want := len(blob), nonceSize+len(tc.plaintext)+tagSize; got != want {
				t.Fatalf("blob 长度 = %d, want %d (nonce||ct||tag)", got, want)
			}
			plain, err := unsealBlob(blob, testKey32)
			if err != nil {
				t.Fatalf("unsealBlob = %v, want nil", err)
			}
			if !bytes.Equal(plain, tc.plaintext) {
				t.Fatalf("Unseal(Seal(x)) != x")
			}
		})
	}

	t.Run("篡改1字节", func(t *testing.T) {
		blob, err := sealBlob([]byte("tamper-me"), testKey32)
		if err != nil {
			t.Fatalf("sealBlob = %v", err)
		}
		blob[nonceSize] ^= 0x01
		if _, err := unsealBlob(blob, testKey32); !errors.Is(err, ErrUnseal) {
			t.Fatalf("篡改后 unsealBlob err = %v, want errors.Is(ErrUnseal)", err)
		}
	})

	t.Run("空blob", func(t *testing.T) {
		if _, err := unsealBlob([]byte{}, testKey32); !errors.Is(err, ErrUnseal) {
			t.Fatalf("空 blob unsealBlob err = %v, want errors.Is(ErrUnseal)", err)
		}
		if _, err := unsealBlob(nil, testKey32); !errors.Is(err, ErrUnseal) {
			t.Fatalf("nil blob unsealBlob err = %v, want errors.Is(ErrUnseal)", err)
		}
	})

	t.Run("错误key", func(t *testing.T) {
		blob, err := sealBlob([]byte("x"), testKey32)
		if err != nil {
			t.Fatalf("sealBlob = %v", err)
		}
		wrong := bytes.Clone(testKey32)
		wrong[0] ^= 0xff
		if _, err := unsealBlob(blob, wrong); !errors.Is(err, ErrUnseal) {
			t.Fatalf("错误 key unsealBlob err = %v, want errors.Is(ErrUnseal)", err)
		}
	})
}

// TestItemAttrs 逐字断言 master key item 的 D-Bus 属性常量。
func TestItemAttrs(t *testing.T) {
	want := map[string]string{"service": "application:little_timer", "key": "master_key"}
	if len(itemAttrs) != len(want) {
		t.Fatalf("itemAttrs 条目数 = %d, want %d", len(itemAttrs), len(want))
	}
	for k, v := range want {
		if got := itemAttrs[k]; got != v {
			t.Fatalf("itemAttrs[%q] = %q, want %q", k, got, v)
		}
	}
}

// TestDecodeMasterKey 断言 decodeMasterKey 对 len != 32 返回错误。
func TestDecodeMasterKey(t *testing.T) {
	cases := []struct {
		name    string
		raw     []byte
		wantErr bool
	}{
		{"空", []byte{}, true},
		{"nil", nil, true},
		{"31字节", make([]byte, 31), true},
		{"32字节", make([]byte, 32), false},
		{"33字节", make([]byte, 33), true},
		{"64字节", make([]byte, 64), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := decodeMasterKey(tc.raw)
			if tc.wantErr && err == nil {
				t.Fatalf("decodeMasterKey(%d 字节) = nil, want error", len(tc.raw))
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("decodeMasterKey(%d 字节) = %v, want nil", len(tc.raw), err)
			}
		})
	}
}

// TestLinuxCurrentName 断言 linux 构建下 Current() 注册了 secret-service 后端。
func TestLinuxCurrentName(t *testing.T) {
	ks := Current()
	if ks == nil {
		t.Fatal("Current() = nil, want linux keystore")
	}
	if got := ks.Name(); got != "secret-service" {
		t.Fatalf("Name() = %q, want %q", got, "secret-service")
	}
}
