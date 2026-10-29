//go:build !linux && !android && !windows && !darwin

// keystore_other.go —— `!linux && !android && !windows && !darwin` 平台的占位实现:
// 除 Name 外所有方法返回 ErrUnavailable(包文档见 keystore.go)。

package keystore

import "fmt"

// macOS Keychain 适配跟踪见 GitHub issue #13;本文件仅覆盖无原生托管的平台占位。
var errOtherUnavailable = fmt.Errorf(
	"%w: 当前平台不支持系统密钥托管",
	ErrUnavailable,
)

// otherStore 是 Keystore 的未实现占位。
type otherStore struct{}

func init() { currentPlatform = otherStore{} }

// Name 返回 "other",供 UI/日志区分占位后端。
func (otherStore) Name() string { return "other" }

// Available 返回包装 ErrUnavailable 的固定错误。
func (otherStore) Available() error { return errOtherUnavailable }

// EnsureMasterKey 返回包装 ErrUnavailable 的固定错误。
func (otherStore) EnsureMasterKey() error { return errOtherUnavailable }

// HasMasterKey 返回 (false, 包装 ErrUnavailable 的固定错误)。
func (otherStore) HasMasterKey() (bool, error) { return false, errOtherUnavailable }

// Seal 返回 (nil, 包装 ErrUnavailable 的固定错误)。
func (otherStore) Seal([]byte) ([]byte, error) { return nil, errOtherUnavailable }

// Unseal 返回 (nil, 包装 ErrUnavailable 的固定错误)。
func (otherStore) Unseal([]byte) ([]byte, error) { return nil, errOtherUnavailable }

// DeleteMasterKey 返回包装 ErrUnavailable 的固定错误。
func (otherStore) DeleteMasterKey() error { return errOtherUnavailable }
