import type { ComponentChildren, VNode } from "preact";

interface Tab {
  id: string;
  label: string;
  icon?: VNode | undefined;
}

interface TabPanelProps {
  tabs: Tab[];
  activeTab: string;
  onTabChange: (tabId: string) => void;
  children: ComponentChildren;
  isAnimated?: boolean;
}

export const TabPanel = ({
  tabs,
  activeTab,
  onTabChange,
  children,
  isAnimated = false,
}: TabPanelProps) => {
  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      <div className={`tabs tabs-boxed bg-base-300 p-1 ${isAnimated ? "animate-slideUp" : ""}`}>
        {tabs.map((tab) => (
          <a
            key={tab.id}
            onClick={() => onTabChange(tab.id)}
            className={`tab flex-1 gap-2 ${activeTab === tab.id ? "tab-active" : ""}`}
          >
            {tab.icon && <span className="w-4 h-4">{tab.icon}</span>}
            <span className="hidden sm:inline">{tab.label}</span>
          </a>
        ))}
      </div>
      <div
        className={`flex-1 overflow-y-auto p-4 flex flex-col gap-4 ${
          isAnimated ? "animate-slideUp" : ""
        }`}
        style={
          isAnimated
            ? { animationDelay: "0.25s", animationFillMode: "both" }
            : {}
        }
      >
        {children}
      </div>
    </div>
  );
};
