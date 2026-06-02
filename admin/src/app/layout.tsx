import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { AuthProvider } from '@/services/auth/store';
import '../styles/globals.css';

const inter = Inter({ subsets: ['latin'] });

export const metadata: Metadata = {
  title: 'RAG 管理后台',
  description: 'RAG 知识库管理控制台',
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
