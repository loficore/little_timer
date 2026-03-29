// 固定的模式标识符，供前后端通信使用；显示文本请使用 i18n t()
export const Mode = {
  Countdown: 'countdown',
  Stopwatch: 'stopwatch',
} as const;

export type Mode = (typeof Mode)[keyof typeof Mode];