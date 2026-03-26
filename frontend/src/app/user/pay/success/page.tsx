'use client';

import { useEffect, useState } from 'react';
import { Button, Card, message, Spin, Tag, Typography } from 'antd';
import apiClient from '@/services/api/client';
import { PAYMENT_API } from '@/config/api';
import { useAuth } from '@/hooks/useAuth';

const { Title, Text } = Typography;

export default function PaySuccessPage() {
  const { user, isAuthenticated } = useAuth();
  const [orderNo, setOrderNo] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const v = localStorage.getItem('payment_last_order_no');
    setOrderNo(v ? String(v) : null);
  }, []);

  useEffect(() => {
    if (!orderNo) {
      setLoading(false);
      return;
    }
    if (!isAuthenticated || !user) {
      message.warning('登录信息缺失，请重新登录后查看订单状态');
      setLoading(false);
      return;
    }

    let cancelled = false;
    const poll = async () => {
      setLoading(true);
      const maxAttempts = 18; // 約：18 * 3s = 54s
      const intervalMs = 3000;

      for (let i = 0; i < maxAttempts; i++) {
        if (cancelled) return;
        try {
          const res: any = await apiClient.post(PAYMENT_API.ORDER_QUERY, { order_no: orderNo });
          const nextStatus = res?.status ? String(res.status) : null;
          if (nextStatus) setStatus(nextStatus);

          if (nextStatus === 'PAID' || nextStatus === 'FULFILLED') {
            return;
          }
        } catch (e) {
          // webhook 可能还没处理到，允许重试
        }

        await new Promise((r) => setTimeout(r, intervalMs));
      }
    };

    poll().finally(() => {
      if (!cancelled) setLoading(false);
    });

    return () => {
      cancelled = true;
    };
  }, [orderNo, isAuthenticated, user]);

  const isSuccess = status === 'PAID' || status === 'FULFILLED';

  return (
    <div className="min-h-screen bg-slate-50 py-10">
      <div className="container mx-auto px-4">
        <Title level={2} className="text-slate-900 mb-2">
          支付结果
        </Title>
        <Text className="text-slate-500">正在校验订单状态…</Text>

        <div className="mt-8">
          <Card className="rounded-3xl shadow-lg border border-slate-100">
            <div className="flex flex-col gap-3">
              <div>
                <Text className="text-sm text-slate-500">订单号</Text>
                <div className="mt-1 font-mono text-slate-900">{orderNo || '-'}</div>
              </div>

              <div>
                <Text className="text-sm text-slate-500">状态</Text>
                <div className="mt-1">
                  {loading ? (
                    <span className="inline-flex items-center gap-2">
                      <Spin size="small" /> 处理中…
                    </span>
                  ) : (
                    <Tag color={isSuccess ? 'success' : 'default'}>
                      {status ? status : '未知'}
                    </Tag>
                  )}
                </div>
              </div>

              <div className="text-xs text-slate-400">
                说明：支付状态以后台 webhook 校验和订单查询为准。
              </div>

              <div className="mt-2 flex gap-3">
                <Button
                  className="rounded-xl"
                  onClick={() => {
                    window.location.href = '/user/pay';
                  }}
                >
                  返回支付中心
                </Button>
                <Button
                  type="primary"
                  className="bg-blue-600 hover:bg-blue-500 rounded-xl"
                  onClick={() => window.location.reload()}
                  disabled={loading}
                >
                  刷新状态
                </Button>
              </div>
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}

