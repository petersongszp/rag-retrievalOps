'use client';

import React, { useState } from 'react';
import { Modal, Select, Button, message, Alert } from 'antd';
import { USER_API } from '@/config/api';

export interface FailoverModel {
    id: number;
    name: string;
}

export interface FailoverData {
    failed_model_name: string;
    error_reason: string;
    available_models: FailoverModel[];
}

interface FailoverModalProps {
    open: boolean;
    data: FailoverData | null;
    onSuccess: () => void;
    onCancel: () => void;
}

const FailoverModal: React.FC<FailoverModalProps> = ({ open, data, onSuccess, onCancel }) => {
    const [selectedModelId, setSelectedModelId] = useState<number | undefined>(undefined);
    const [loading, setLoading] = useState(false);

    // When modal is newly opened or data changes, try to auto-select the first available model
    React.useEffect(() => {
        if (open && data?.available_models && data.available_models.length > 0) {
            setSelectedModelId(data.available_models[0].id);
        } else {
            setSelectedModelId(undefined);
        }
    }, [open, data]);

    const handleSwitch = async () => {
        if (!selectedModelId) {
            message.warning('请选择一个备用模型');
            return;
        }

        setLoading(true);
        try {
            const token = localStorage.getItem('token');
            if (!token) {
                message.error('未绑定登录态，请重新登录');
                setLoading(false);
                return;
            }

            // API call to switch models
            const response = await fetch(USER_API.SWITCH_MODEL, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    old_model_id: 0, // Using 0 to disable all old models safety-net implemented in backend
                    new_model_id: selectedModelId,
                }),
            });

            if (!response.ok) {
                throw new Error('网络请求失败');
            }

            const result = await response.json();
            if (result.code !== 200 && result.code !== 0) {
                throw new Error(result.message || result.msg || '切换失败');
            }

            message.success('模型切换成功，面试即将继续...');
            onSuccess();
        } catch (error: any) {
            console.error('[FailoverModal] Switch Error:', error);
            message.error(error.message || '切换模型失败，请稍后重试');
        } finally {
            setLoading(false);
        }
    };

    return (
        <Modal
            title="大模型连接断开"
            open={open}
            onCancel={onCancel}
            footer={[
                <Button key="cancel" onClick={onCancel} disabled={loading}>
                    取消/回首页
                </Button>,
                <Button
                    key="submit"
                    type="primary"
                    loading={loading}
                    onClick={handleSwitch}
                    disabled={!selectedModelId}
                >
                    确认切换
                </Button>,
            ]}
            maskClosable={false}
            closable={!loading}
            centered
        >
            <div className="flex flex-col gap-4 py-2">
                <Alert
                    type="error"
                    message={`当前模型 ${data?.failed_model_name || ''} 失去响应`}
                    description={
                        <div className="text-xs mt-1 text-slate-500 overflow-hidden text-ellipsis max-h-24">
                            报错信息: {data?.error_reason || '未知错误'}
                        </div>
                    }
                    showIcon
                />
                <div className="text-sm text-slate-700 mt-2">
                    发现您可以使用的备用大模型，请选择一个进行快速切换并恢复会话：
                </div>
                <Select
                    value={selectedModelId}
                    onChange={(val) => setSelectedModelId(val)}
                    placeholder="请选择大模型"
                    className="w-full"
                    size="large"
                    options={data?.available_models?.map(m => ({
                        label: m.name,
                        value: m.id
                    })) || []}
                />
            </div>
        </Modal>
    );
};

export default FailoverModal;
