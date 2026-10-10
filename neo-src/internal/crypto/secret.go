// Package crypto —— 由平台 keystore 托管的 secret 存储(v2 薄壳)。
//
// SecretStorage 不再管理主口令:加解密委托给注入的 master key 托管
// (消费侧最小接口 Keystore,由 internal/keystore 的平台实现以结构类型
// 满足),本包只负责"内存明文 cache + JSON 序列化 + 带 magic 前缀的
// envelope 落盘"。明文只在 Seal/Unseal 前后短暂存在,口令 → 密钥派生
// 路径已整体删除(见 legacy_v1.go 的 Deprecated 说明)。
//
// 磁盘 blob 的 wire format:
//
//	magic (8 字节:"LTMSEKv2") || Keystore.Seal( JSON(map[string]string) )
//
// Seal 的明文载荷为 `{"key": base64(value), …}`(与 v1 相同);Seal 产物
// 的布局由平台实现决定(Linux 后端为 nonce(12) || ciphertext || tag(16))。
// v1 文件("LTMSECv1")由 keystore 迁移器一次性处理,本薄壳不读取。
//
// 依赖边界:本包不 import internal/keystore —— keystore 的平台实现文件
// 反向使用本包的 AES-GCM 原语,直接 import 会构成编译环;因此这里声明
// 消费侧接口,任何 keystore.Keystore 值均可直接传入 NewSecretStorage。
package crypto

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"

	"little-timer/internal/log"
)

// SecretError 枚举 secret 存储的各类失败模式。
type SecretError string

const (
	ErrSecretNotFound         SecretError = "secret not found"
	ErrSecretAlreadyExists    SecretError = "secret already exists"
	ErrSecretInvalidValue     SecretError = "invalid value"
	ErrSecretNoAccess         SecretError = "no access"
	ErrSecretOutOfMemory      SecretError = "out of memory"
	ErrSecretDecryptionFailed SecretError = "decryption failed"
	ErrSecretIO               SecretError = "secret store i/o failed"

	// ErrKeystoreUnavailable —— master key 托管不可用:key 缺失、
	// Seal/Unseal 失败(如 keyring 被重置)时透出。语义 =「现有凭据
	// 读不出来,需要走重置/迁移流程」,调用方不得当成"用户没设置过
	// secret"静默处理(spec §10;Review Focus #2)。
	ErrKeystoreUnavailable SecretError = "keystore unavailable"
)

func (e SecretError) Error() string { return string(e) }

// Keystore —— SecretStorage 对 master key 托管的最小依赖面(消费侧接口)。
// 取 internal/keystore.Keystore 方法集的子集,语义与那边逐字一致:
//   - HasMasterKey 报告 master key 是否已托管(区分"新装"与"老数据");
//   - Seal 用 master key 加密明文;
//   - Unseal 解密 Seal 产物,认证/取 key 失败返回 keystore 哨兵错误。
//
// 在消费侧声明(而非 import keystore 包)是为了消除 crypto ↔ keystore
// 编译环:keystore 的 Linux 实现用本包原语做加解密。
type Keystore interface {
	HasMasterKey() (bool, error)
	Seal(plaintext []byte) ([]byte, error)
	Unseal(blob []byte) ([]byte, error)
}

// v2 磁盘 blob 的 magic 前缀。用来把自己的文件和磁盘上的意外垃圾
// (含 v1 遗留 "LTMSECv1")区分开。
var secretMagic = []byte("LTMSEKv2")

// SecretStorage 是 keystore envelope 保护的凭据存储薄壳。
// 每进程一个实例;所有方法都是 goroutine 安全的。
type SecretStorage struct {
	mu sync.Mutex

	// filePath 是加密 blob 在磁盘上的位置。为空表示
	// "不持久化 —— 进程退出后 secret 即消失"。
	filePath string

	// ks 是注入的 master key 托管,构造后只读。不得为 nil。
	ks Keystore

	// secrets 是内存明文 cache。key 是用户提供的任意字节切片;在 JSON
	// map 内按 string(key) 序列化。
	secrets map[string][]byte

	// loadFailure 非 nil = 构造时磁盘 secret 加载失败(master key 缺失、
	// Unseal 失败或文件损坏),记录透出原因。此时 cache 必为空,"查询
	// 无结果"必须报告该原因而不是 not-found/空列表,防止调用方把
	// keyring 被重置误读成"用户没设过凭据"。
	loadFailure error
}

// NewSecretStorage 返回由 ks 托管 master key、持久化到 filePath 的存储。
// filePath 为 "" 时只在内存中工作。
//
// 构造永不失败(返回的指针总是可用):
//   - master key 就绪(HasMasterKey==true)且文件存在 → loadV2 填充 cache;
//   - master key 缺失而文件存在 → 空 cache,打一行 warn,并把不可用原因
//     记入 loadFailure —— 退出/重置策略由启动流程(spec §9)决定;
//   - 文件不存在 → 空 cache,首次 Store 落盘。
func NewSecretStorage(filePath string, ks Keystore) *SecretStorage {
	s := &SecretStorage{
		filePath: filePath,
		ks:       ks,
		secrets:  make(map[string][]byte),
	}
	if filePath == "" {
		return s // 纯内存模式:无盘可载。
	}
	if _, err := os.Stat(filePath); err != nil {
		return s // 文件不存在(或不可 stat):按新装处理,Store 时再暴露读写错误。
	}

	has, hasErr := ks.HasMasterKey()
	if hasErr != nil {
		s.loadFailure = fmt.Errorf("%w: 查询 master key 状态失败: %v",
			ErrKeystoreUnavailable, hasErr)
		log.Warn("crypto: keystore key missing, secrets unusable", "error", hasErr.Error())
		return s
	}
	if !has {
		s.loadFailure = fmt.Errorf("%w: master key 不在托管中,既有 secret 不可读",
			ErrKeystoreUnavailable)
		// 不 fail 构造:让启动流程/迁移器(Task 8)决定退出还是重建。
		log.Warn("crypto: keystore key missing, secrets unusable")
		return s
	}

	if err := s.loadV2(); err != nil {
		s.loadFailure = err
		log.Warn("crypto: secret file load failed, existing secrets unusable",
			"error", err.Error())
	}
	return s
}

// Secret 操作。

// Store 插入或替换一对 key/value,随后原子落盘(纯内存模式跳过落盘)。
func (s *SecretStorage) Store(key, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.secrets[string(key)] = append([]byte(nil), value...)
	return s.persistLocked()
}

// Retrieve 返回 key 对应的明文值。返回的是副本;调用方用完可自行擦除。
// cache 未命中且构造时加载失败过 → 透出 loadFailure(不谎报 not-found)。
func (s *SecretStorage) Retrieve(key []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v, ok := s.secrets[string(key)]; ok {
		return append([]byte(nil), v...), nil
	}
	if s.loadFailure != nil {
		return nil, s.loadFailure
	}
	return nil, ErrSecretNotFound
}

// Delete 删除一个 key。删除不存在的 key 是 no-op(仍然落盘,保持幂等语义)。
func (s *SecretStorage) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v, ok := s.secrets[string(key)]; ok {
		zeroBytes(v)
		delete(s.secrets, string(key))
	}
	return s.persistLocked()
}

// Clear 擦除所有 secret。适用于"重置凭据"流程。
func (s *SecretStorage) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, v := range s.secrets {
		zeroBytes(v)
		delete(s.secrets, k)
	}
	return s.persistLocked()
}

// Items 返回当前 cache 中的全部 key(按字典序升序),供迁移器与 settings
// 反查 has_* 标记使用。cache 因加载失败为空时透出 loadFailure 而不是空
// 列表。
func (s *SecretStorage) Items() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.secrets) == 0 && s.loadFailure != nil {
		return nil, s.loadFailure
	}
	names := make([]string, 0, len(s.secrets))
	for k := range s.secrets {
		names = append(names, k)
	}
	sort.Strings(names)
	return names, nil
}

// 内部实现。

// loadV2 读取磁盘 blob,Unseal + JSON 解析填充 s.secrets;任何一步失败
// 都返回带哨兵错误的说明且保持 cache 为空(构造场景专用,无并发)。
func (s *SecretStorage) loadV2() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 与 Store 前被外部删除竞态:空 cache 即可。
		}
		return fmt.Errorf("%w: 读取 secret 文件失败: %v", ErrSecretIO, err)
	}
	if !bytes.HasPrefix(data, secretMagic) {
		return fmt.Errorf("%w: 文件前缀不是 %q(未加密或 v1 遗留文件,需迁移)",
			ErrSecretDecryptionFailed, secretMagic)
	}

	plaintext, err := s.ks.Unseal(data[len(secretMagic):])
	if err != nil {
		return fmt.Errorf("%w: keystore 解封失败: %v", ErrKeystoreUnavailable, err)
	}
	defer zeroBytes(plaintext)

	decoded := make(map[string]string)
	if err := json.Unmarshal(plaintext, &decoded); err != nil {
		return fmt.Errorf("%w: 解封内容不是合法 JSON: %v", ErrSecretDecryptionFailed, err)
	}
	secrets := make(map[string][]byte, len(decoded))
	for k, v := range decoded {
		raw, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return fmt.Errorf("%w: secret 值 base64 解码失败: %v", ErrSecretDecryptionFailed, err)
		}
		secrets[k] = raw
	}
	// 全部解析成功后才提升为活动 cache。
	s.secrets = secrets
	return nil
}

// persistLocked 把 s.secrets 序列化为 JSON(map[string]base64,与 v1 同
// 格式),经 ks.Seal 加密后以 magic || sealed 布局写盘:先写
// filePath+".tmp"(0o600)再 os.Rename 原子替换。未配置 filePath 时是
// no-op;Seal 失败透出 ErrKeystoreUnavailable。
func (s *SecretStorage) persistLocked() error {
	if s.filePath == "" {
		return nil
	}

	encoded := make(map[string]string, len(s.secrets))
	for k, v := range s.secrets {
		encoded[k] = base64.StdEncoding.EncodeToString(v)
	}
	plaintext, err := json.Marshal(encoded)
	if err != nil {
		return fmt.Errorf("%w: JSON 序列化失败: %v", ErrSecretIO, err)
	}
	defer zeroBytes(plaintext)

	sealed, err := s.ks.Seal(plaintext)
	if err != nil {
		return fmt.Errorf("%w: keystore 密封失败: %v", ErrKeystoreUnavailable, err)
	}

	out := make([]byte, 0, len(secretMagic)+len(sealed))
	out = append(out, secretMagic...)
	out = append(out, sealed...)

	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0o600); err != nil {
		return fmt.Errorf("%w: 写入临时文件失败: %v", ErrSecretIO, err)
	}
	if err := os.Rename(tmpPath, s.filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("%w: 原子替换 secret 文件失败: %v", ErrSecretIO, err)
	}
	return nil
}

// zeroBytes 就地擦除一个 byte slice。针对 heap dump 的尽力防御;
// Go 的 GC 可能已复制过数据,所以这是卫生习惯,不是保证。
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
