'use client';

import { useMemo, useState } from 'react';
import { Button, Card, message, Spin, Tag, Typography } from 'antd';
import apiClient from '@/services/api/client';
import { PAYMENT_API } from '@/config/api';
import { useAuth } from '@/hooks/useAuth';

const { Title, Text } = Typography;

export default function PayPage() {
  const { user, isAuthenticated } = useAuth();
  const [loading, setLoading] = useState(false);

  const products = useMemo(
    () => [
      {
        key: 'vip_monthly',
        label: 'VIP Monthly (One-time)',
        provider: 'paypal',
        product_code: 'monthly',
        price_code: 'monthly_9.99',
        amount: 999, // 最小货币单位（示例：$9.99 -> 999 cents）
        currency: 'usd',
        product_name: 'VIP Monthly Pass',
      },
    ],
    [],
  );

  const handleCreateCheckout = async (p: (typeof products)[number]) => {
    if (!isAuthenticated || !user) {
      message.error('Please login first');
      return;
    }

    if (typeof window === 'undefined') return;

    const origin = window.location.origin;
    const success_url = `${origin}/user/pay/success`;
    const cancel_url = `${origin}/user/pay/cancel`;

    setLoading(true);
    try {
      const req = {
        product_code: p.product_code,
        price_code: p.price_code,
        provider: p.provider,
        amount: p.amount,
        currency: p.currency,
        product_name: p.product_name,
        success_url,
        cancel_url,
      };

      const res: any = await apiClient.post(PAYMENT_API.CHECKOUT_CREATE, req);
      if (!res?.checkout_url || !res?.order_no) {
        throw new Error('Payment service returned incomplete data');
      }

      localStorage.setItem('payment_last_order_no', String(res.order_no));
      localStorage.setItem('payment_last_provider', String(p.provider));

      // 跳转到 PayPal approve 页面
      window.location.href = res.checkout_url;
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || 'Payment initiation failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-50 py-10">
      <div className="container mx-auto px-4">
        <Title level={2} className="text-slate-900 mb-2">
          Payment Center
        </Title>
        <Text className="text-slate-500">Select a plan and complete payment via PayPal.</Text>

        <div className="mt-8 grid grid-cols-1 md:grid-cols-2 gap-6">
          {products.map((p) => (
            <Card key={p.key} className="rounded-3xl shadow-lg border border-slate-100" hoverable>
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="flex items-center gap-2">
                    <Text className="text-lg font-bold text-slate-900">{p.label}</Text>
                    <Tag color="blue">{p.provider.toUpperCase()}</Tag>
                  </div>
                  <div className="mt-2 text-sm text-slate-500">
                    Amount: {(p.amount / 100).toFixed(2)} {p.currency.toUpperCase()}
                  </div>
                </div>
                <div className="text-right">
                  <Button
                    type="primary"
                    className="bg-blue-600 hover:bg-blue-500 rounded-xl"
                    onClick={() => handleCreateCheckout(p)}
                    disabled={loading}
                  >
                    {loading ? <Spin size="small" /> : 'Pay Now'}
                  </Button>
                </div>
              </div>
            </Card>
          ))}
        </div>

        <div className="mt-6 text-xs text-slate-400">
          Note: Payment results are verified by backend webhook.
        </div>
      </div>
    </div>
  );
}

