export interface TimerState {
    time: number;
    elapsed?: number;
    mode: 'countdown' | 'stopwatch';
    is_running: boolean;
    is_finished: boolean;
    in_rest: boolean;
    loop_remaining: number;
    loop_total: number;
    rest_remaining: number;
    timezone: number;
    habit_id?: number;
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
     * @param {number} [habitId] 习惯 ID（可选）
     * @returns {Promise<{habit_id: number | null}>} 返回一个 Promise，表示操作完成
     */
    async startTimer(habitId?: number): Promise<{ habit_id: number | null }> {
        const body = habitId ? JSON.stringify({ habit_id: habitId }) : '';
        const response = await fetch(`${this.baseUrl}/api/start`, {
            method: 'POST',
            headers: body ? { 'Content-Type': 'application/json' } : {},
            body: body || undefined
        });
        if (!response.ok) {
            throw new Error(`Error starting timer: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 开始休息
     * @returns {Promise<{rest_seconds: number}>} 休息时长
     */
    async startRest(): Promise<{ rest_seconds: number }> {
        const response = await fetch(`${this.baseUrl}/api/timer/rest`, { method: 'POST' });
        if (!response.ok) {
            throw new Error(`Error starting rest: ${response.statusText}`);
        }
        return await response.json();
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
     * @param {'countdown' | 'stopwatch'} mode 目标模式
     */
    async changeMode(mode: 'countdown' | 'stopwatch'): Promise<void> {
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

    // === 习惯集 API ===

    /**
     * 获取习惯集列表
     * @returns {Promise<any[]>} 返回一个 Promise，解析为习惯集列表对象
     */
    async getHabitSets(): Promise<any[]> {
        const response = await fetch(`${this.baseUrl}/api/habit-sets`);
        if (!response.ok) {
            throw new Error(`Error fetching habit sets: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 创建习惯集
     * @param {string} name 
     * @param {string} description 
     * @param {string} color 
     * @returns {Promise<any>} 返回一个 Promise，解析为创建的习惯集对象
     */
    async createHabitSet(name: string, description: string, color: string): Promise<any> {
        const response = await fetch(`${this.baseUrl}/api/habit-sets`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, description, color })
        });
        if (!response.ok) {
            throw new Error(`Error creating habit set: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 更新习惯集
     * @param {number} id 习惯集 ID
     * @param {string} name 名称
     * @param {string} description 描述
     * @param {string} color 颜色
     * @returns {Promise<any>}
     */
    async updateHabitSet(id: number, name: string, description: string, color: string): Promise<any> {
        const response = await fetch(`${this.baseUrl}/api/habit-sets/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, description, color })
        });
        if (!response.ok) {
            throw new Error(`Error updating habit set: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 删除习惯集
     * @param {number} id 习惯集 ID
     * @returns {Promise<any>}
     */
    async deleteHabitSet(id: number): Promise<any> {
        const response = await fetch(`${this.baseUrl}/api/habit-sets/${id}`, {
            method: 'DELETE'
        });
        if (!response.ok) {
            throw new Error(`Error deleting habit set: ${response.statusText}`);
        }
        return await response.json();
    }

    // === 习惯 API ===

    /**
     * 获取习惯列表
     * @returns {Promise<any[]>} 返回一个 Promise，解析为习惯列表对象
     */
    async getHabits(): Promise<any[]> {
        const response = await fetch(`${this.baseUrl}/api/habits`);
        if (!response.ok) {
            throw new Error(`Error fetching habits: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 创建习惯
     * @param {number} setId 习惯集 ID
     * @param {string} name 习惯名称
     * @param {number} goalSeconds 目标时间（秒）
     * @param {string} color 颜色
     * @returns {Promise<any>}
     */
    async createHabit(setId: number, name: string, goalSeconds: number, color: string): Promise<any> {
        const response = await fetch(`${this.baseUrl}/api/habits`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                set_id: setId,
                name,
                goal_seconds: goalSeconds,
                color
            })
        });
        if (!response.ok) {
            throw new Error(`Error creating habit: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 删除习惯
     * @param {number} id 
     * @returns {Promise<any>} 返回一个 Promise，解析为删除的习惯对象
     */
    async deleteHabit(id: number): Promise<any> {
        const response = await fetch(`${this.baseUrl}/api/habits/${id}`, {
            method: 'DELETE'
        });
        if (!response.ok) {
            throw new Error(`Error deleting habit: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 更新习惯
     * @param {number} id 习惯 ID
     * @param {string} name 名称
     * @param {number} goalSeconds 目标时长（秒）
     * @param {string} color 颜色
     * @returns {Promise<any>}
     */
    async updateHabit(id: number, name: string, goalSeconds: number, color: string): Promise<any> {
        const response = await fetch(`${this.baseUrl}/api/habits/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                name,
                goal_seconds: goalSeconds,
                color
            })
        });
        if (!response.ok) {
            throw new Error(`Error updating habit: ${response.statusText}`);
        }
        return await response.json();
    }

    // === 记录 API ===

    /**
     * 创建记录
     * @param {number} habitId 习惯 ID
     * @param {number} durationSeconds 持续时间（秒）
     * @param {number} count 次数
     * @param {string} date 日期
     * @returns {Promise<any>} 返回一个 Promise，解析为创建的记录对象
     */
    async createSession(habitId: number, durationSeconds: number, count: number, date: string): Promise<any> {
        const response = await fetch(`${this.baseUrl}/api/sessions`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                habit_id: habitId,
                duration_seconds: durationSeconds,
                count,
                date
            })
        });
        if (!response.ok) {
            throw new Error(`Error creating session: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 获取记录列表
     * @param {string} date 日期
     * @param {string} startDate 开始日期
     * @param {string} endDate 结束日期
     * @returns {Promise<any[]>} 返回一个 Promise，解析为记录列表对象
     */
    async getSessions(date?: string, startDate?: string, endDate?: string): Promise<any[]> {
        const params = new URLSearchParams();
        if (date) params.set('date', date);
        if (startDate) params.set('start_date', startDate);
        if (endDate) params.set('end_date', endDate);

        const response = await fetch(`${this.baseUrl}/api/sessions?${params.toString()}`);
        if (!response.ok) {
            throw new Error(`Error fetching sessions: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 获取习惯连胜
     * @param {number} habitId 习惯 ID
     * @param {number} goalSeconds 目标时长
     * @returns {Promise<{ habit_id: number; streak: number }>} 返回一个 Promise，解析为包含习惯 ID 和连胜数的对象
     */
    async getHabitStreak(habitId: number, goalSeconds?: number): Promise<{ habit_id: number; streak: number }> {
        const params = new URLSearchParams();
        if (goalSeconds) params.set('goal_seconds', goalSeconds.toString());
        const response = await fetch(`${this.baseUrl}/api/habits/${habitId}/streak?${params.toString()}`);
        if (!response.ok) {
            throw new Error(`Error fetching streak: ${response.statusText}`);
        }
        return await response.json();
    }
}
