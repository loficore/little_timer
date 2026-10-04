package aigtd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"little-timer/internal/domain"
)

// DecompositionJSONSchema 是发给 LLM 的输出契约(OpenAI / Anthropic 都用它
// 描述期望结构,本地校验也复用同名常量)。
const DecompositionJSONSchema = `{
  "type": "object",
  "required": ["title", "is_complex", "estimated_minutes", "subtasks"],
  "properties": {
    "title": {"type": "string", "maxLength": 100},
    "notes": {"type": "string"},
    "is_complex": {"type": "boolean"},
    "due_date": {"type": ["string", "null"], "pattern": "^(\\d{4}-\\d{2}-\\d{2})?$"},
    "estimated_minutes": {"type": "integer", "minimum": 5, "maximum": 180},
    "subtasks": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["title", "estimated_minutes"],
        "properties": {
          "title": {"type": "string", "maxLength": 100},
          "estimated_minutes": {"type": "integer", "minimum": 5, "maximum": 180},
          "due_date": {"type": ["string", "null"], "pattern": "^(\\d{4}-\\d{2}-\\d{2})?$"}
        }
      }
    }
  }
}`

// ValidateDecomposition 校验 LLM 输出文本:
//
//   - 第一个返回值为解析并通过校验的 AIDecompositionResult(在校验全部通过时非 nil)
//   - 第二个返回值为人类可读的校验错误列表(全部通过时为空)
//   - 第三个返回值仅在 JSON 完全无法解析时设置
//
// 通过 `json.RawMessage` 探测关键字段是否出现,以区分"缺字段"与"显式 null"。
func ValidateDecomposition(raw string) (*domain.AIDecompositionResult, []string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil, fmt.Errorf("empty response")
	}

	// 第一次扫描:探测必需字段是否出现。
	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// 第二次扫描:解析为强类型。
	var d struct {
		Title            string              `json:"title"`
		Notes            string              `json:"notes"`
		IsComplex        bool                `json:"is_complex"`
		DueDate          *string             `json:"due_date"`
		EstimatedMinutes int                 `json:"estimated_minutes"`
		Subtasks         []domain.SubtaskDraft `json:"subtasks"`
	}
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}

	errs := []string{}
	if _, ok := probe["is_complex"]; !ok {
		errs = append(errs, "缺少字段 is_complex")
	}
	if _, ok := probe["subtasks"]; !ok {
		errs = append(errs, "缺少字段 subtasks")
	}

	title := strings.TrimSpace(d.Title)
	if title == "" {
		errs = append(errs, "title 不能为空")
	}
	if len([]rune(title)) > 100 {
		errs = append(errs, "title 超过 100 字符")
	}

	if d.EstimatedMinutes < 5 || d.EstimatedMinutes > 180 {
		errs = append(errs, fmt.Sprintf("estimated_minutes %d 超出范围 [5,180]", d.EstimatedMinutes))
	}

	if d.DueDate != nil && *d.DueDate != "" {
		if _, err := time.Parse("2006-01-02", *d.DueDate); err != nil {
			errs = append(errs, fmt.Sprintf("due_date %q 不是合法 YYYY-MM-DD", *d.DueDate))
		}
	}

	if d.IsComplex {
		if len(d.Subtasks) < 2 {
			errs = append(errs, "is_complex=true 但 subtasks 少于 2 个")
		}
		if len(d.Subtasks) > 6 {
			errs = append(errs, fmt.Sprintf("subtasks 数量 %d 过多(≤6)", len(d.Subtasks)))
		}
	} else if len(d.Subtasks) > 0 {
		errs = append(errs, "is_complex=false 但存在 subtasks")
	}

	for i, st := range d.Subtasks {
		t := strings.TrimSpace(st.Title)
		if t == "" {
			errs = append(errs, fmt.Sprintf("subtask[%d] title 不能为空", i))
		}
		if len([]rune(t)) > 100 {
			errs = append(errs, fmt.Sprintf("subtask[%d] title 超过 100 字符", i))
		}
		if st.EstimatedMinutes < 5 || st.EstimatedMinutes > 180 {
			errs = append(errs, fmt.Sprintf("subtask[%d] estimated_minutes %d 超出范围 [5,180]", i, st.EstimatedMinutes))
		}
		if st.DueDate != nil && *st.DueDate != "" {
			if _, err := time.Parse("2006-01-02", *st.DueDate); err != nil {
				errs = append(errs, fmt.Sprintf("subtask[%d] due_date %q 不是合法 YYYY-MM-DD", i, *st.DueDate))
			}
		}
	}

	res := &domain.AIDecompositionResult{
		Title:            title,
		Notes:            d.Notes,
		IsComplex:        d.IsComplex,
		DueDate:          d.DueDate,
		EstimatedMinutes: d.EstimatedMinutes,
		Subtasks:         d.Subtasks,
	}
	if res.Subtasks == nil {
		res.Subtasks = []domain.SubtaskDraft{}
	}
	return res, errs, nil
}
