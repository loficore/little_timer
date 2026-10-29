//go:build windows

// keystore_windows_test.go —— windows-dpapi 后端的行为测试,跑在 CI 的
// `test-windows` job(`go test -tags windows ./internal/keystore/`)。
//
// 覆盖两面:
//   - no-op 面:master key 相关方法在 Windows 后端恒为成功(§4.3),
//     Available 在 CI 会话应探测通过;
//   - 往返面:Seal 产物经 Unseal 解回原文恒等(正例),以及 blob 被
//     篡改后的完整性负例(review focus #2):逐字节翻转断言"拒绝或
//     明文不变"(DPAPI 头部元数据不受外层 HMAC 覆盖,翻转后仍可安全
//     解回原文,详见 TestUnseal_TamperedBlob_NoSilentContentChange),
//     并对尾部密文/MAC 区单独断言必拒。错 entropy 负例依赖
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

// TestUnseal_TamperedBlob_NoSilentContentChange —— review focus #2 的真实
// 完整性不变量(替代早期"任意一字节翻转必拒"的错误前提,后者在真实
// Windows CI run 40306855467 上被证伪):
//
// DPAPI blob 头部(version dword + provider GUID,偏移约 0-19)是元数据,
// 不被外层 HMAC 覆盖;CURRENT_USER 作用域用默认 provider 解 blob 时
// CryptUnprotectData 忽略这些字节,翻转后照常解回完全相同的原文。因此
// 对 Seal 产物的每一个字节偏移,翻转(XOR 0xFF)后 Unseal 必须满足其一:
//
//	(a) 返回错误且 errors.Is(err, ErrUnseal)(篡改被 HMAC 检出而拒绝),或
//	(b) 解回的明文与 Seal 输入 bytes.Equal 恒等(惰性元数据,内容未变)。
//
// 成功且明文改变是唯一的真实违规(静默内容篡改),收集并列出全部违规
// 偏移。另断言至少存在一个 (a) 类拒绝:若所有偏移都走 (b) 直通,
// HMAC 完整性不可观测,测试必须变红(防止整段假绿)。
//
// 聚焦子断言:blob 末尾 16 字节(密文/MAC 尾区,不依赖头部精确布局)
// 的每个偏移翻转后必须得到 (a) 类拒绝 —— "密文区篡改必拒"。若未来 CI
// 显示尾区也能直通,那是真实的完整性红灯,保持断言让其暴露。
//
// 每个偏移翻转前 bytes.Clone,避免翻转叠加。失败信息只含偏移与长度,
// 不含明文与 blob 内容。
func TestUnseal_TamperedBlob_NoSilentContentChange(t *testing.T) {
	ks := windowsStore{}

	plaintext := []byte("little-timer 篡改负例 payload")
	blob, err := ks.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal 失败: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("Seal 返回空 blob")
	}

	var silentChanges []int
	rejections := 0
	for i := range blob {
		tampered := bytes.Clone(blob)
		tampered[i] ^= 0xFF
		got, err := ks.Unseal(tampered)
		switch {
		case err != nil:
			if !errors.Is(err, ErrUnseal) {
				t.Fatalf("偏移 %d 字节翻转的失败未透出 ErrUnseal 链: %v", i, err)
			}
			rejections++
		case bytes.Equal(got, plaintext):
			// 惰性元数据字节:篡改未被 HMAC 覆盖但内容原样解回,允许
		default:
			silentChanges = append(silentChanges, i)
		}
	}
	if len(silentChanges) > 0 {
		t.Fatalf("偏移 %v 字节翻转(XOR 0xFF)后成功解出与原文不同的明文(原文长度 %d)—— 静默内容篡改,完整性不变量被破坏",
			silentChanges, len(plaintext))
	}
	if rejections == 0 {
		t.Fatalf("blob 长度 %d 的全部偏移翻转均直通(0 次拒绝)—— HMAC 完整性不可观测", len(blob))
	}

	tailStart := len(blob) - 16
	if tailStart < 0 {
		tailStart = 0
	}
	var tailPass []int
	for i := tailStart; i < len(blob); i++ {
		tampered := bytes.Clone(blob)
		tampered[i] ^= 0xFF
		_, err := ks.Unseal(tampered)
		if err == nil {
			tailPass = append(tailPass, i)
			continue
		}
		if !errors.Is(err, ErrUnseal) {
			t.Fatalf("尾区偏移 %d 字节翻转的失败未透出 ErrUnseal 链: %v", i, err)
		}
	}
	if len(tailPass) > 0 {
		t.Fatalf("密文/MAC 尾区(末 %d 字节,偏移 %v 起,blob 长度 %d)中偏移 %v 翻转后竟被成功解出 —— 密文区篡改必拒",
			len(blob)-tailStart, tailStart, len(blob), tailPass)
	}
}
