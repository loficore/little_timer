/**
 * Schedule 组件共享的展示层类型。
 *
 * 注意:这里刻意用 camelCase/snake_case 混合的「展示」形状(与 TaskDTO 的
 * snake_case 评分字段一致),与后端 PascalCase 的 `SchedulerPlacement` 解耦。
 * SchedulePage 负责把 `Plan.Placements` + `TaskDTO` 映射成 `TimelineTask`。
 */
export interface TimelineTask {
  id: number;
  title: string;
  priority_score: number;
  /** unix 秒 */
  scheduled_start: number;
  /** unix 秒 */
  scheduled_end: number;
  context_tag?: string;
}
