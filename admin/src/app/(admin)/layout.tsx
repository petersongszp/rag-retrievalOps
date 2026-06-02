'use client';

import { AdminShell } from '@/components/admin/admin-shell';
import { RequireAuth } from '@/components/auth/require-auth';

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return (
    <RequireAuth>
      <AdminShell>{children}</AdminShell>
    </RequireAuth>
  );
}
