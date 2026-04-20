// 全局类型声明

interface Window {
  webui?: {
    call: (functionName: string, ...args: unknown[]) => void;
  };
}
