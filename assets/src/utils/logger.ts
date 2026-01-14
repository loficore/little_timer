// 日志辅助函数

export const logInfo = (msg: string) => {
  console.log('%c[前端] ' + new Date().toLocaleTimeString() + ' - ' + msg, 
    'color: #667eea; font-weight: bold;');
};

export const logSuccess = (msg: string) => {
  console.log('%c[前端] ' + new Date().toLocaleTimeString() + ' - ' + msg, 
    'color: #28a745; font-weight: bold;');
};

export const logError = (msg: string) => {
  console.error('%c[前端] ' + new Date().toLocaleTimeString() + ' - ' + msg, 
    'color: #dc3545; font-weight: bold;');
};
