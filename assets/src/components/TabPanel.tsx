import type { ComponentChildren } from "preact";

interface Tab {
  id: string;
  label: string;
  icon?: string;
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
      <div
        className={`flex gap-0 border-b border-border-dark bg-primary-dark px-2 sm:px-4 md:px-8 overflow-x-auto scrollbar-hide ${
          isAnimated ? "animate-slideUp" : ""
        }`}
        style={
          isAnimated
            ? { animationDelay: "0.2s", animationFillMode: "both" }
            : {}
        }
      >
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => onTabChange(tab.id)}
            className={`px-3 sm:px-4 md:px-6 py-3 sm:py-4 font-medium text-xs sm:text-sm whitespace-nowrap flex items-center gap-1 sm:gap-2 transition-all duration-200 border-b-2 hover:scale-105 active:scale-95 min-h-[44px] ${
              activeTab === tab.id
                ? "text-text-primary-dark border-accent-dark"
                : "text-text-secondary-dark border-transparent hover:text-text-primary-dark"
            }`}
          >
            {tab.icon && (
              <span className="text-base sm:text-lg">{tab.icon}</span>
            )}
            <span className="hidden sm:inline">{tab.label}</span>
          </button>
        ))}
      </div>
      <div
        className={`flex-1 overflow-y-auto px-4 sm:px-6 md:px-8 py-4 sm:py-6 md:py-8 flex flex-col gap-4 sm:gap-6 bg-primary-dark ${
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
