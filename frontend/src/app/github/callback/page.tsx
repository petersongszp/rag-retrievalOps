'use client';

import { useSearchParams, useRouter } from 'next/navigation';
import { useEffect, useState, useRef, Suspense } from 'react';
import { Spin, Result, Button } from 'antd';
import apiClient from '@/services/api/client';
import { useAuthStore } from '@/store/authStore';

function GitHubCallbackContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const login = useAuthStore((s) => s.login);
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  const [errorMsg, setErrorMsg] = useState<string>('');
  const didRun = useRef(false);

  useEffect(() => {
    const code = searchParams.get('code');
    if (!code) {
      setErrorMsg('缺少授权码，请从登录页重新使用 GitHub 登录');
      setStatus('error');
      return;
    }

    // 防止 React Strict Mode 或依赖变化导致 effect 执行两次，用同一 code 重复请求会报错并先显示“登录失败”
    if (didRun.current) return;
    didRun.current = true;

    (async () => {
      try {
        const res: any = await apiClient.post('/user/github/callback', { code }, { timeout: 30000 });
        const data = res?.data ?? res;
        const token = data?.token || data?.accessToken;
        if (!token) {
          setErrorMsg('登录失败：未返回令牌');
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
          login({ id: '0', email: '', name: 'GitHub 用户', username: 'github_user' });
        }
        setStatus('success');
        setTimeout(() => router.replace('/'), 800);
      } catch (e: any) {
        const isTimeout = e?.code === 'ECONNABORTED' || String(e?.message || '').includes('timeout');
        const msg = isTimeout
          ? 'GitHub 登录请求超时，请检查后端服务与 GitHub 连通性后重试'
          : e?.response?.data?.message || e?.message || 'GitHub 登录验证失败，请重试';
        setErrorMsg(msg);
        setStatus('error');
      }
    })();
  }, [searchParams, login, router]);

  if (status === 'loading') {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
        <Spin size="large" />
        <p className="text-slate-500">正在完成 GitHub 登录…</p>
      </div>
    );
  }

  if (status === 'error') {
    return (
      <div className="flex justify-center items-center min-h-[60vh] px-4">
        <Result
          status="error"
          title="登录失败"
          subTitle={errorMsg}
          extra={[
            <Button type="primary" key="home" onClick={() => router.push('/')}>
              返回首页
            </Button>,
            <Button key="retry" onClick={() => router.push('/')}>
              重新登录
            </Button>,
          ]}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
      <Spin size="large" />
      <p className="text-slate-500">登录成功，正在跳转…</p>
    </div>
  );
}

export default function GitHubCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
          <Spin size="large" />
          <p className="text-slate-500">加载中…</p>
        </div>
      }
    >
      <GitHubCallbackContent />
    </Suspense>
  );
}
