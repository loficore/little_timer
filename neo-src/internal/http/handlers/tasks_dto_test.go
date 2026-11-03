package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// TestToSubtaskDTO_ExposesVersion 锁住前端乐观锁契约：子任务编辑必须能拿到
// 子任务自己的 version（父任务的 version 不能替代，否则 PATCH 必然 409）。
func TestToSubtaskDTO_ExposesVersion(t *testing.T) {
	row := domain.SubtaskRow{
		ID:               9,
		TaskID:           3,
		Title:            "列大纲",
		Status:           domain.SubtaskStatusActive,
		EstimatedMinutes: 25,
		OrderIndex:       1,
		Version:          7,
	}

	dto := toSubtaskDTO(row)
	if dto.Version != 7 {
		t.Errorf("subtask DTO version: got %d, want 7", dto.Version)
	}
}

// TestTaskList_IncludesSubtasks 锁住「列表也要带拆解步骤」的契约。
//
// 列表接口曾经给 toTaskDTO 传 nil，导致 AI 拆好的步骤在刷新页面后整块消失，
// 卡片上也无法展开编辑。
func TestTaskList_IncludesSubtasks(t *testing.T) {
	a := newTestApp(t)
	taskID, err := a.SQLite.AITasks().CreateTask("写周报", "manual")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	subID, err := a.SQLite.AITasks().CreateSubtask(taskID, "列大纲", 25, nil, 0)
	if err != nil {
		t.Fatalf("CreateSubtask: %v", err)
	}
	// 改一次子任务，把它的 version 顶到 1 —— 列表必须透出这个值，
	// 否则前端拿 0 去 PATCH 会稳定 409。
	done := domain.SubtaskStatusDone
	if err := a.SQLite.AITasks().UpdateSubtask(subID, storage.SubtaskUpdateFields{Status: &done}, 0); err != nil {
		t.Fatalf("UpdateSubtask: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app", a)
		c.Next()
	})
	r.GET("/api/tasks", TaskList)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/tasks", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (%s)", w.Code, w.Body.String())
	}

	var list []TaskDTO
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len: got %d, want 1", len(list))
	}
	subs := list[0].Subtasks
	if len(subs) != 1 || subs[0].Title != "列大纲" || subs[0].EstimatedMinutes != 25 {
		t.Fatalf("列表应带拆解步骤, got %+v", subs)
	}
	if subs[0].Version != 1 {
		t.Errorf("列表里的子任务 version: got %d, want 1", subs[0].Version)
	}
}
