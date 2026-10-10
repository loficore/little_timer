// keystore_android_core.go —— Android KeystoreBridge 后端的平台无关核心:
// androidBridge 传输接口、Keystore 实现、blob 布局校验与错误映射。
//
// 本文件刻意不带 build tag:全部逻辑只依赖接口,JNI 传输层
// (keystore_android.go,tag `android`)是 androidBridge 的一个薄适配器,
// 真机之外的构建(linux 单测、gomobile/wails 元数据生成)即可复用同一份
// 逻辑而无需链接 libaart JNI。包文档见 keystore.go。
//
// 安全约定:主密钥(material)永不离开 AndroidKeyStore;本包与传输层
// 只传递 Seal/Unseal 的入参与产物 blob(iv(12)||ct||tag),不落任何
// 密钥材料日志。

package keystore

import (
	"errors"
	"fmt"
	"strings"

	"little-timer/internal/crypto"
)

// androidKeystoreName 是本后端标识,与 brief 约定逐字一致。
const androidKeystoreName = "android-keystore"

// Java 侧(KeystoreBridge.java)跨 JNI 约定的错误消息标记。
// Java 抛出的 GeneralSecurityException 消息以这些标记开头,
// Go 端 mapAndroidError 据此映射为包内哨兵错误。
const (
	bridgeMarkKeyNotFound  = "KEY_NOT_FOUND"       // alias 不存在
	bridgeMarkUnsealFailed = "UNSEAL_FAILED"       // GCM 认证/解密失败
	bridgeMarkDisconnected = "BRIDGE_DISCONNECTED" // JNI 桥未注册/断连
)

// errBridgeDisconnected 由传输层在 bridge 未注册(JNI 断连)时返回。
var errBridgeDisconnected = errors.New("android keystore bridge 未注册(JNI 不可达)")

// keystoreBridgeGuidance —— Available 失败时的可执行中文指引
// (对齐 keystore_linux.go 的 unavailable 指引模式)。
const keystoreBridgeGuidance = "需要 Android KeystoreBridge 随 App 启动完成 JNI 注册" +
	"(WailsBridge.initialize → nativeKeystoreInit)。若提示持续出现,请确认 APK 内 " +
	"libwails.so 与本 App 版本匹配,必要时重装 App;主密钥托管在硬件 Keystore 中," +
	"卸载/清除数据将不可恢复地丢失已存凭据。"

// androidBridge 是到 Java KeystoreBridge 的最小传输接口。
// 生产实现:keystore_android.go 的 cgo JNI 适配器;测试实现:fake。
type androidBridge interface {
	// reachable 报告 JNI 桥是否已完成注册(不缓存副作用)。
	reachable() bool
	// ensureKey 幂等生成硬件主密钥;失败返回带标记的 error。
	ensureKey() error
	// hasKey 报告 alias 是否存在。
	hasKey() (bool, error)
	// seal/unseal 透传明文或 iv(12)||ct||tag blob。
	seal(plaintext []byte) ([]byte, error)
	unseal(blob []byte) ([]byte, error)
	// deleteKey 移除 alias(幂等)。
	deleteKey() error
}

// androidKeystore 是 Keystore 的 Android 后端;除传输细节外全部
// 逻辑在本类型上,由 keystore_android.go 的 init() 注册给 Current()。
type androidKeystore struct {
	bridge androidBridge
}

// newAndroidKeystoreWithBridge 用给定传输构造 android-keystore 后端。
// 真机构建经 newAndroidKeystore()(keystore_android.go)注入 JNI 适配器;
// linux 单测注入 fake。
func newAndroidKeystoreWithBridge(bridge androidBridge) Keystore {
	return &androidKeystore{bridge: bridge}
}

// Name 返回 "android-keystore",供 UI/日志区分后端。
func (*androidKeystore) Name() string { return androidKeystoreName }

// Available 检查 JNI 桥可达;断连时返回带中文指引的 ErrUnavailable。
func (k *androidKeystore) Available() error {
	if !k.connected() {
		return bridgeUnreachable()
	}
	return nil
}

// EnsureMasterKey 幂等建立硬件 alias "little_timer/master_key"
// (密钥生成与存放均在 AndroidKeyStore 内,永不导出)。
// 与 keystore_linux.go 同构:hasKey 已存在则直接返回,不再请求 Java
// 侧生成(Java 的 ensureKey 自身也幂等,这里是双重保险)。
func (k *androidKeystore) EnsureMasterKey() error {
	if !k.connected() {
		return bridgeUnreachable()
	}
	has, err := k.bridge.hasKey()
	if err != nil {
		return mapAndroidError(err)
	}
	if has {
		return nil
	}
	return mapAndroidError(k.bridge.ensureKey())
}

// HasMasterKey 报告硬件 alias 是否已存在。
func (k *androidKeystore) HasMasterKey() (bool, error) {
	if !k.connected() {
		return false, bridgeUnreachable()
	}
	has, err := k.bridge.hasKey()
	if err != nil {
		return false, mapAndroidError(err)
	}
	return has, nil
}

// Seal 请硬件侧加密,返回 iv(12)||ct||tag。明文不经 Go 侧加密,
// key 不出硬件。alias 缺失 → ErrNotFound;桥断连 → ErrUnavailable。
func (k *androidKeystore) Seal(plaintext []byte) ([]byte, error) {
	if !k.connected() {
		return nil, bridgeUnreachable()
	}
	blob, err := k.bridge.seal(plaintext)
	if err != nil {
		return nil, mapAndroidError(err)
	}
	if _, _, err := splitAndroidBlob(blob); err != nil {
		return nil, err
	}
	return blob, nil
}

// Unseal 解密 Seal 产物。先做本地布局校验(避免无谓 JNI 往返),
// 再交硬件侧 GCM 解密;认证失败 → ErrUnseal。
func (k *androidKeystore) Unseal(blob []byte) ([]byte, error) {
	if _, _, err := splitAndroidBlob(blob); err != nil {
		return nil, err
	}
	if !k.connected() {
		return nil, bridgeUnreachable()
	}
	plaintext, err := k.bridge.unseal(blob)
	if err != nil {
		return nil, mapAndroidError(err)
	}
	return plaintext, nil
}

// DeleteMasterKey 移除硬件 alias;alias 本不存在视为幂等成功。
func (k *androidKeystore) DeleteMasterKey() error {
	if !k.connected() {
		return bridgeUnreachable()
	}
	err := mapAndroidError(k.bridge.deleteKey())
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// connected 报告传输是否可达(桥未注册时所有操作短路)。
func (k *androidKeystore) connected() bool {
	return k.bridge != nil && k.bridge.reachable()
}

// bridgeUnreachable 统一把 JNI 断连包装为 ErrUnavailable + 中文指引
// (对齐 secret_service.go 的 unavailable 模式:原因在前、指引在后)。
func bridgeUnreachable() error {
	return fmt.Errorf("%w: %s; %s", ErrUnavailable, errBridgeDisconnected.Error(), keystoreBridgeGuidance)
}

// mapAndroidError 把传输层错误(Java 异常经 JNI 转成的带标记字符串)
// 映射到包内哨兵错误。已包装哨兵的错误原样透传,不重复套壳。
func mapAndroidError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnseal) || errors.Is(err, ErrUnavailable) {
		return err
	}
	msg := err.Error()
	switch {
	case errors.Is(err, errBridgeDisconnected), strings.Contains(msg, bridgeMarkDisconnected):
		return bridgeUnreachable()
	case strings.Contains(msg, bridgeMarkKeyNotFound):
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	case strings.Contains(msg, bridgeMarkUnsealFailed):
		return fmt.Errorf("%w: %v", ErrUnseal, err)
	default:
		// 未知 Java 异常/JNI 故障:通道本身已不可信,按不可用上报。
		return fmt.Errorf("%w: 桥接调用失败: %v", ErrUnavailable, err)
	}
}

// splitAndroidBlob 校验并拆分 Seal 产物:iv(12)||ct||tag。
// ct 可为空(空明文),故最小合法长度 = 12+16。布局与 crypto.Decrypt
// (nonce(12)||ct||tag(16))完全一致,保证 Linux/Android 两后端 blob 互通
// (前提是 key 相同;跨后端共享属后续迁移任务范畴)。
func splitAndroidBlob(blob []byte) (iv, ctTag []byte, err error) {
	min := crypto.AES256GCMNonceSize + crypto.AES256GCMTagSize
	if len(blob) < min {
		return nil, nil, fmt.Errorf("%w: blob 长度 %d 不足 iv(%d)+tag(%d)",
			ErrUnseal, len(blob), crypto.AES256GCMNonceSize, crypto.AES256GCMTagSize)
	}
	return blob[:crypto.AES256GCMNonceSize], blob[crypto.AES256GCMNonceSize:], nil
}
