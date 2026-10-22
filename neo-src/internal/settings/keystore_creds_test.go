package settings

import (
	"testing"

	"little-timer/internal/keystore"
)

// TestCredentialKeyConstantsAlignWithKeystore 守住两侧常量表的漂移。
//
// 迁移器在 keystore 包，不能 import settings，于是自带了一份凭据 key 字面量
// （keystore.CredWebDAVPassword 等）。本包的 SecretXxx 常量供日常读写分流用。
// 两份表一旦漂移，迁移会往 keystore 里写进一个与后续 Save 路径读不上的 key——
// 表现为"迁移成功但凭据读不出来"，且没有任何报错。故用本测试钉死。
func TestCredentialKeyConstantsAlignWithKeystore(t *testing.T) {
	for _, c := range []struct{ name, ours, theirs string }{
		{"webdav_password", SecretWebDAVPassword, keystore.CredWebDAVPassword},
		{"s3_access_key", SecretS3AccessKey, keystore.CredS3AccessKey},
		{"s3_secret_key", SecretS3SecretKey, keystore.CredS3SecretKey},
	} {
		if c.ours != c.theirs {
			t.Errorf("%s: settings=%q keystore=%q 两侧不一致", c.name, c.ours, c.theirs)
		}
	}
}
