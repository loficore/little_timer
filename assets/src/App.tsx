import { useEffect, useState } from 'react';
import './App.css';
import { logInfo, logSuccess, logError } from './utils/logger';

const Mode = {
  Countdown: '倒计时模式',
  Stopwatch: '正计时模式',
  WorldClock: '世界时钟模式',
} as const;

export type Mode = (typeof Mode)[keyof typeof Mode];

// 声明全局 webui 类型
declare global {
  interface Window {
    webui?: {
      call: (functionName: string, ...args: any[]) => void;
    };
  }
}

export const App = () => {
  const [time, setTime] = useState('25:00:00');
  const [mode, setMode] = useState(Mode.Countdown);

  useEffect(() => {
    logSuccess('✅ React 应用已加载，准备就绪');
    
    // 检查 webui 对象是否存在
    if (typeof window.webui !== 'undefined') {
      logSuccess('✅ webui 对象已加载');
    } else {
      logError('❌ webui 对象未加载！这可能是一个问题');
    }

    // 设置全局事件处理函数
    (window as any).webuiEvent = (event: any) => {
      logInfo('收到来自后端的事件: ' + event.function);
      
      if (event.function === 'update_time') {
        setTime(event.data);
        logInfo('⏱️ 时间已更新: ' + event.data);
      } else if (event.function === 'update_mode') {
        setMode(event.data);
        logInfo('🔄 模式已更新: ' + event.data);
      }
    };

    logInfo('初始化完成，等待用户交互...');
  }, []);

  const handleStart = () => {
    logInfo('🚀 "开始"按钮被点击');
    try {
      if (typeof window.webui === 'undefined') {
        logError('❌ webui 对象未定义！');
        return;
      }
      logInfo('✓ webui 对象存在，准备调用 start 函数');
      window.webui.call('start');
      logSuccess('✓ webui.call("start") 调用成功');
    } catch (e) {
      logError('❌ 调用 start 时发生错误: ' + (e as Error).message);
      console.error(e);
    }
  };

  const handlePause = () => {
    logInfo('⏸️ "暂停"按钮被点击');
    try {
      if (typeof window.webui === 'undefined') {
        logError('❌ webui 对象未定义！');
        return;
      }
      logInfo('✓ webui 对象存在，准备调用 pause 函数');
      window.webui.call('pause');
      logSuccess('✓ webui.call("pause") 调用成功');
    } catch (e) {
      logError('❌ 调用 pause 时发生错误: ' + (e as Error).message);
      console.error(e);
    }
  };

  const handleReset = () => {
    logInfo('🔄 "重置"按钮被点击');
    try {
      if (typeof window.webui === 'undefined') {
        logError('❌ webui 对象未定义！');
        return;
      }
      logInfo('✓ webui 对象存在，准备调用 reset 函数');
      window.webui.call('reset');
      logSuccess('✓ webui.call("reset") 调用成功');
    } catch (e) {
      logError('❌ 调用 reset 时发生错误: ' + (e as Error).message);
      console.error(e);
    }
  };

  //切换模式
  const handleModeChange = (newMode: Mode) => {
    try {
      if (typeof window.webui === 'undefined') {
        logError('❌ webui 对象未定义！');
        return;
      }
      logInfo(`✓ webui 对象存在，准备调用 change_mode 函数，参数: ${newMode}`);
      window.webui.call('change_mode', newMode);
      logSuccess('✓ webui.call("change_mode") 调用成功');
    } catch (e) {
      logError('❌ 调用 change_mode 时发生错误: ' + (e as Error).message);
      console.error(e);
    }
  };

  return (
    <div className="container">
      <h1>Little Timer</h1>
      <div className="time" id="time">{time}</div>
      <div className="controls">
        <button onClick={handleStart}>开始</button>
        <button onClick={handlePause}>暂停</button>
        <button onClick={handleReset}>重置</button>
      </div>
      <div className="mode-indicator" id="mode">{mode}</div>
      <div className="mode-controls">
        <button onClick={() => handleModeChange(Mode.Countdown)}>倒计时模式</button>
        <button onClick={() => handleModeChange(Mode.Stopwatch)}>正计时模式</button>
        <button onClick={() => handleModeChange(Mode.WorldClock)}>世界时钟模式</button>
      </div>
    </div>
  );
}

export default App;