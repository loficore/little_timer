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
    "notes": {"type": "string", "maxLength": 2000},
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
    },
    "_scores": {
      "type": "object",
      "description": "4 维调度评分;嵌套形态优先(spec §5.3)。",
      "properties": {
        "priority_score":  {"type": "integer", "minimum": 1, "maximum": 5},
        "urgency_score":   {"type": "integer", "minimum": 1, "maximum": 10},
        "energy_required": {"type": "integer", "minimum": 1, "maximum": 3},
        "context_tag":     {"type": "string", "enum": ["写作", "编码", "沟通", "学习", "杂事", "general"]}
      }
    }
  }
}`

// 4 维评分默认值(spec §3.1;与 tasks 表列 DEFAULT 一致,确保越界回落时
// DB CHECK 约束不会拒)。
const (
	defaultPriority = 3 // tasks.priority_score DEFAULT 3
	defaultUrgency  = 5 // tasks.urgency_score  DEFAULT 5
	defaultEnergy   = 2 // tasks.energy_required DEFAULT 2
)

// scoreWarnPrefix 是 4 维评分警告的统一前缀(spec §5.3 / task 7)。ParseWithRetry
// 据此把警告与"真 schema 错误"区分:警告不触发解析重试、警告不影响最终成功
// —— 警告的存在只意味着 LLM 给了越界值,ValidateDecomposition 已就地回退
// 默认并把信息留在 errs(供上层日志 / UX),worker 拿到的 res 已含合法值
// (Review Focus #3:永不 panic;§12:LLM 评分失败 → 默认值 + 提示)。
const scoreWarnPrefix = "[scores] "

// validContextTags 是 context_tag 的枚举白名单(spec §3.1 / §5.3)。
// 显式列出便于 review & 复用;新增枚举时同步改 schema + tasks CHECK。
var validContextTags = map[string]struct{}{
	"写作":      {},
	"编码":      {},
	"沟通":      {},
	"学习":      {},
	"杂事":      {},
	"general": {},
}

// scoreRaw 是 4 维评分的原始 JSON 形态。指针用于探测"出现 vs 显式零值";
// ValidateDecomposition 据此区分"缺省(静默默认)"与"显式 0(回退 + 警告)"。
type scoreRaw struct {
	PriorityScore  *int    `json:"priority_score"`
	UrgencyScore   *int    `json:"urgency_score"`
	EnergyRequired *int    `json:"energy_required"`
	ContextTag     *string `json:"context_tag"`
}

// ValidateDecomposition 校验 LLM 输出文本:
//
//   - 第一个返回值为解析并通过校验的 AIDecompositionResult(在校验全部通过时非 nil)
//   - 第二个返回值为人类可读的校验错误列表(全部通过时为空)
//   - 第三个返回值仅在 JSON 完全无法解析时设置
//
// 通过 `json.RawMessage` 探测关键字段是否出现,以区分"缺字段"与"显式 null"。
//
// 4 维评分(spec §5.3 / task 7):同时支持嵌套 `_scores` 对象与顶层键,嵌套形态
// 优先;缺省视为默认(静默);越界 / 非枚举回退默认并向 errs 追加警告——
// 永不 panic(Review Focus #3 / §12)。
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
		Title            string                `json:"title"`
		Notes            string                `json:"notes"`
		IsComplex        bool                  `json:"is_complex"`
		DueDate          *string               `json:"due_date"`
		EstimatedMinutes int                   `json:"estimated_minutes"`
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
	if len([]rune(d.Notes)) > 2000 {
		errs = append(errs, "notes 超过 2000 字符")
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
		// 下限为 1 而非 2:append 模式允许只新增 1 个子任务(#9)。
		// 仍拒绝 is_complex=true + subtasks=[] 这种自相矛盾的状态。
		if len(d.Subtasks) < 1 {
			errs = append(errs, "is_complex=true 但 subtasks 少于 1 个")
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

	// 4 维评分解析(spec §5.3 / task 7):
	//   1. 优先取嵌套 `_scores` 的字段;_scores 不存在或非对象 → 回退顶层键。
	//   2. 每一维缺省(指针 nil)→ 静默默认;显式越界 / 非枚举 → 回退 + 警告。
	var top scoreRaw
	_ = json.Unmarshal([]byte(raw), &top) // 顶层键,无 _scores 时是唯一来源

	var nested scoreRaw
	nestedValid := false
	if rawScores, ok := probe["_scores"]; ok && len(rawScores) > 0 && string(rawScores) != "null" {
		// _scores 解析失败不是 fatal — 视为对象不存在,fallback 到顶层键 + 警告。
		if jerr := json.Unmarshal(rawScores, &nested); jerr != nil {
			errs = append(errs, fmt.Sprintf("_scores 不是合法对象(%v)", jerr))
		} else {
			nestedValid = true
		}
	}

	pickInt := func(n, t *int) *int {
		if nestedValid && n != nil {
			return n
		}
		return t
	}
	pickStr := func(n, t *string) *string {
		if nestedValid && n != nil {
			return n
		}
		return t
	}

	priority := defaultPriority
	if p := pickInt(nested.PriorityScore, top.PriorityScore); p != nil {
		if *p >= 1 && *p <= 5 {
			priority = *p
		} else {
			errs = append(errs, scoreWarnPrefix+fmt.Sprintf("priority_score %d 超出范围 [1,5],已回退默认 %d", *p, defaultPriority))
		}
	}
	urgency := defaultUrgency
	if u := pickInt(nested.UrgencyScore, top.UrgencyScore); u != nil {
		if *u >= 1 && *u <= 10 {
			urgency = *u
		} else {
			errs = append(errs, scoreWarnPrefix+fmt.Sprintf("urgency_score %d 超出范围 [1,10],已回退默认 %d", *u, defaultUrgency))
		}
	}
	energy := defaultEnergy
	if e := pickInt(nested.EnergyRequired, top.EnergyRequired); e != nil {
		if *e >= 1 && *e <= 3 {
			energy = *e
		} else {
			errs = append(errs, scoreWarnPrefix+fmt.Sprintf("energy_required %d 超出范围 [1,3],已回退默认 %d", *e, defaultEnergy))
		}
	}
	contextTag := "general"
	if c := pickStr(nested.ContextTag, top.ContextTag); c != nil {
		if _, ok := validContextTags[*c]; ok {
			contextTag = *c
		} else {
			errs = append(errs, scoreWarnPrefix+fmt.Sprintf("context_tag %q 不在枚举(写作/编码/沟通/学习/杂事/general),已回退 general", *c))
		}
	}

	res := &domain.AIDecompositionResult{
		Title:            title,
		Notes:            d.Notes,
		IsComplex:        d.IsComplex,
		DueDate:          d.DueDate,
		EstimatedMinutes: d.EstimatedMinutes,
		Subtasks:         d.Subtasks,
		PriorityScore:    priority,
		UrgencyScore:     urgency,
		EnergyRequired:   energy,
		ContextTag:       contextTag,
	}
	if res.Subtasks == nil {
		res.Subtasks = []domain.SubtaskDraft{}
	}
	return res, errs, nil
}
