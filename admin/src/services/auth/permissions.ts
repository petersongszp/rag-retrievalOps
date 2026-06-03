import type { UserRole } from '@/types/auth';

export function canCreateKB(role?: UserRole): boolean {
  return role === 'owner' || role === 'admin' || role === 'member';
}

export function canUploadDocument(role?: UserRole): boolean {
  return role === 'owner' || role === 'admin' || role === 'member';
}

export function canDeleteKB(role?: UserRole): boolean {
  return role === 'owner' || role === 'admin';
}

export function canManageAPIKey(role?: UserRole): boolean {
  return role === 'owner' || role === 'admin' || role === 'member';
}

export function canViewTenantSettings(role?: UserRole): boolean {
  return role === 'owner';
}

export function canViewUsage(role?: UserRole): boolean {
  return role === 'owner' || role === 'admin' || role === 'member';
}
