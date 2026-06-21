import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { AuthProvider } from '@/services/auth/store';
import '../styles/globals.css';

const inter = Inter({ subsets: ['latin'] });

export const metadata: Metadata = {
  title: '智能知识库管理平台',
  description: '知识库、检索质量与运营治理平台',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN">
      <body className={inter.className}>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
