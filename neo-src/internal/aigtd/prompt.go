package aigtd

import (
	"fmt"
	"strings"

	"little-timer/internal/domain"
)

// ReviewPromptTemplate 是 GTD Review 模式的提示词模板,内含 system 段(规则 +
// 输出契约)与任务清单。worker 在 review 分支直接以 UserPrompt 字段传整段;
// SystemPrompt 留空,因为 review 的 system 约束已在此模板里。
//
// 规则要点(参 docs/superpowers/specs/2026-10-08-gtd-review-gaps-design.md Δ3):
//   - action ∈ {reschedule, drop, modify_subtasks, no_op}
//   - suggested_subtask_changes[].op ∈ {delete, modify_title, add, mark_done}
//   - mark_done 只针对 active 子任务;delete/modify_title 也只针对 active
//   - done / archived 子任务不要提议任何改动("用户数据最重要")
//   - change_id 必须全局唯一(优先 UUID v4);缺失/重复由 ValidateReviewResult 兜底
//   - 长期停滞(>30 天)可建议 drop,reason 必填
const ReviewPromptTemplate = `你是 GTD 智能助理(Weekly Review 模式)。当前日期 %s。

任务:审视以下积压任务,识别延误、改期、拆分、放弃、已完成未勾、重写;返回 JSON 提议。
**只返回 JSON,不要附加任何解释、markdown 代码块或前后缀文字。**

规则:
- action ∈ {reschedule, drop, modify_subtasks, no_op}
- suggested_subtask_changes[].op ∈ {delete, modify_title, add, mark_done}
- mark_done 只针对 status='active' 的子任务(用户其实已完成但没勾)
- delete / modify_title 也只针对 active 子任务
- status='done' 或 'archived' 的子任务不要提议任何改动
- change_id 必须全局唯一(优先 UUID v4);同结果内不能重复
- 长期停滞(>30 天)可建议 drop,reason 必填
- is_complex=false 的简单任务直接 no_op,不要再补

输出契约:
{
  "proposals": [
    {"change_id":"...", "task_id": N, "action":"reschedule|drop|modify_subtasks|no_op",
     "suggested_scheduled_start": <unix>, "suggested_scheduled_end": <unix>,
     "suggested_subtask_changes": [
       {"subtask_id": M, "op":"delete|modify_title|mark_done", "new_title":"...", "reason":"..."},
       {"op":"add", "new_title":"...", "reason":"..."}
     ],
     "reason":"..."}
  ]
}

任务清单:
%s
`

// TaskSummary 描述一个被 review 的任务摘要,喂给 ReviewPrompt。
// SubtaskTitles 与 SubtaskStatuses 等长;SubtaskStatuses 用于在 prompt 里
// 标注哪些子任务已 done(LLM 据此知道不要提议改动)。
type TaskSummary struct {
	ID              int64
	Title           string
	Status          string // active/inbox/done/archived/rejected
	SubtaskTitles   []string
	SubtaskStatuses []string
	DueDate         *string
}

// ReviewPrompt 渲染 review 模式提示词(currentDate: 本地 YYYY-MM-DD)。
func ReviewPrompt(currentDate string, pending []TaskSummary) string {
	var b strings.Builder
	for i, t := range pending {
		fmt.Fprintf(&b, "\n[%d] id=%d title=%q status=%s", i, t.ID, t.Title, t.Status)
		if t.DueDate != nil {
			fmt.Fprintf(&b, " due=%s", *t.DueDate)
		}
		b.WriteString("\n  子任务:\n")
		for j, st := range t.SubtaskTitles {
			status := ""
			if j < len(t.SubtaskStatuses) {
				status = t.SubtaskStatuses[j]
			}
			fmt.Fprintf(&b, "  - %q [%s]\n", st, status)
		}
	}
	return fmt.Sprintf(ReviewPromptTemplate, currentDate, b.String())
}

// systemPromptTemplate 是发给 LLM 的系统提示词模板。`%s` 注入当前日期。
// 必须与 DecompositionJSONSchema 描述一致。
const systemPromptTemplate = `你是 GTD 智能助理。输入是一段用户随手记,可能是中文/英文混杂。
你的唯一输出是符合系统约束 JSON Schema 的结构化对象,不要附加任何解释、markdown 代码块或前后缀文字。

规则:
1. 如果描述具体且 ≤30 分钟内可完成 → is_complex=false,subtasks=[]。
2. 如果模糊或耗时 >45 分钟 → is_complex=true,拆成 2-4 个具体行动步骤,
   每个步骤能在一次专注内完成(15-90 分钟)。
3. 时间词(明早/下周三/月底)以当前日期 %s 为基准解析;无时间词 → due_date=null。
4. 提取的子任务标题必须是"动词开头"的具体动作(如"撰写大纲"而非"想想大纲")。
5. estimated_minutes 从 {15, 25, 30, 45, 60, 90} 中选择最贴近的一个,范围 [5,180]。
6. title 字段 ≤100 字符,subtasks 数组元素个数为 0 或 1-4(增量新增时可为 1)。
7. 若用户消息中给出了"已有子任务清单",你只能额外提出尚未覆盖的新增子任务,不得重复、修改或删除清单中已有的项。
8. 你必须额外输出 4 个评分维度(放在顶层,或放在名为 "_scores" 的嵌套对象里):
   - priority_score (1-5):任务重要性(数字越大越重要)。
   - urgency_score (1-10):时间紧迫度(数字越大越紧迫)。
   - energy_required (1-3):完成所需精力(1=低,2=中,3=高)。
   - context_tag:从 {写作,编码,沟通,学习,杂事,general} 中选一个最贴近的任务情境。
   评分依据:priority/urgency 对应 Eisenhower 矩阵的"重要 × 紧急"两轴;
   energy_required 对应 Loehr/Schwartz 能量管理维度;context_tag 用于降低
   context switching cost(CHIEA 2008:打断后平均需 23 分钟重建上下文)。
   这 4 项是调度启发式输入,不是精确科学——给出你最好的判断即可。

只返回 JSON,不要返回任何其他内容。`

// SystemPrompt 渲染系统提示词,`currentDate` 为 ISO 日期(YYYY-MM-DD)。
func SystemPrompt(currentDate string) string {
	return fmt.Sprintf(systemPromptTemplate, currentDate)
}

// SubtaskHint 描述一个已存在的子任务,供增量拆解 prompt 使用。
type SubtaskHint struct {
	Title  string
	Status string // active | done | archived
}

// BuildUserPrompt 把用户随手记包成一次拆解请求。
//   - existing 非空且 mode 允许(append):prompt 末尾追加已有子任务清单,并告知
//     LLM 只能产出新增。
//   - replace 模式:即使有 existing 也不带 hints,改为"全量重新分解"指令,避免
//     LLM 因"不要重复"规则主动省略未完成子任务而被 worker 当作"可删除"。
func BuildUserPrompt(rawText string, existing []SubtaskHint, mode domain.AIJobMode) string {
	base := `请把下面的随手记整理为结构化任务。
---
` + rawText + `
---
严格按照系统约束的 JSON Schema 输出,不要附加任何解释。`
	if len(existing) == 0 || mode == domain.AIJobModeReplace {
		if mode == domain.AIJobModeReplace {
			base += "\n\n这是一个完整重新分解(replace)请求:请根据随手记从头输出整个子任务集;系统会保留 status='done' 的子任务,其余将被替换。若该任务确实是复杂任务,请给出完整的 2-4 个子任务;若可一次完成则按系统规则返回 is_complex=false。"
		}
		return base
	}
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\n该任务下已有的子任务清单(标题 - 状态):\n")
	for _, s := range existing {
		b.WriteString("- ")
		b.WriteString(s.Title)
		b.WriteString(" - ")
		b.WriteString(s.Status)
		b.WriteString("\n")
	}
	b.WriteString("这些子任务已经存在,请不要在新输出中重复它们,也不要修改或删除;只输出尚未覆盖的新增子任务。")
	return b.String()
}
