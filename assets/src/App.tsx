import { useState } from "preact/hooks";
import { HomePage } from "./Homepage.tsx";
import { SettingsPage } from "./Settings.tsx";
import { ErrorNotification } from "./components/ErrorNotification.tsx";

export const App = () => {
  const [currentPage, setCurrentPage] = useState<"home" | "settings">("home");

  return (
    <>
      <ErrorNotification visible={true} />

      {currentPage === "home" && (
        <HomePage onSettingsClick={() => setCurrentPage("settings")} />
      )}
      {currentPage === "settings" && (
        <SettingsPage onBackClick={() => setCurrentPage("home")} />
      )}
    </>
  );
};
