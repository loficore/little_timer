// Package main —— Task 6 启动 fail-fast 的单元测试。
//
// 覆盖 spec §9 的决策函数 bootstrapKeystore:
//   - keystore 可用 → EnsureMasterKey 成功,返回 nil;
//   - keystore 可用但 EnsureMasterKey 失败 → fail-fast 错误(同 ErrUnavailable 链);
//   - keystore 不可用且磁盘上有 v1("LTMSECv1")secret 文件 → errMigrationPending
//     (main 进入迁移等待模式,Task 8/9 接管);
//   - keystore 不可用且无 v1 文件 → 错误链保持 keystore.ErrUnavailable,
//     且错误文案原样透出 §9 的 gnome-keyring 指引(Review Focus #5)。
//
// 不触碰 DB/HTTP:纯决策函数,全部用本地可控 fake 驱动。
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"little-timer/internal/keystore"
)

// keyringGuidanceSubstring —— spec §9 指引中必须透出的可操作命令片段
// (与 internal/keystore/secret_service.go 的 keyringGuidance 同源;此处只做
// 断言,不复制整段文案)。
const keyringGuidanceSubstring = "gnome-keyring-daemon --daemonize --components=secrets"

// fakeKeystore —— cmd/server 本地最小可控 fake。
// internal/keystore/keystoretest.FakeKeystore 的 Available()/EnsureMasterKey()
// 恒为 nil,无法驱动 fail-fast 分支,故这里不共享、只实现本测试需要的行为;
// 其余方法为编译所需的直通桩。
type fakeKeystore struct {
	availableErr error
	ensureErr    error
	ensureCalls  int
}

var _ keystore.Keystore = (*fakeKeystore)(nil)

func (f *fakeKeystore) Name() string                { return "fake" }
func (f *fakeKeystore) Available() error            { return f.availableErr }
func (f *fakeKeystore) EnsureMasterKey() error      { f.ensureCalls++; return f.ensureErr }
func (f *fakeKeystore) HasMasterKey() (bool, error) { return true, nil }
func (f *fakeKeystore) Seal(plaintext []byte) ([]byte, error) {
	return append([]byte(nil), plaintext...), nil
}
func (f *fakeKeystore) Unseal(blob []byte) ([]byte, error) {
	return append([]byte(nil), blob...), nil
}
func (f *fakeKeystore) DeleteMasterKey() error { return nil }

// unavailableWithGuidance 复刻 linux 后端 unavailable() 的错误结构:
// 哨兵 + 原因 + §9 指引,验证 bootstrapKeystore 是纯透传而非重写文案。
func unavailableWithGuidance(reason string) error {
	return fmt.Errorf("%w: %s; 桌面环境通常自带;sway/最小安装可运行 "+
		"`gnome-keyring-daemon --daemonize --components=secrets` "+
		"或安装 KWallet / KeePassXC。", keystore.ErrUnavailable, reason)
}

// writeSecretFile 在独立临时目录下放置指定内容的 secret.db 并返回其路径。
func writeSecretFile(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret.db")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("写入 secret.db 失败: %v", err)
	}
	return path
}

func TestBootstrapKeystore(t *testing.T) {
	tests := []struct {
		name string
		ks   *fakeKeystore
		// secretPath 构造:惰性(需要文件用例的目录互相独立),nil 表示
		// "文件不存在,用空 temp 目录路径"。
		secretPath func(t *testing.T) string
		wantErr    string // 期望错误语义
		check      func(t *testing.T, err error, ks *fakeKeystore)
	}{
		{
			name:       "available-ok 走 EnsureMasterKey 并返回 nil",
			ks:         &fakeKeystore{},
			secretPath: nil,
			check: func(t *testing.T, err error, ks *fakeKeystore) {
				if err != nil {
					t.Fatalf("期望 nil,实际 %v", err)
				}
				if ks.ensureCalls != 1 {
					t.Errorf("EnsureMasterKey 应恰好调用 1 次,实际 %d 次", ks.ensureCalls)
				}
			},
		},
		{
			name: "available-fail + v1 文件 → errMigrationPending",
			ks:   &fakeKeystore{availableErr: unavailableWithGuidance("会话总线上没有密钥服务")},
			secretPath: func(t *testing.T) string {
				return writeSecretFile(t, append([]byte("LTMSECv1"), make([]byte, 64)...))
			},
			check: func(t *testing.T, err error, ks *fakeKeystore) {
				if !errors.Is(err, errMigrationPending) {
					t.Fatalf("期望 errMigrationPending,实际 %v", err)
				}
				if ks.ensureCalls != 0 {
					t.Errorf("迁移等待不应调用 EnsureMasterKey,实际 %d 次", ks.ensureCalls)
				}
			},
		},
		{
			name:       "available-fail + 无文件 → ErrUnavailable 链 + 指引文案",
			ks:         &fakeKeystore{availableErr: unavailableWithGuidance("会话总线上没有密钥服务")},
			secretPath: nil,
			wantErr:    "密钥服务不可用",
			check: func(t *testing.T, err error, ks *fakeKeystore) {
				if err == nil {
					t.Fatal("期望非 nil 错误")
				}
				if errors.Is(err, errMigrationPending) {
					t.Fatalf("无 v1 文件不应进入迁移等待模式: %v", err)
				}
				if !errors.Is(err, keystore.ErrUnavailable) {
					t.Errorf("错误链应可 errors.Is 到 keystore.ErrUnavailable: %v", err)
				}
				if !strings.Contains(err.Error(), keyringGuidanceSubstring) {
					t.Errorf("错误文案必须含 §9 指引 %q,实际: %v", keyringGuidanceSubstring, err)
				}
			},
		},
		{
			name: "available-fail + v2 文件(非 v1)→ 仍 fail-fast,不进迁移等待",
			ks:   &fakeKeystore{availableErr: unavailableWithGuidance("密钥服务未运行")},
			secretPath: func(t *testing.T) string {
				return writeSecretFile(t, append([]byte("LTMSEKv2"), make([]byte, 64)...))
			},
			check: func(t *testing.T, err error, ks *fakeKeystore) {
				if err == nil || errors.Is(err, errMigrationPending) {
					t.Fatalf("v2 文件不构成迁移等待理由,期望 fail-fast 错误,实际 %v", err)
				}
				if !errors.Is(err, keystore.ErrUnavailable) {
					t.Errorf("错误链应可 errors.Is 到 keystore.ErrUnavailable: %v", err)
				}
				if !strings.Contains(err.Error(), keyringGuidanceSubstring) {
					t.Errorf("错误文案必须含 §9 指引,实际: %v", err)
				}
			},
		},
		{
			name: "available-fail + 无 magic 垃圾文件 → fail-fast",
			ks:   &fakeKeystore{availableErr: unavailableWithGuidance("密钥服务未运行")},
			secretPath: func(t *testing.T) string {
				return writeSecretFile(t, []byte("not an envelope at all"))
			},
			check: func(t *testing.T, err error, ks *fakeKeystore) {
				if err == nil || errors.Is(err, errMigrationPending) {
					t.Fatalf("垃圾前缀不构成迁移等待理由,期望 fail-fast 错误,实际 %v", err)
				}
			},
		},
		{
			name: "available-fail + 不足 8 字节的截断文件 → fail-fast",
			ks:   &fakeKeystore{availableErr: unavailableWithGuidance("密钥服务未运行")},
			secretPath: func(t *testing.T) string {
				return writeSecretFile(t, []byte("LTM"))
			},
			check: func(t *testing.T, err error, ks *fakeKeystore) {
				if err == nil || errors.Is(err, errMigrationPending) {
					t.Fatalf("截断文件不构成迁移等待理由,期望 fail-fast 错误,实际 %v", err)
				}
			},
		},
		{
			name: "available-ok 但 EnsureMasterKey 失败 → fail-fast 错误(Review Focus #5 同要求)",
			ks: &fakeKeystore{
				ensureErr: unavailableWithGuidance("集合已锁定"),
			},
			secretPath: nil,
			check: func(t *testing.T, err error, ks *fakeKeystore) {
				if err == nil {
					t.Fatal("期望非 nil 错误")
				}
				if errors.Is(err, errMigrationPending) {
					t.Fatalf("EnsureMasterKey 失败不应转入迁移等待模式: %v", err)
				}
				if !errors.Is(err, keystore.ErrUnavailable) {
					t.Errorf("错误链应保留 keystore.ErrUnavailable: %v", err)
				}
				if !strings.Contains(err.Error(), keyringGuidanceSubstring) {
					t.Errorf("错误文案必须含 §9 指引,实际: %v", err)
				}
				if ks.ensureCalls != 1 {
					t.Errorf("EnsureMasterKey 应被尝试 1 次,实际 %d 次", ks.ensureCalls)
				}
			},
		},
		{
			name: "v1 文件存在但 keystore 可用 → 正常模式(迁移器 Task 8 处理)",
			ks:   &fakeKeystore{},
			secretPath: func(t *testing.T) string {
				return writeSecretFile(t, append([]byte("LTMSECv1"), make([]byte, 64)...))
			},
			check: func(t *testing.T, err error, ks *fakeKeystore) {
				if err != nil {
					t.Fatalf("keystore 可用时应走正常模式,实际 %v", err)
				}
				if ks.ensureCalls != 1 {
					t.Errorf("EnsureMasterKey 应恰好调用 1 次,实际 %d 次", ks.ensureCalls)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "secret.db") // 默认:不存在
			if tt.secretPath != nil {
				path = tt.secretPath(t)
			}
			tt.check(t, bootstrapKeystore(tt.ks, path), tt.ks)
		})
	}
}

func TestBootstrapKeystoreNilKeystore(t *testing.T) {
	err := bootstrapKeystore(nil, filepath.Join(t.TempDir(), "secret.db"))
	if err == nil {
		t.Fatal("平台 keystore 未注册必须 fail-fast")
	}
	if errors.Is(err, errMigrationPending) {
		t.Errorf("nil keystore 不应转入迁移等待: %v", err)
	}
	if !errors.Is(err, keystore.ErrUnavailable) {
		t.Errorf("错误链应可 errors.Is 到 keystore.ErrUnavailable: %v", err)
	}
}

// TestCurrentKeystoreUnavailableErrorCarriesGuidance 验证真实 linux 后端的
// Available() 失败时错误链同时保留哨兵与 §9 指引 —— 这是 Review Focus #5
// 在透传路径上的事实依据(bootstrapKeystore 只 %w 透传,不重写文案)。
// 本机密钥服务可用时跳过(不可用场景无法人造,但 CI 精简镜像会命中)。
func TestCurrentKeystoreUnavailableErrorCarriesGuidance(t *testing.T) {
	ks := keystore.Current()
	if ks == nil || ks.Name() != "secret-service" {
		t.Skipf("非 linux Secret Service 后端(%v),指引文案不适用", ks)
	}
	err := ks.Available()
	if err == nil {
		t.Skip("本机密钥服务可用,无法断言不可用文案")
	}
	if !errors.Is(err, keystore.ErrUnavailable) {
		t.Errorf("Available 失败应链到 ErrUnavailable: %v", err)
	}
	if !strings.Contains(err.Error(), keyringGuidanceSubstring) {
		t.Errorf("linux 后端不可用错误必须自带 §9 指引 %q,实际: %v", keyringGuidanceSubstring, err)
	}
	// 决策函数在同输入下必须原样透出这段文案。
	if got := bootstrapKeystore(ks, filepath.Join(t.TempDir(), "secret.db")); got == nil ||
		!strings.Contains(got.Error(), keyringGuidanceSubstring) {
		t.Errorf("bootstrapKeystore 应透传指引文案,实际: %v", got)
	}
}
