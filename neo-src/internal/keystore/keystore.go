// Package keystore 封装平台原生密钥托管的统一抽象。
//
// 每个平台提供一个 Keystore 实现,由 build tag 选择:
//   - keystore_linux.go(tag `linux`):Freedesktop Secret Service(D-Bus)
//   - keystore_android.go(tag `android`):Android Keystore,经 JNI 调用,
//     硬件 key 不可导出
//   - keystore_other.go(tag `!linux && !android`):占位实现,全部操作返回
//     ErrUnavailable(Windows/macOS 见 issue #13 后续)
//
// 平台实现文件在 init() 中把自身实例注册给 Current();调用方只与
// Keystore 接口交互,不感知后端差异。
package keystore

// KeystoreError 是类型化哨兵错误,可通过 errors.Is / errors.As 进行匹配。
type KeystoreError string

const (
	// ErrNotFound:master key item 不存在(尚未生成或已被删除)。
	ErrNotFound KeystoreError = "master key not found"

	// ErrUnseal:解密 / 认证失败(blob 损坏或 master key 已更换)。
	ErrUnseal KeystoreError = "unseal failed"

	// ErrUnavailable:平台密钥托管通道不可用(keyring daemon 未运行、
	// collection 锁定、JNI 断连,或本平台尚未实现)。
	ErrUnavailable KeystoreError = "keystore unavailable"
)

func (e KeystoreError) Error() string { return string(e) }

// Keystore —— 平台原生密钥托管。Seal/Unseal 是抽象边界:
// Linux 在 Go 内用取回的 master key 做 AES-GCM;Android 的 key
// 不可导出,加解密发生在硬件侧(JNI Cipher)。
type Keystore interface {
	// Name 返回后端标识,用于 UI/日志: "secret-service" | "android-keystore"。
	Name() string

	// Available 检查平台通道就绪(Linux: session bus 上存在
	// org.freedesktop.secrets;Android: JNI bridge 与 key 可访问)。
	// 仅用于启动期诊断与错误信息,不缓存结果。
	Available() error

	// EnsureMasterKey 幂等建立 item "little_timer/master_key":
	// 不存在则生成 32B 随机 key 并托管。启动期调用一次。
	EnsureMasterKey() error

	// HasMasterKey 报告 master key item 是否已存在。
	// 用于区分"新装用户"与"老数据待迁移"。
	HasMasterKey() (bool, error)

	// Seal 用 master key 加密明文,返回 nonce(12)||ct||tag(16)。
	Seal(plaintext []byte) ([]byte, error)

	// Unseal 解密 Seal 的产物;认证失败返回 ErrUnseal。
	Unseal(blob []byte) ([]byte, error)

	// Delete 移除 master key item(幂等)。供 Reset 与迁移收尾使用。
	DeleteMasterKey() error
}

// currentPlatform 由各平台实现文件(带各自 build tag)在 init() 中注册。
// linux / android 实现在后续任务落地,注册前为 nil。
var currentPlatform Keystore

// Current 按 build tag 返回当前构建目标的平台实现。
func Current() Keystore { return currentPlatform }
