package aigtd

import (
	"fmt"
	"strings"

	"little-timer/internal/domain"
)

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
6. title 字段 ≤100 字符,subtasks 数组元素个数为 0 或 2-4。
7. 若用户消息中给出了"已有子任务清单",你只能额外提出尚未覆盖的新增子任务,不得重复、修改或删除清单中已有的项。

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
