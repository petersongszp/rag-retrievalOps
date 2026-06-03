'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { AuthStatusScreen } from '@/components/auth/auth-status-screen';
import { useAuth } from '@/services/auth/store';

export default function RootPage() {
  const router = useRouter();
  const { status } = useAuth();

  useEffect(() => {
    if (status === 'authenticated') {
      router.replace('/dashboard');
    } else if (status === 'anonymous') {
      router.replace('/login');
    }
  }, [router, status]);

  return (
    <AuthStatusScreen
      title="正在进入管理台"
      description="请稍候，我们正在根据当前会话为你选择正确的入口。"
    />
  );
}
