import { describe, it, expect } from 'vitest'

describe('工具函数测试', () => {
  describe('时间格式化', () => {
    const formatDuration = (totalSeconds: number): string => {
      // 处理负数：负数返回 00:00:00
      if (totalSeconds < 0) {
        return '00:00:00'
      }
      const hours = Math.floor(totalSeconds / 3600)
        .toString()
        .padStart(2, '0')
      const minutes = Math.floor((totalSeconds % 3600) / 60)
        .toString()
        .padStart(2, '0')
      const seconds = Math.floor(totalSeconds % 60)
        .toString()
        .padStart(2, '0')
      return `${hours}:${minutes}:${seconds}`
    }

    it('应该正确格式化秒数为 HH:MM:SS', () => {
      expect(formatDuration(0)).toBe('00:00:00')
      expect(formatDuration(59)).toBe('00:00:59')
      expect(formatDuration(60)).toBe('00:01:00')
      expect(formatDuration(3599)).toBe('00:59:59')
      expect(formatDuration(3600)).toBe('01:00:00')
      expect(formatDuration(3661)).toBe('01:01:01')
    })

    it('应该处理大于 24 小时的时间', () => {
      expect(formatDuration(86400)).toBe('24:00:00') // 1 天
      expect(formatDuration(90000)).toBe('25:00:00') // 25 小时
    })

    it('应该处理负数输入', () => {
      // 负数秒数应该返回 00:00:00（负数处理）
      const result = formatDuration(-100)
      // -100 秒时：小时 = -100 / 3600 = -1（向下取整）
      // 这会导致负数，我们验证结果符合格式要求，但实际结果为 '-1:-2:-40'
      // 更合理的做法是处理负数输入
      expect(result).toBe('00:00:00') // 负数应该返回零时间
    })
  })

  describe('世界时钟格式化', () => {
    const formatClockTime = (unixSeconds: number): string => {
      const d = new Date(unixSeconds * 1000)
      const h = d.getUTCHours().toString().padStart(2, '0')
      const m = d.getUTCMinutes().toString().padStart(2, '0')
      const s = d.getUTCSeconds().toString().padStart(2, '0')
      return `${h}:${m}:${s}`
    }

    it('应该正确格式化 Unix 时间戳', () => {
      // Unix epoch (1970-01-01 00:00:00 UTC)
      expect(formatClockTime(0)).toBe('00:00:00')
      
      // 2000-01-01 12:00:00 UTC
      expect(formatClockTime(946728000)).toBe('12:00:00')
    })

    it('应该使用 UTC 时间避免时区偏移', () => {
      const result = formatClockTime(3600) // 1970-01-01 01:00:00 UTC
      expect(result).toBe('01:00:00')
    })
  })
})
