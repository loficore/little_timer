import type { ComponentChildren, VNode } from "preact";

interface Tab {
  id: string;
  label: string;
  icon?: VNode | null | undefined;
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
      <div className={`my-tabs ${isAnimated ? "animate-slideUp" : ""}`}>
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => onTabChange(tab.id)}
            className={`my-tab ${activeTab === tab.id ? "my-tab-active" : ""}`}
          >
            {tab.icon && <span className="w-4 h-4">{tab.icon}</span>}
            <span className="text-[0.72rem] sm:text-sm leading-none">{tab.label}</span>
          </button>
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
