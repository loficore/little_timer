export interface TimerState {
    time: number;
    mode: 'countdown' | 'stopwatch' | 'world_clock';
    is_running: boolean;
    is_finished: boolean;
    in_rest: boolean;
    loop_remaining: number;
    loop_total: number;
    rest_remaining: number;
    timezone: number;
}

export interface Settings {
    basic: {
        timezone: number;
        language: string;
        default_mode: string;
        theme_mode: string;
    };
    countdown: object;
    stopwatch: object;
    world_clock: object;
}

/**
 * API 客户端，用于与后端 API 进行交互
 */
export class APIClient {
    private baseUrl: string;

    /**
     * 构造函数，接受 API 基础 URL
     * @param {string} baseUrl API 基础 URL，例如 http://localhost:8000
     */
    constructor(baseUrl: string) {
        this.baseUrl = baseUrl;
    }

    /**
     * 获取当前计时器状态
     * @returns {Promise<TimerState>} 返回一个 Promise，解析为 TimerState 对象
     */
    async getState(): Promise<TimerState> {
        const response = await fetch(`${this.baseUrl}/api/state`);
        if (!response.ok) {
            throw new Error(`Error fetching state: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 开始计时器
     * @returns {Promise<void>} 返回一个 Promise，表示操作完成
     */
    async startTimer(): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/start`, { method: 'POST' });
        if (!response.ok) {
            throw new Error(`Error starting timer: ${response.statusText}`);
        }
    }

    /**
     * 暂停计时器
     * @returns {Promise<void>} 返回一个 Promise，表示操作完成
     */
    async pauseTimer(): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/pause`, { method: 'POST' });
        if (!response.ok) {
            throw new Error(`Error pausing timer: ${response.statusText}`);
        }
    }

    /**
     * 重置计时器
     */
    async resetTimer(): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/reset`, { method: 'POST' });
        if (!response.ok) {
            throw new Error(`Error resetting timer: ${response.statusText}`);
        }
    }

    /**
     * 切换计时器模式
     * @param {'countdown' | 'stopwatch' | 'world_clock'} mode 目标模式
     */
    async changeMode(mode: 'countdown' | 'stopwatch' | 'world_clock'): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/mode`, {
            method: 'POST',
            body: mode
        });
        if (!response.ok) {
            throw new Error(`Error changing mode: ${response.statusText}`);
        }
    }

    /**
     * 获取设置
     * @returns {Promise<Settings>} 返回一个 Promise，解析为 Settings 对象
     */
    async getSettings(): Promise<Settings> {
        const response = await fetch(`${this.baseUrl}/api/settings`);
        if (!response.ok) {
            throw new Error(`Error fetching settings: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 更新设置
     * @param {object} settings 要更新的设置对象
     * @returns {Promise<void>} 返回一个 Promise，表示操作完成
     */
    async updateSettings(settings: object): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/settings`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(settings)
        });
        if (!response.ok) {
            throw new Error(`Error updating settings: ${response.statusText}`);
        }
    }

    /**
     * 获取预设列表
     * @returns {Promise<object>} 返回一个 Promise，解析为预设列表对象
     */
    async getPresets(): Promise<object> {
        const response = await fetch(`${this.baseUrl}/api/presets`);
        if (!response.ok) {
            throw new Error(`Error fetching presets: ${response.statusText}`);
        }
        return await response.json();
    }
}
