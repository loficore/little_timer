import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { HomePage } from "../Homepage";

vi.mock("../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    getState: vi.fn().mockResolvedValue({
      time: 1500,
      mode: "countdown",
      is_running: false,
      is_finished: false,
      in_rest: false,
      loop_remaining: null,
      loop_total: null,
      rest_remaining: 0,
      timezone: 8,
      habit_id: null,
      elapsed: 0,
    }),
    getHabitSets: vi.fn().mockResolvedValue([]),
    getHabits: vi.fn().mockResolvedValue([]),
    startTimer: vi.fn().mockResolvedValue({}),
    pauseTimer: vi.fn().mockResolvedValue({}),
    resetTimer: vi.fn().mockResolvedValue({}),
    startRest: vi.fn().mockResolvedValue({}),
  })),
}));

vi.mock("../utils/sseClient", () => ({
  SSEClient: vi.fn(() => ({
    connect: vi.fn(),
    close: vi.fn(),
  })),
}));

vi.mock("../utils/audio", () => ({
  audioEngine: {
    setPreferences: vi.fn(),
    playFinish: vi.fn(),
  },
  loadAudioPreferences: vi.fn(() => ({
    sound_enabled: true,
    sound_tick: true,
    sound_finish: true,
    sound_volume: 80,
  })),
}));

vi.mock("../utils/logger", () => ({
  logInfo: vi.fn(),
  logSuccess: vi.fn(),
  logError: vi.fn(),
}));

vi.mock("../utils/formatters", () => ({
  formatDuration: vi.fn((seconds: number) => {
    const hours = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    return `${hours.toString().padStart(2, "0")}:${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  }),
  getToday: vi.fn(() => "2024-01-15"),
}));

vi.mock("../utils/i18n", () => ({
  t: (key: string, params?: Record<string, unknown>) => {
    const translations: Record<string, string> = {
      "common.app_name": "小计时器",
      "habit.no_sets": "暂无习惯集",
      "habit.no_habits": "暂无习惯",
      "habit.create_set": "创建习惯集",
      "habit.add_habit": "添加习惯",
      "habit.habit_list": "习惯列表",
      "timer.goal": "目标",
      "common.minutes": "分钟",
      "modal.rest_5min": "休息 5 分钟",
      "connection.disconnected": "连接已断开",
      "common.version": "版本",
    };
    let result = translations[key] || key;
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        result = result.replace(`{${k}}`, String(v));
      });
    }
    return result;
  },
}));

vi.mock("../components/Header", () => ({
  Header: ({ title, showBack, showStats, onBackClick, onStatsClick }: any) => (
    <div data-testid="header">
      <span data-testid="header-title">{title}</span>
      {showBack && <button onClick={onBackClick}>Back</button>}
      {showStats && <button onClick={onStatsClick}>Stats</button>}
    </div>
  ),
}));

vi.mock("../components/TimeDisplay", () => ({
  TimeDisplay: ({ time, isRunning }: { time: string; isRunning: boolean }) => (
    <div data-testid="time-display">{time}</div>
  ),
}));

vi.mock("../components/ControlPanel", () => ({
  ControlPanel: ({ isRunning, onStart, onPause, onReset }: any) => (
    <div data-testid="control-panel">
      <button onClick={onStart}>Start</button>
      <button onClick={onPause}>Pause</button>
      <button onClick={onReset}>Reset</button>
    </div>
  ),
}));

vi.mock("../components/HabitModal", () => ({
  HabitModal: ({ isOpen, mode, setId, onClose, onSuccess }: any) => {
    if (!isOpen) return null;
    return (
      <div data-testid="habit-modal">
        <span data-testid="modal-mode">{mode}</span>
        <button onClick={onClose}>Close</button>
        <button onClick={onSuccess}>Success</button>
      </div>
    );
  },
}));

describe("HomePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  describe("初始渲染", () => {
    it("应该渲染首页", () => {
      const { container } = render(<HomePage />);
      expect(container.querySelector(".flex.flex-col")).toBeTruthy();
    });

    it("应该显示默认标题", () => {
      render(<HomePage />);
      expect(screen.getByTestId("header-title")).toBeTruthy();
      expect(screen.getByTestId("header-title").textContent).toBe("小计时器");
    });
  });

  describe("习惯集列表", () => {
    it("没有习惯集时应该显示空状态", async () => {
      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByText("暂无习惯集")).toBeTruthy();
      });
    });

    it("有习惯集时应该显示习惯集列表", async () => {
      const mockSets = [
        { id: 1, name: "学习", description: "学习习惯", color: "#6366f1" },
      ];

      const { getAPIClient } = await import("../utils/apiClientSingleton");
      vi.mocked(getAPIClient).mockReturnValue({
        getState: vi.fn().mockResolvedValue({
          time: 1500,
          mode: "countdown",
          is_running: false,
          is_finished: false,
          in_rest: false,
          loop_remaining: null,
          loop_total: null,
          rest_remaining: 0,
          timezone: 8,
          habit_id: null,
          elapsed: 0,
        }),
        getHabitSets: vi.fn().mockResolvedValue(mockSets),
        getHabits: vi.fn().mockResolvedValue([]),
        startTimer: vi.fn().mockResolvedValue({}),
        pauseTimer: vi.fn().mockResolvedValue({}),
        resetTimer: vi.fn().mockResolvedValue({}),
        startRest: vi.fn().mockResolvedValue({}),
      });

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByText("学习")).toBeTruthy();
      });
    });

    it("点击习惯集应该触发 onSetClick", async () => {
      const onSetClick = vi.fn();
      const mockSets = [{ id: 1, name: "学习", description: "学习习惯", color: "#6366f1" }];

      const { getAPIClient } = await import("../utils/apiClientSingleton");
      vi.mocked(getAPIClient).mockReturnValue({
        getState: vi.fn().mockResolvedValue({
          time: 1500,
          mode: "countdown",
          is_running: false,
          is_finished: false,
          in_rest: false,
          loop_remaining: null,
          loop_total: null,
          rest_remaining: 0,
          timezone: 8,
          habit_id: null,
          elapsed: 0,
        }),
        getHabitSets: vi.fn().mockResolvedValue(mockSets),
        getHabits: vi.fn().mockResolvedValue([]),
        startTimer: vi.fn().mockResolvedValue({}),
        pauseTimer: vi.fn().mockResolvedValue({}),
        resetTimer: vi.fn().mockResolvedValue({}),
        startRest: vi.fn().mockResolvedValue({}),
      });

      render(<HomePage onSetClick={onSetClick} />);

      await waitFor(() => {
        expect(screen.getByText("学习")).toBeTruthy();
      });

      const setCard = screen.getByText("学习").closest(".cursor-pointer");
      if (setCard) {
        fireEvent.click(setCard);
      }

      expect(onSetClick).toHaveBeenCalledWith(1);
    });
  });

  describe("习惯列表 (selectedSetId)", () => {
    it("没有习惯时应该显示空状态", async () => {
      const mockSets = [{ id: 1, name: "学习", description: "学习习惯", color: "#6366f1" }];

      const { getAPIClient } = await import("../utils/apiClientSingleton");
      vi.mocked(getAPIClient).mockReturnValue({
        getState: vi.fn().mockResolvedValue({
          time: 1500,
          mode: "countdown",
          is_running: false,
          is_finished: false,
          in_rest: false,
          loop_remaining: null,
          loop_total: null,
          rest_remaining: 0,
          timezone: 8,
          habit_id: null,
          elapsed: 0,
        }),
        getHabitSets: vi.fn().mockResolvedValue(mockSets),
        getHabits: vi.fn().mockResolvedValue([]),
        startTimer: vi.fn().mockResolvedValue({}),
        pauseTimer: vi.fn().mockResolvedValue({}),
        resetTimer: vi.fn().mockResolvedValue({}),
        startRest: vi.fn().mockResolvedValue({}),
      });

      render(<HomePage selectedSetId={1} />);

      await waitFor(() => {
        expect(screen.getByText("暂无习惯")).toBeTruthy();
      });
    });

    it("点击习惯应该触发 onHabitClick", async () => {
      const onHabitClick = vi.fn();
      const mockSets = [{ id: 1, name: "学习", description: "学习习惯", color: "#6366f1" }];
      const mockHabits = [{ id: 1, set_id: 1, name: "背单词", goal_seconds: 1500, color: "#22c55e" }];

      const { getAPIClient } = await import("../utils/apiClientSingleton");
      vi.mocked(getAPIClient).mockReturnValue({
        getState: vi.fn().mockResolvedValue({
          time: 1500,
          mode: "countdown",
          is_running: false,
          is_finished: false,
          in_rest: false,
          loop_remaining: null,
          loop_total: null,
          rest_remaining: 0,
          timezone: 8,
          habit_id: null,
          elapsed: 0,
        }),
        getHabitSets: vi.fn().mockResolvedValue(mockSets),
        getHabits: vi.fn().mockResolvedValue(mockHabits),
        startTimer: vi.fn().mockResolvedValue({}),
        pauseTimer: vi.fn().mockResolvedValue({}),
        resetTimer: vi.fn().mockResolvedValue({}),
        startRest: vi.fn().mockResolvedValue({}),
      });

      render(<HomePage selectedSetId={1} onHabitClick={onHabitClick} />);

      await waitFor(() => {
        expect(screen.getByText("背单词")).toBeTruthy();
      });

      const habitCard = screen.getByText("背单词").closest(".cursor-pointer");
      if (habitCard) {
        fireEvent.click(habitCard);
      }

      expect(onHabitClick).toHaveBeenCalledWith(
        expect.objectContaining({ id: 1, name: "背单词" })
      );
    });
  });

  describe("计时页面 (selectedHabit)", () => {
    it("应该显示习惯名称", () => {
      const mockHabit = { id: 1, set_id: 1, name: "背单词", goal_seconds: 1500, color: "#22c55e" };

      render(<HomePage selectedHabit={mockHabit} />);

      const habitName = screen.getAllByText("背单词")[1];
      expect(habitName).toBeTruthy();
    });

    it("应该显示目标时间", () => {
      const mockHabit = { id: 1, set_id: 1, name: "背单词", goal_seconds: 1500, color: "#22c55e" };

      render(<HomePage selectedHabit={mockHabit} />);

      expect(screen.getByText(/目标/)).toBeTruthy();
    });

    it("应该显示 ControlPanel", () => {
      const mockHabit = { id: 1, set_id: 1, name: "背单词", goal_seconds: 1500, color: "#22c55e" };

      render(<HomePage selectedHabit={mockHabit} />);

      expect(screen.getByTestId("control-panel")).toBeTruthy();
    });
  });

  describe("导航", () => {
    it("选中习惯集时应该显示返回按钮", async () => {
      const mockSets = [{ id: 1, name: "学习", description: "学习习惯", color: "#6366f1" }];

      const { getAPIClient } = await import("../utils/apiClientSingleton");
      vi.mocked(getAPIClient).mockReturnValue({
        getState: vi.fn().mockResolvedValue({
          time: 1500,
          mode: "countdown",
          is_running: false,
          is_finished: false,
          in_rest: false,
          loop_remaining: null,
          loop_total: null,
          rest_remaining: 0,
          timezone: 8,
          habit_id: null,
          elapsed: 0,
        }),
        getHabitSets: vi.fn().mockResolvedValue(mockSets),
        getHabits: vi.fn().mockResolvedValue([]),
        startTimer: vi.fn().mockResolvedValue({}),
        pauseTimer: vi.fn().mockResolvedValue({}),
        resetTimer: vi.fn().mockResolvedValue({}),
        startRest: vi.fn().mockResolvedValue({}),
      });

      const onBackClick = vi.fn();
      render(<HomePage selectedSetId={1} onBackClick={onBackClick} />);

      const backButton = screen.getByRole("button", { name: /back/i });
      expect(backButton).toBeTruthy();

      fireEvent.click(backButton);
      expect(onBackClick).toHaveBeenCalled();
    });

    it("未选中习惯时应该显示 Stats 按钮", async () => {
      const onStatsClick = vi.fn();

      const { getAPIClient } = await import("../utils/apiClientSingleton");
      vi.mocked(getAPIClient).mockReturnValue({
        getState: vi.fn().mockResolvedValue({
          time: 1500,
          mode: "countdown",
          is_running: false,
          is_finished: false,
          in_rest: false,
          loop_remaining: null,
          loop_total: null,
          rest_remaining: 0,
          timezone: 8,
          habit_id: null,
          elapsed: 0,
        }),
        getHabitSets: vi.fn().mockResolvedValue([]),
        getHabits: vi.fn().mockResolvedValue([]),
        startTimer: vi.fn().mockResolvedValue({}),
        pauseTimer: vi.fn().mockResolvedValue({}),
        resetTimer: vi.fn().mockResolvedValue({}),
        startRest: vi.fn().mockResolvedValue({}),
      });

      render(<HomePage onStatsClick={onStatsClick} />);

      const statsButton = screen.getByRole("button", { name: /stats/i });
      expect(statsButton).toBeTruthy();

      fireEvent.click(statsButton);
      expect(onStatsClick).toHaveBeenCalled();
    });
  });

  describe("HabitModal", () => {
    it("应该显示创建习惯集按钮", async () => {
      render(<HomePage />);

      const addButton = screen.getByRole("button", { name: /创建习惯集/i });
      expect(addButton).toBeTruthy();
    });

    it("点击创建习惯集应该打开弹窗", async () => {
      render(<HomePage />);

      const addButton = screen.getByRole("button", { name: /创建习惯集/i });
      fireEvent.click(addButton);

      await waitFor(() => {
        expect(screen.getByTestId("habit-modal")).toBeTruthy();
        expect(screen.getByTestId("modal-mode").textContent).toBe("set");
      });
    });

    it("点击关闭按钮应该关闭弹窗", async () => {
      render(<HomePage />);

      const addButton = screen.getByRole("button", { name: /创建习惯集/i });
      fireEvent.click(addButton);

      await waitFor(() => {
        expect(screen.getByTestId("habit-modal")).toBeTruthy();
      });

      const closeButton = screen.getByRole("button", { name: /close/i });
      fireEvent.click(closeButton);

      await waitFor(() => {
        expect(screen.queryByTestId("habit-modal")).toBeNull();
      });
    });
  });
});