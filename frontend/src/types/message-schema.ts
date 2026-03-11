// 角色类型定义（与后端保持一致）
export type RoleType =
    | 'main_interviewer'
    | 'tech_interviewer'
    | 'project_interviewer'
    | 'candidate'
    | 'system';

// 动作类型
export type ActionType = 'thinking' | 'speaking' | 'waiting';

// 消息状态
export type MessageStatus = 'pending' | 'streaming' | 'complete' | 'error';

// 结构化消息体（Message Schema）
export interface StructuredMessage {
    message_id: string;
    timestamp: number;
    role: RoleType;
    role_name: string;
    role_avatar: string;
    content: string;
    action_type: ActionType;
    status: MessageStatus;
    metadata?: Record<string, any>;
}

// SSE 事件类型
export interface SSEEvent {
    type: string;
    [key: string]: any;
}

// 角色配置
export interface RoleConfig {
    roleType: RoleType;
    roleName: string;
    avatarSeed: string;
    colorClass: string;
    borderClass: string;
}

// 获取角色配置
export const getRoleConfig = (roleType: RoleType): RoleConfig => {
    const configs: Record<RoleType, RoleConfig> = {
        main_interviewer: {
            roleType: 'main_interviewer',
            roleName: '主面试官',
            avatarSeed: 'interviewer-main-v2',
            colorClass: 'text-orange-500',
            borderClass: 'border-orange-200',
        },
        tech_interviewer: {
            roleType: 'tech_interviewer',
            roleName: '技术面试官',
            avatarSeed: 'interviewer-tech-v2',
            colorClass: 'text-blue-500',
            borderClass: 'border-blue-200',
        },
        project_interviewer: {
            roleType: 'project_interviewer',
            roleName: '项目面试官',
            avatarSeed: 'interviewer-project-v2',
            colorClass: 'text-emerald-500',
            borderClass: 'border-emerald-200',
        },
        candidate: {
            roleType: 'candidate',
            roleName: '候选人',
            avatarSeed: 'user-candidate-v2',
            colorClass: 'text-blue-600',
            borderClass: 'border-blue-100',
        },
        system: {
            roleType: 'system',
            roleName: '系统',
            avatarSeed: 'system-v2',
            colorClass: 'text-slate-400',
            borderClass: 'border-slate-200',
        },
    };

    return configs[roleType] || configs.system;
};

// 从内容中检测角色（兼容旧的基于前缀的识别方式）
export const detectRoleFromContent = (content: string): RoleType => {
    // 优先检查副面试官/技术面试官关键词
    if (
        content.includes('副面试官') ||
        content.includes('技术副面') ||
        content.includes('技术面试官') ||
        content.includes('我是技术面试官：')
    ) {
        return 'tech_interviewer';
    }

    // 检查项目面试官
    if (
        content.includes('项目面试官') ||
        content.includes('项目负责人') ||
        content.includes('项目专家') ||
        content.includes('我是项目面试官：')
    ) {
        return 'project_interviewer';
    }

    // 检查主面试官
    if (content.includes('主面试官') || content.includes('我是主面试官：')) {
        return 'main_interviewer';
    }

    // 默认为主面试官
    return 'main_interviewer';
};

// SSE 流解析器（支持多路复用）
export class SSEStreamParser {
    private buffer: string = '';

    // 解析流数据块
    parse(chunk: string): SSEEvent[] {
        this.buffer += chunk;
        const events: SSEEvent[] = [];

        // SSE 格式：事件之间用 \n\n 分隔
        const blocks = this.buffer.split('\n\n');

        // 最后一个可能是不完整的，保留在 buffer 中
        this.buffer = blocks.pop() || '';

        for (const block of blocks) {
            if (!block.trim()) continue;

            try {
                const event = this.parseBlock(block);
                if (event) {
                    events.push(event);
                }
            } catch (e) {
                console.error('Failed to parse SSE block:', block, e);
            }
        }

        return events;
    }

    // 解析单个事件块
    private parseBlock(block: string): SSEEvent | null {
        const lines = block.split('\n');
        let eventType = 'message';
        let data = '';

        for (const line of lines) {
            if (line.startsWith('event:')) {
                eventType = line.substring(6).trim();
            } else if (line.startsWith('data:')) {
                data = line.substring(5).trim();
            }
        }

        if (!data) return null;

        try {
            const payload = JSON.parse(data);
            return { type: eventType, ...payload };
        } catch (e) {
            console.error('Failed to parse JSON data:', data, e);
            return null;
        }
    }

    // 重置解析器
    reset() {
        this.buffer = '';
    }
}

// 会话历史项
export interface ConversationItem {
    type: 'question' | 'answer';
    content: string;
    role?: RoleType;
    roleConfig?: RoleConfig;
    index?: number;
    timestamp: number;
    actionType?: ActionType;
}

