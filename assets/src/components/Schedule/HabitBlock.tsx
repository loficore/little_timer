/**
 * HabitBlock —— 习惯时段块(占位)。
 *
 * v1 暂不实现 habit_schedules 落地(spec §7.2 明确「习惯融合」属 Plan 3,v1 不在
 * 范围),后端 `loadHabitSlotsForDate` 永远返回空切片。该组件在 Timeline 中
 * 渲染一条带说明的占位轨,让 v1 用户能直观看到这条 lane 为何空白。
 */

import { t } from "../../utils/i18n";

export function HabitBlock() {
  return (
    <div
      className="timeline-habit-block mt-2 flex h-8 items-center justify-center rounded-lg border border-dashed border-base-content/30 bg-base-200/50 text-xs opacity-70"
      data-testid="habit-lane"
    >
      <span>{t("schedule.habit_placeholder")}</span>
    </div>
  );
}