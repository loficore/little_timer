// 测试环境设置文件
import { afterEach, vi } from 'vitest'
import { cleanup } from '@testing-library/preact'

// 每个测试后清理
afterEach(() => {
  cleanup()
})

// Mock window.webui
declare global {
  interface Window {
    webui?: {
      call: (functionName: string, ...args: unknown[]) => void;
    };
  }
}

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
}
global.localStorage = localStorageMock as any

// Mock matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})
