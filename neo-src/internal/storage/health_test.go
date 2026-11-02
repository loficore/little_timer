package storage

import (
	"database/sql"
	"testing"
)

// TestUpdateRecordKeepsChecksum 验证刷新健康检查行时不会把 checksum 列抹成
// NULL。
//
// 历史写法是 `INSERT OR REPLACE INTO health_check (...)` 且列清单漏了
// checksum —— REPLACE 先删旧行再插新行,未列出的列退回 DEFAULT(NULL)。
// 该列目前应用层不读,但保留它比留一个"哪天开始用就被静默清零"的坑更便宜。
func TestUpdateRecordKeepsChecksum(t *testing.T) {
	m := openTempSqlite(t)
	h := m.Health()
	if err := h.Initialize(); err != nil {
		t.Fatalf("Health.Initialize: %v", err)
	}

	const want = "sha256:abc123"
	if _, err := m.DB().Exec(
		`UPDATE health_check SET checksum = ? WHERE id = 1;`, want,
	); err != nil {
		t.Fatalf("预置 checksum: %v", err)
	}

	if err := h.UpdateRecord(); err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}

	var got sql.NullString
	if err := m.DB().QueryRow(
		`SELECT checksum FROM health_check WHERE id = 1;`,
	).Scan(&got); err != nil {
		t.Fatalf("读回 checksum: %v", err)
	}
	if !got.Valid || got.String != want {
		t.Errorf("checksum: got %v, want %q", got, want)
	}
}
