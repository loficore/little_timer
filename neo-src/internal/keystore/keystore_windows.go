//go:build windows

// keystore_windows.go —— windows 平台的 Keystore 实现:payload 直接用
// crypt32 DPAPI 的 CryptProtectData/CryptUnprotectData 加密(用户作用域),
// 无 cgo、无应用自管的 master key —— DPAPI 密钥由 OS 随登录用户档案托管与
// 派生,master key 相关方法在本后端为 no-op 语义(spec §4.3)。
//
// 应用 entropy 常量逐字传入两侧调用并参与 blob 的 HMAC,entropy 不符的
// 解密必然失败(spec §4.2);每次调用恒带 CRYPTPROTECT_UI_FORBIDDEN,
// 禁止 DPAPI 弹任何 UI(全局约束 W1)。
//
// 凭据明文与 DPAPI blob 内容永不写日志:失败路径只透出 HRESULT 文本与长度。

package keystore

import (
	"bytes"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// dpapiEntropy 是随 CryptProtectData/CryptUnprotectData 逐字传入的
// 应用 entropy,参与 blob HMAC;用不同 entropy 解密必然失败
// (spec §4.2, review focus #3)。
const dpapiEntropy = "little-timer/secret-envelope-v1"

// flagUiForbidden 即 CRYPTPROTECT_UI_FORBIDDEN:禁止 DPAPI 弹任何 UI,
// 所有调用恒带(全局约束 W1)。
const flagUiForbidden = 0x1

// crypt32.dll 与两个懒绑定过程入口:首次使用时才 Load/Find,无 init 期成本。
var (
	crypt32                = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
)

func init() { currentPlatform = windowsStore{} }

// windowsStore 是 DPAPI 后端的 Keystore 实现。无状态:master key 由 OS 在
// 用户档案侧托管,进程内不缓存任何密钥材料。
type windowsStore struct{}

// Name 返回 "windows-dpapi",供 UI/日志区分后端。
func (windowsStore) Name() string { return "windows-dpapi" }

// Available 仅探测 crypt32.dll 在当前会话能否加载(用户态 DPAPI 无独立
// daemon 可查)。失败包装 ErrUnavailable。
func (windowsStore) Available() error {
	if err := crypt32.Load(); err != nil {
		return fmt.Errorf("%w: crypt32.dll 加载失败: %v", ErrUnavailable, err)
	}
	return nil
}

// EnsureMasterKey 为 no-op:DPAPI 用户主密钥随用户档案由 OS 建立,
// 应用无法也无须生成(spec §4.3)。
func (windowsStore) EnsureMasterKey() error { return nil }

// HasMasterKey 恒 (true, nil):已登录会话的用户级 DPAPI 密钥始终存在。
func (windowsStore) HasMasterKey() (bool, error) { return true, nil }

// DeleteMasterKey 为 no-op:用户主密钥生命周期归 OS,应用不得触碰。
func (windowsStore) DeleteMasterKey() error { return nil }

// Seal 调 CryptProtectData 把明文加密成 DPAPI blob;proc 返回 0 一律
// 返回包装 ErrUnseal 的错误(只含 HRESULT 与长度,不含数据)。
func (windowsStore) Seal(plaintext []byte) ([]byte, error) {
	entropy := []byte(dpapiEntropy)
	in := dataBlobOf(plaintext)
	ent := dataBlobOf(entropy)
	var out windows.DataBlob

	// 七参顺序:pDataIn, szDataDescr=nil, pOptionalEntropy, pvReserved=nil,
	// pPromptStruct=nil, dwFlags, pDataOut。
	ret, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		uintptr(unsafe.Pointer(&ent)),
		0,
		0,
		uintptr(flagUiForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	// uintptr 化的指针参数不被 GC 追踪,显式钉住输入对象至调用返回。
	runtime.KeepAlive([]any{plaintext, entropy, &in, &ent, &out})
	if ret == 0 {
		return nil, fmt.Errorf("%w: CryptProtectData 失败(输入 %d 字节): %v",
			ErrUnseal, len(plaintext), err)
	}
	return copyOutBlob(&out), nil
}

// Unseal 调 CryptUnprotectData 把 Seal 产物解回明文;proc 返回 0(blob 被
// 篡改、entropy 不符、跨用户等)一律返回包装 ErrUnseal 的错误,绝不吞错
// (review focus #2 要求 errors.Is(err, ErrUnseal) 可匹配)。
func (windowsStore) Unseal(blob []byte) ([]byte, error) {
	entropy := []byte(dpapiEntropy)
	in := dataBlobOf(blob)
	ent := dataBlobOf(entropy)
	var out windows.DataBlob

	// 同序反向:pDataIn, ppszDataDescr=nil(不需要描述串,置 nil 也就没有
	// 需要 LocalFree 的描述串堆内存), pOptionalEntropy, pvReserved=nil,
	// pPromptStruct=nil, dwFlags, pDataOut。
	ret, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		uintptr(unsafe.Pointer(&ent)),
		0,
		0,
		uintptr(flagUiForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	runtime.KeepAlive([]any{blob, entropy, &in, &ent, &out})
	if ret == 0 {
		return nil, fmt.Errorf("%w: CryptUnprotectData 失败(输入 %d 字节): %v",
			ErrUnseal, len(blob), err)
	}
	return copyOutBlob(&out), nil
}

// dataBlobOf 把 Go slice 包装成 crypt32 DATA_BLOB;空输入包装为
// Size=0、指针 nil,DPAPI 两侧调用均接受。
func dataBlobOf(b []byte) windows.DataBlob {
	var p *byte
	if len(b) > 0 {
		p = &b[0]
	}
	return windows.DataBlob{Size: uint32(len(b)), Data: p}
}

// copyOutBlob 把 DPAPI 写进本地堆的 DATA_BLOB 拷回 Go slice 并立即
// LocalFree(用毕必须归还);Size=0 或 NULL 指针按空产出处理。
func copyOutBlob(b *windows.DataBlob) []byte {
	// b.Data 是 DPAPI 在本地堆分配的任务内存,用毕必须 LocalFree;
	// 参数在 defer 语句时求值,此时仍指向有效堆块。
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(b.Data)))
	if b.Size == 0 || b.Data == nil {
		return nil
	}
	return bytes.Clone(unsafe.Slice(b.Data, b.Size))
}
