'use client';

import { useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/services/auth/store';
import { AuthStatusScreen } from './auth-status-screen';

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const { status } = useAuth();

  useEffect(() => {
    if (status === 'anonymous') {
      const next = pathname && pathname !== '/' ? `?next=${encodeURIComponent(pathname)}` : '';
      router.replace(`/login${next}`);
    }
  }, [pathname, router, status]);

  if (status === 'loading') {
    return (
      <AuthStatusScreen
        title="正在恢复会话"
        description="我们正在验证你的身份，受保护内容会在验证完成后再显示。"
      />
    );
  }

  if (status === 'anonymous') {
    return (
      <AuthStatusScreen
        title="正在跳转登录页"
        description="当前页面需要先登录后访问，我们正在带你前往登录页。"
      />
    );
  }

  return <>{children}</>;
}
