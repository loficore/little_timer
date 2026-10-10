// Package handlers —— 测试用内存 keystore。原本地 fakeKeystore 副本已去重到
// little-timer/internal/keystore/keystoretest(让凭据 store 在无 keyring
// daemon 的 CI 上也能真实 Seal/Unseal 落盘 round-trip,跑通 SecretStorage v2
// 的磁盘路径而不依赖 dbus)。本文件仅保留编译期断言。
package handlers

import (
	"little-timer/internal/keystore"
	"little-timer/internal/keystore/keystoretest"
)

var _ keystore.Keystore = keystoretest.FakeKeystore{}
