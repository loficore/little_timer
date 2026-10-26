//go:build windows && integration_dpapi

// dpapi_spike_test.go —— crypt32 DPAPI 在 CI 会话中可用性的 spike 测试(spec §6.2 待验证项)。
//
// 目的:证明 GitHub-hosted windows-latest runner 上的用户态进程能否调用
// CryptProtectData / CryptUnprotectData 完成往返。结论决定后续任务的正式
// Windows 后端(keystore_windows.go)能否建立在同一 syscall 模式上;若不可用,
// 按 spec §6.2 退路执行(job 保留、集成步骤改 workflow_dispatch 门)。
//
// 真实往返断言只在 Windows CI 上执行:
//   go test -v -tags "windows integration_dpapi" ./internal/keystore/ -run DpapiSpike
// Linux 侧仅做交叉编译验证(GOOS=windows go vet),该 tag 不参与 Linux 的
// go test ./...,全绿基线不受影响。
//
// spike 阶段 entropy 允许为 nil(brief 约束);全局约束里的
// CRYPTPROTECT_UI_FORBIDDEN 标志从本 spike 起就恒带。凭据明文与 DPAPI blob
// 内容永不写日志——下面仅打印长度。

package keystore

import (
	"bytes"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// dataBlob 是 crypt32 的 DATA_BLOB(cbData: DWORD + pbData: BYTE*)。
// 本地定义即可:Go 在 64 位下会在 Size 后自动补 4 字节对齐填充,
// 与 C 结构内存布局逐字节一致;32 位下指针恰为 4 字节,同样吻合。
type dataBlob struct {
	Size uint32
	Data *byte
}

// cryptprotectUIForbidden 即 CRYPTPROTECT_UI_FORBIDDEN:禁止 DPAPI 弹任何 UI,
// 无人值守 CI 会话的硬要求,也是正式后端恒带的标志(全局约束 W1 同源)。
const cryptprotectUIForbidden = 0x00000001

// TestDpapiSpike_RoundTrip 直接经 LazyDLL 调 crypt32 的加解密对,
// 断言 "spike" 原文经 DPAPI 往返后恒等。Linux 交叉编译只验语法/链接,
// 真实结果以 Windows CI 运行日志为准。
func TestDpapiSpike_RoundTrip(t *testing.T) {
	plaintext := []byte("spike")

	crypt32 := windows.NewLazySystemDLL("crypt32.dll")
	protect := crypt32.NewProc("CryptProtectData")
	unprotect := crypt32.NewProc("CryptUnprotectData")
	if err := protect.Find(); err != nil {
		t.Fatalf("crypt32.dll!CryptProtectData 解析失败: %v", err)
	}
	if err := unprotect.Find(); err != nil {
		t.Fatalf("crypt32.dll!CryptUnprotectData 解析失败: %v", err)
	}

	in := dataBlob{Size: uint32(len(plaintext)), Data: &plaintext[0]}
	var sealed dataBlob
	// 参数序:pDataIn, szDataDescr(NULL), pOptionalEntropy(spike 阶段 nil),
	// pvReserved, pPromptStruct, dwFlags, pDataOut。
	ret, _, err := protect.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		uintptr(cryptprotectUIForbidden),
		uintptr(unsafe.Pointer(&sealed)),
	)
	if ret == 0 {
		t.Fatalf("CryptProtectData 失败: %v —— DPAPI 在本会话不可用(spec §6.2 退路触发条件)", err)
	}
	// sealed.Data 是 DPAPI 在本地堆上分配的任务内存,用毕必须 LocalFree。
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(sealed.Data)))

	if sealed.Size == 0 || sealed.Data == nil {
		t.Fatalf("CryptProtectData 返回空 blob: Size=%d", sealed.Size)
	}
	t.Logf("CryptProtectData OK: 明文 %d 字节 → blob %d 字节", len(plaintext), sealed.Size)

	blob := unsafe.Slice(sealed.Data, sealed.Size)

	blobIn := dataBlob{Size: uint32(len(blob)), Data: &blob[0]}
	var opened dataBlob
	// 参数序:pDataIn, ppszDataDescr(不需要描述串,传 NULL), pOptionalEntropy,
	// pvReserved, pPromptStruct, dwFlags, pDataOut。
	ret, _, err = unprotect.Call(
		uintptr(unsafe.Pointer(&blobIn)),
		0,
		0,
		0,
		0,
		uintptr(cryptprotectUIForbidden),
		uintptr(unsafe.Pointer(&opened)),
	)
	if ret == 0 {
		t.Fatalf("CryptUnprotectData 失败: %v", err)
	}
	// opened.Data 同上,由 CryptUnprotectData 分配,必须 LocalFree。
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(opened.Data)))

	got := unsafe.Slice(opened.Data, opened.Size)
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("DPAPI 往返不恒等: 原文 %q 长度 %d, 解出长度 %d", plaintext, len(plaintext), len(got))
	}
	t.Logf("DPAPI 往返恒等,%d 字节 blob 解回 %d 字节明文 —— 本会话 DPAPI 可用", sealed.Size, len(got))
}
