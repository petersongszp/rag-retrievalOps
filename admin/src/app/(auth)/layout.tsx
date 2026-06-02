import { GuestOnly } from '@/components/auth/guest-only';

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return <GuestOnly>{children}</GuestOnly>;
}
