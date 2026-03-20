export const metadata = {
  title: 'Google 登录',
  description: 'Google OAuth callback',
};

export default function GoogleCallbackLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="zh">
      <body>{children}</body>
    </html>
  );
}
