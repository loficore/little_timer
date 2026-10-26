//go:build windows

// keystore_windows_test.go —— windows-dpapi 后端的行为测试,跑在 CI 的
// `test-windows` job(`go test -tags windows ./internal/keystore/`)。
//
// 覆盖两面:
//   - no-op 面:master key 相关方法在 Windows 后端恒为成功(§4.3),
//     Available 在 CI 会话应探测通过;
//   - 往返面:Seal 产物经 Unseal 解回原文恒等(正例),以及 blob 被
//     篡改任意一字节必拒(负例,review focus #2)。错 entropy 负例依赖
//     直接 syscall,单独放在 integration_dpapi tag
//     (见 integration_dpapi_test.go),不混入本文件的纯 windows 测试面。
// 断言常量逐字值,防止 entropy/标志被无意改动(review focus #3)。
// 凭据明文与 DPAPI blob 内容永不写日志:失败信息只含长度。

package keystore

import (
	"bytes"
	"errors"
	"testing"
)

// TestWindowsConstants 断言 entropy 与 UI 标志常量的逐字值。
func TestWindowsConstants(t *testing.T) {
	if dpapiEntropy != "little-timer/secret-envelope-v1" {
		t.Fatalf("dpapiEntropy = %q, want %q", dpapiEntropy, "little-timer/secret-envelope-v1")
	}
	if flagUiForbidden != 0x1 {
		t.Fatalf("flagUiForbidden = %#x, want 0x1", flagUiForbidden)
	}
}

// TestWindowsRegisteredAsCurrent 断言 init() 已把 windowsStore 注册给 Current()。
func TestWindowsRegisteredAsCurrent(t *testing.T) {
	ks := Current()
	if ks == nil {
		t.Fatal("Current() = nil, want windowsStore")
	}
	if ks.Name() != "windows-dpapi" {
		t.Fatalf("Current().Name() = %q, want %q", ks.Name(), "windows-dpapi")
	}
}

// TestWindowsNoOps 断言 master key 面在 Windows 后端为 no-op 语义,
// 且 Available 探测 crypt32 成功。
func TestWindowsNoOps(t *testing.T) {
	ks := windowsStore{}
	if err := ks.Available(); err != nil {
		t.Fatalf("Available() = %v, want nil", err)
	}
	if err := ks.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey() = %v, want nil", err)
	}
	if has, err := ks.HasMasterKey(); err != nil || !has {
		t.Fatalf("HasMasterKey() = (%v, %v), want (true, nil)", has, err)
	}
	if err := ks.DeleteMasterKey(); err != nil {
		t.Fatalf("DeleteMasterKey() = %v, want nil", err)
	}
}

// TestWindowsSealUnsealRoundTrip 对 windowsStore{}.Seal 的产物做 Unseal,
// 断言往返恒等(spec §4:DPAPI blob 自描述,同 entropy 才能解回)。
func TestWindowsSealUnsealRoundTrip(t *testing.T) {
	ks := windowsStore{}

	plaintext := []byte("little-timer windows 凭据\x00 payload")
	blob, err := ks.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal 失败: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("Seal 返回空 blob")
	}
	if bytes.Equal(blob, plaintext) {
		t.Fatal("Seal 产物与明文相同,未经加密")
	}

	got, err := ks.Unseal(blob)
	if err != nil {
		t.Fatalf("Unseal 失败(明文长度 %d, blob 长度 %d): %v", len(plaintext), len(blob), err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("往返不恒等: 明文长度 %d, 解出长度 %d", len(plaintext), len(got))
	}
}

// TestUnseal_TamperedBlob —— review focus #2:Seal 产物任意一字节
// XOR 0xFF 后 Unseal 必须失败,且错误链 errors.Is(err, ErrUnseal)
// 可匹配(DPAPI blob 的 HMAC 认证覆盖全文,上层据此拒绝写盘)。
// 逐字节全扫:任意位置的单字节翻转都必须被拒。失败信息只含偏移与
// 长度,不含 blob 内容。
func TestUnseal_TamperedBlob(t *testing.T) {
	ks := windowsStore{}

	blob, err := ks.Seal([]byte("little-timer 篡改负例 payload"))
	if err != nil {
		t.Fatalf("Seal 失败: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("Seal 返回空 blob")
	}

	for i := range blob {
		tampered := bytes.Clone(blob)
		tampered[i] ^= 0xFF
		got, err := ks.Unseal(tampered)
		if err == nil {
			t.Fatalf("偏移 %d 字节翻转(XOR 0xFF)竟被解出 %d 字节 —— blob 未受篡改保护", i, len(got))
		}
		if !errors.Is(err, ErrUnseal) {
			t.Fatalf("偏移 %d 字节翻转的失败未透出 ErrUnseal 链: %v", i, err)
		}
	}
}
