//go:build windows && integration_dpapi

// integration_dpapi_test.go —— windows-dpapi 后端的深层负例(真 syscall 面)。
//
// tag 定位与 Linux 侧 integration_dbus 相同:不进纯 `windows` tag 的测试面,
// 只由 CI test-windows job 的显式步骤执行:
//   go test -v -tags "windows integration_dpapi" ./internal/keystore/
// Linux 侧仅交叉编译验证(GOOS=windows go vet),全绿基线不受影响。
//
// Review focus #3:其他程序以不同 entropy 直接调 CryptUnprotectData 解
// 我们的 blob 必须失败 —— 应用 entropy 参与 blob 的 HMAC,不符则认证
// 必然不过。本测试绕过 Unseal,直接经 keystore_windows.go 的包级 proc
// (procCryptUnprotectData)与 blob helper(dataBlobOf/copyOutBlob)以
// entropy "other-app" 解 windowsStore.Seal 的产物。
//
// 负例自带正向对照(TestUnseal_CorrectEntropyDirectSuccess):错 entropy
// 负例只断言"解不开",若回归把 Seal/Unseal 两侧的 entropy 参数一起丢掉
// (pOptionalEntropy=NULL),blob 变成与 entropy 无关,负例依旧 ret==0 而
// 保绿,锁定失效。对照测试用逐字 entropy 直化解同一 Seal 产物,必须成功
// 且解回原文 —— 由此把"Seal 产物恰好绑定该 entropy 串"钉成运行时事实,
// 也让负例的 ret==0 不再可能因无关原因假绿。
//
// 凭据明文与 DPAPI blob 内容永不写日志:失败信息只含长度与 HRESULT。
//
// TestUnseal_CrossAccountRejectedManual —— 跨账户不可解的事实(review
// focus #1;DPAPI 用户作用域主密钥随用户档案由 OS 托管)不做自动测试:
// CI runner 没有第二个可登录账户。登记为文档化验收项,待 Task 7 搬运进
// 验收清单。人工步骤:
//  1. 同一台 Windows 机器上建两个本地标准账户 A 与 B;
//  2. 登录 A,用 windows-dpapi 后端 Seal 任意测试 payload,把 blob 落盘
//     (临时程序或应用凭据路径均可);
//  3. 登出并切换登录 B,让 B 对同一份 blob 走 Unseal(或带正确 entropy
//     直接调 CryptUnprotectData);
//  验收预期:步骤 3 必须失败且 errors.Is(err, ErrUnseal) 可匹配(DPAPI
//  返回解密/损坏类错误),B 永远得不到 A 的明文。属外部安全边界事实,
//  无须代码改动,只验行为。

package keystore

import (
	"bytes"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestUnseal_CorrectEntropyDirectSuccess 是错 entropy 负例的正向对照:
// 经 windowsStore.Seal 产 blob 后,绕过 Unseal 直接用逐字应用 entropy
// (本测试内自行构造的 []byte,不依赖任何 helper 内部实现)调
// CryptUnprotectData,必须成功(ret != 0)且解回的明文与 Seal 的输入逐
// 字节相等。这钉住"Seal 产物恰好绑定这条 entropy 串":若两侧 entropy
// 参数被一起删掉,此处会因"带 entropy 解不带 entropy 的 blob"而失败,
// 对照测试立刻变红。
func TestUnseal_CorrectEntropyDirectSuccess(t *testing.T) {
	ks := windowsStore{}

	plaintext := []byte("little-timer 正确 entropy 对照 payload\x00")
	blob, err := ks.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal 失败: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("Seal 返回空 blob")
	}

	// 参数序与 windowsStore.Unseal 一致;pOptionalEntropy 用逐字常量串,
	// 在本测试内自行 []byte 化,不复用 Unseal/Seal 的内部构造。
	entropy := []byte("little-timer/secret-envelope-v1")
	in := dataBlobOf(blob)
	ent := dataBlobOf(entropy)
	var out windows.DataBlob
	ret, _, callErr := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		uintptr(unsafe.Pointer(&ent)),
		0,
		0,
		uintptr(flagUiForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	// uintptr 化的指针不被 GC 追踪,钉住至调用返回。
	runtime.KeepAlive([]any{blob, entropy, &in, &ent, &out})

	if ret == 0 {
		t.Fatalf("正确 entropy 直解失败:blob %d 字节、entropy %d 字节, HRESULT: %v"+
			" —— Seal 产物未绑定预期 entropy", len(blob), len(entropy), callErr)
	}
	// copyOutBlob 拷回后立即 LocalFree,不泄漏本地堆。
	got := copyOutBlob(&out)
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("直解产出与 Seal 输入不恒等:明文长度 %d, 解出长度 %d",
			len(plaintext), len(got))
	}
	t.Logf("正确 entropy 直解成功(对照):blob %d 字节解出 %d 字节", len(blob), len(got))
}

// TestUnseal_WrongEntropyRejected 绕过 Unseal 直接 syscall:以逐字
// "other-app" 作 pOptionalEntropy 解 Seal 产物,ret 必须为 0(失败)。
// 若意外解得开,说明我们的 entropy 没参与认证 —— 正是 review focus #3
// 要防的回归。"解不开"这一事实由上方对照测试保证为"因 entropy 不符而
// 解不开",而非无关失败。
func TestUnseal_WrongEntropyRejected(t *testing.T) {
	ks := windowsStore{}

	plaintext := []byte("little-timer 错 entropy 负例 payload")
	blob, err := ks.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal 失败: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("Seal 返回空 blob")
	}

	// 参数序与 windowsStore.Unseal 一致,仅 pOptionalEntropy 换成
	// "other-app"(模拟其他程序自带的应用域熵)。
	in := dataBlobOf(blob)
	other := dataBlobOf([]byte("other-app"))
	var out windows.DataBlob
	ret, _, callErr := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		uintptr(unsafe.Pointer(&other)),
		0,
		0,
		uintptr(flagUiForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	// uintptr 化的指针不被 GC 追踪,钉住至调用返回。
	runtime.KeepAlive([]any{blob, &in, &other, &out})

	if ret != 0 {
		// 失败路径之外的唯一产出归还:先拷回再 LocalFree,不泄漏本地堆。
		got := copyOutBlob(&out)
		t.Fatalf("错 entropy 意外解成功:%d 字节 blob 解出 %d 字节 —— 应用 entropy 未参与认证",
			len(blob), len(got))
	}
	t.Logf("错 entropy 被拒(预期):blob %d 字节解密失败, HRESULT: %v", len(blob), callErr)
}
