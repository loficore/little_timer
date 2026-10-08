// Package storage —— AdoptionCrud:adoptions 表的 CRUD(GTD Review 用户采纳审计,v12)。
//
//   - RecordAdoption / RecordAdoptionTx 写一行采纳记录;唯一键 (ai_job_id, change_id)
//     由 idx_adoptions_idempotent 兜底(UNIQUE 冲突返回 ErrAdoptionDuplicate,
//     让 caller 按幂等处理,而非视作 hard error);
//   - FindAdoption 按幂等键读一行(handler adopt 前置命中用);无行返 (nil, nil)。
package storage

import (
	"database/sql"
	"errors"
	"fmt"

	"little-timer/internal/domain"
)

// ErrAdoptionDuplicate 标记 (ai_job_id, change_id) 重复采纳 —— idx_adoptions_idempotent
// UNIQUE 索引兜底的并发竞态(handler 捕获后按幂等返回)。
var ErrAdoptionDuplicate = errors.New("storage: adoption duplicate (ai_job_id, change_id)")

// AdoptionCrud 操作 adoptions 表。
type AdoptionCrud struct {
	db *sql.DB
}

// NewAdoptionCrud 返回空的 AdoptionCrud。SetDB 由 SqliteManager 调用。
func NewAdoptionCrud() *AdoptionCrud { return &AdoptionCrud{} }

// SetDB 接入 *sql.DB。
func (c *AdoptionCrud) SetDB(db *sql.DB) { c.db = db }

// insertSQL 是 RecordAdoption 与 RecordAdoptionTx 共享的 INSERT 语句。
const adoptionInsertSQL = `INSERT INTO adoptions
    (task_id, ai_job_id, change_id, version_before, version_after)
    VALUES (?, ?, ?, ?, ?);`

// recordAdoptionExec 是 RecordAdoption / RecordAdoptionTx 共享的中心函数;
// dbExecer 同时满足 *sql.DB 与 *sql.Tx,事务/非事务路径 SQL 不分叉。
func (c *AdoptionCrud) recordAdoptionExec(exec dbExecer, a domain.Adoption) (int64, error) {
	res, err := exec.Exec(adoptionInsertSQL,
		a.TaskID, a.AIJobID, a.ChangeID, a.VersionBefore, a.VersionAfter)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrAdoptionDuplicate
		}
		return 0, fmt.Errorf("%w: %v", ErrAITaskInsertFailed, err)
	}
	return res.LastInsertId()
}

// RecordAdoption 写一行采纳。
func (c *AdoptionCrud) RecordAdoption(a domain.Adoption) (int64, error) {
	return c.recordAdoptionExec(c.db, a)
}

// RecordAdoptionTx 是 RecordAdoption 的事务变体。
func (c *AdoptionCrud) RecordAdoptionTx(tx *sql.Tx, a domain.Adoption) (int64, error) {
	return c.recordAdoptionExec(tx, a)
}

// FindAdoption 按 (ai_job_id, change_id) 查一行。无行 → (nil, nil)。
func (c *AdoptionCrud) FindAdoption(aiJobID int64, changeID string) (*domain.Adoption, error) {
	var a domain.Adoption
	err := c.db.QueryRow(
		`SELECT id, task_id, ai_job_id, change_id, version_before, version_after, applied_at
		 FROM adoptions WHERE ai_job_id = ? AND change_id = ?;`,
		aiJobID, changeID,
	).Scan(&a.ID, &a.TaskID, &a.AIJobID, &a.ChangeID, &a.VersionBefore, &a.VersionAfter, &a.AppliedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAITaskQueryFailed, err)
	}
	return &a, nil
}
