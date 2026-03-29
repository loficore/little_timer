import type { TimerState } from './apiClient';

export type EventCallback = (state: TimerState) => void;
export type ErrorCallback = (error: Event) => void;

/**
 * SSE 客户端，用于连接到后端 SSE 服务器并接收计时器状态更新
 */
export class SSEClient {
    private eventSource: EventSource | null = null;
    private baseUrl: string;
    private reconnectAttempts = 0;
    private maxReconnectAttempts = 5;
    private reconnectDelay = 1000;
    private onEvent: EventCallback | null = null;
    private onError: ErrorCallback | null = null;

    /**
     * 构造函数，接受 SSE 服务器的基础 URL
     * @param {string} baseUrl SSE 服务器的基础 URL，例如 http://localhost:8000
     */
    constructor(baseUrl: string) {
        this.baseUrl = baseUrl;
    }

    /**
     * 连接到 SSE 服务器并设置事件回调
     * @param {EventCallback} onEvent - 当接收到新的计时器状态时调用的回调函数
     * @param {ErrorCallback} [onError] - 当连接发生错误时调用的可选回调函数
     */
    connect(onEvent: EventCallback, onError?: ErrorCallback): void {
        this.onEvent = onEvent;
        this.onError = onError || null;
        this.createConnection();
    }

    private createConnection(): void {
        this.eventSource = new EventSource(`${this.baseUrl}/api/events`);

        this.eventSource.onopen = () => {
            console.log('SSE connection opened');
            this.reconnectAttempts = 0;
        };

        this.eventSource.onmessage = (event: MessageEvent) => {
            const dataStr = String(event.data);
            if (dataStr.startsWith(':')) {
                console.log('SSE heartbeat received');
                return;
            }
            console.log('SSE received:', dataStr);
            try {
                const data = JSON.parse(dataStr) as Partial<TimerState>;
                if (this.onEvent && data && Object.keys(data).length > 0) {
                    this.onEvent(data as TimerState);
                }
            } catch (e) {
                console.error('Failed to parse SSE data:', e);
            }
        };

        this.eventSource.addEventListener('ping', () => {
            console.log('SSE ping received');
        });

        this.eventSource.onerror = (error: Event) => {
            console.error('SSE connection error:', error);
            
            if (this.onError) {
                this.onError(error);
            }

            this.handleReconnect();
        };
    }

    private handleReconnect(): void {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('Max reconnection attempts reached');
            this.close();
            return;
        }

        this.reconnectAttempts++;
        const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
        
        console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})...`);
        
        setTimeout(() => {
            this.createConnection();
        }, delay);
    }

    /**
     * 关闭 SSE 连接
     */
    close(): void {
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
        }
        this.reconnectAttempts = 0;
    }

    /**
     * 检查 SSE 连接是否处于打开状态
     * @returns {boolean} 如果连接打开则返回 true，否则返回 false
     */
    isConnected(): boolean {
        return this.eventSource !== null && this.eventSource.readyState === EventSource.OPEN;
    }
}
