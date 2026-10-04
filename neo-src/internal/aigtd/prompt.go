package aigtd

import "fmt"

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

只返回 JSON,不要返回任何其他内容。`

// SystemPrompt 渲染系统提示词,`currentDate` 为 ISO 日期(YYYY-MM-DD)。
func SystemPrompt(currentDate string) string {
	return fmt.Sprintf(systemPromptTemplate, currentDate)
}

// BuildUserPrompt 把用户随手记包成一次拆解请求。
func BuildUserPrompt(rawText string) string {
	return `请把下面的随手记整理为结构化任务。
---
` + rawText + `
---
严格按照系统约束的 JSON Schema 输出,不要附加任何解释。`
}
