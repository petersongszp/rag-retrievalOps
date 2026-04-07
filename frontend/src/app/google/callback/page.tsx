'use client';

import { useSearchParams, useRouter } from 'next/navigation';
import { useEffect, useState, useRef, Suspense } from 'react';
import { Spin, Result, Button } from 'antd';
import apiClient from '@/services/api/client';
import { useAuthStore } from '@/store/authStore';

function GoogleCallbackContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const login = useAuthStore((s) => s.login);
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  const [errorMsg, setErrorMsg] = useState<string>('');
  const didRun = useRef(false);

  useEffect(() => {
    const code = searchParams.get('code');
    if (!code) {
      setErrorMsg('Missing authorization code. Please retry Google login from the login page.');
      setStatus('error');
      return;
    }

    // 防止 React Strict Mode 或依赖变化导致 effect 执行两次，用同一 code 重复请求会报错并先显示"登录失败"
    if (didRun.current) return;
    didRun.current = true;

    (async () => {
      try {
        const res: any = await apiClient.post('/user/google/callback', { code }, { timeout: 30000 });
        const data = res?.data ?? res;
        const token = data?.token || data?.accessToken;
        if (!token) {
          setErrorMsg('Login failed: No token returned');
          setStatus('error');
          return;
        }
        localStorage.setItem('token', token);
        try {
          document.cookie = `token=${token};path=/;max-age=${60 * 60 * 24}`;
        } catch {}
        const user = data?.user;
        if (user) {
          localStorage.setItem('user', JSON.stringify(user));
          login({
            id: String(user.id ?? user.ID ?? ''),
            username: user.username,
            name: user.nickname ?? user.name ?? user.username,
            email: user.email ?? '',
            avatar: user.avatar,
          });
        } else {
          login({ id: '0', email: '', name: 'Google User', username: 'google_user' });
        }
        setStatus('success');
        setTimeout(() => router.replace('/'), 800);
      } catch (e: any) {
        const isTimeout = e?.code === 'ECONNABORTED' || String(e?.message || '').includes('timeout');
        const msg = isTimeout
          ? 'Google login request timed out. Please check backend connectivity and retry.'
          : e?.response?.data?.message || e?.message || 'Google login verification failed. Please retry.';
        setErrorMsg(msg);
        setStatus('error');
      }
    })();
  }, [searchParams, login, router]);

  if (status === 'loading') {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
        <Spin size="large" />
        <p className="text-slate-500">Completing Google login…</p>
      </div>
    );
  }

  if (status === 'error') {
    return (
      <div className="flex justify-center items-center min-h-[60vh] px-4">
        <Result
          status="error"
          title="Login Failed"
          subTitle={errorMsg}
          extra={[
            <Button type="primary" key="home" onClick={() => router.push('/')}>
              Go Home
            </Button>,
            <Button key="retry" onClick={() => router.push('/')}>
              Retry Login
            </Button>,
          ]}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
      <Spin size="large" />
      <p className="text-slate-500">Login successful, redirecting…</p>
    </div>
  );
}

export default function GoogleCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
          <Spin size="large" />
          <p className="text-slate-500">Loading…</p>
        </div>
      }
    >
      <GoogleCallbackContent />
    </Suspense>
  );
}
