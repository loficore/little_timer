import { useState, useEffect } from "preact/hooks";
import { HomePage } from "./Homepage.tsx";
import { SettingsPage } from "./Settings.tsx";
import {
  ErrorNotification,
  OfflineModeIndicator,
} from "./components/ErrorNotification.tsx";
import {
  initWebuiManager,
  getConnectionState,
  ConnectionState,
} from "./utils/webui-manager.ts";

export const App = () => {
  const [currentPage, setCurrentPage] = useState<"home" | "settings">("home");
  const [isOffline, setIsOffline] = useState(false);

  // 初始化 WebUI 管理器
  useEffect(() => {
    initWebuiManager();
  }, []);

  // 监听连接状态变化
  useEffect(() => {
    const checkConnection = () => {
      const state = getConnectionState();
      setIsOffline(state !== ConnectionState.CONNECTED);
    };

    const interval = setInterval(checkConnection, 2000);
    return () => clearInterval(interval);
  }, []);

  return (
    <>
      <ErrorNotification visible={true} autoHideDuration={0} />
      <OfflineModeIndicator show={isOffline} />

      {currentPage === "home" && (
        <HomePage onSettingsClick={() => setCurrentPage("settings")} />
      )}
      {currentPage === "settings" && (
        <SettingsPage onBackClick={() => setCurrentPage("home")} />
      )}
    </>
  );
};
