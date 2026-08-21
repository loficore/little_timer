interface Window {
  webui?: {
    call: (functionName: string, ...args: unknown[]) => void;
  };
}
