'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/services/auth/store';
import { AuthStatusScreen } from './auth-status-screen';

export function GuestOnly({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { status } = useAuth();

  useEffect(() => {
    if (status === 'authenticated') {
      router.replace('/dashboard');
    }
  }, [router, status]);

  if (status === 'loading') {
    return (
      <AuthStatusScreen
        title="正在检查登录状态"
        description="请稍候，我们正在确认当前会话。"
      />
    );
  }

  if (status === 'authenticated') {
    return (
      <AuthStatusScreen
        title="正在进入管理台"
        description="已检测到有效会话，正在为你跳转到控制台。"
      />
    );
  }

  return <>{children}</>;
}
