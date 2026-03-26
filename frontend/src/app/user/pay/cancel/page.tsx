'use client';

import { useEffect, useState } from 'react';
import { Button, Card, Typography } from 'antd';

const { Title, Text } = Typography;

export default function PayCancelPage() {
  const [orderNo, setOrderNo] = useState<string | null>(null);

  useEffect(() => {
    const v = localStorage.getItem('payment_last_order_no');
    setOrderNo(v ? String(v) : null);
  }, []);

  return (
    <div className="min-h-screen bg-slate-50 py-10">
      <div className="container mx-auto px-4">
        <Title level={2} className="text-slate-900 mb-2">
          支付已取消
        </Title>
        <Text className="text-slate-500">你已取消支付流程。</Text>

        <div className="mt-8">
          <Card className="rounded-3xl shadow-lg border border-slate-100">
            <div className="flex flex-col gap-3">
              <div>
                <Text className="text-sm text-slate-500">订单号</Text>
                <div className="mt-1 font-mono text-slate-900">{orderNo || '-'}</div>
              </div>
              <Button
                className="rounded-xl bg-blue-600 hover:bg-blue-500 w-fit"
                onClick={() => {
                  window.location.href = '/user/pay';
                }}
              >
                返回支付中心
              </Button>
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}

