'use client';

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { message } from 'antd';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type { KnowledgeBase } from '@/types/kb';

type CreateKnowledgeBasePayload = {
  name: string;
  description?: string;
};

type KnowledgeBaseContextValue = {
  bases: KnowledgeBase[];
  selectedBase: KnowledgeBase | null;
  isLoading: boolean;
  error: string | null;
  /** true when the last load failure was a 403 permission error */
  isPermissionDenied: boolean;
  refreshBases: () => Promise<KnowledgeBase[]>;
  createBase: (payload: CreateKnowledgeBasePayload) => Promise<KnowledgeBase>;
  setSelectedBaseId: (id?: number | null) => void;
};

const STORAGE_KEY = 'admin.selectedKnowledgeBaseId';

const KnowledgeBaseContext = createContext<KnowledgeBaseContextValue | null>(null);

function isPermissionError(error: unknown): boolean {
  if (error && typeof error === 'object') {
    // Axios HTTP 403
    if (
      'response' in error &&
      error.response &&
      typeof error.response === 'object' &&
      'status' in error.response &&
      error.response.status === 403
    ) {
      return true;
    }
    // Business code 403
    if ('code' in error && error.code === 403) {
      return true;
    }
  }
  return false;
}

function normalizeError(error: unknown, fallback: string): string {
  if (
    error &&
    typeof error === 'object' &&
    'message' in error &&
    typeof error.message === 'string'
  ) {
    return error.message;
  }

  return fallback;
}

export function KnowledgeBaseProvider({ children }: { children: React.ReactNode }) {
  const [bases, setBases] = useState<KnowledgeBase[]>([]);
  const [selectedBaseId, setSelectedBaseIdState] = useState<number | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isPermissionDenied, setIsPermissionDenied] = useState(false);

  const setSelectedBaseId = (id?: number | null) => {
    const normalized = typeof id === 'number' ? id : null;
    setSelectedBaseIdState(normalized);

    if (typeof window !== 'undefined') {
      if (normalized === null) {
        window.localStorage.removeItem(STORAGE_KEY);
      } else {
        window.localStorage.setItem(STORAGE_KEY, String(normalized));
      }
    }
  };

  const refreshBases = useCallback(async (): Promise<KnowledgeBase[]> => {
    try {
      setIsLoading(true);
      setError(null);
      setIsPermissionDenied(false);
      const data = (await apiClient.get(KB_ADMIN_API.LIST_BASES)) as { items?: KnowledgeBase[] };
      const items = data?.items ?? [];
      setBases(items);

      setSelectedBaseIdState((previous) => {
        const existingSelected = items.find((item) => item.id === previous);
        if (existingSelected) {
          return existingSelected.id;
        }

        if (typeof window !== 'undefined') {
          const storedValue = window.localStorage.getItem(STORAGE_KEY);
          if (storedValue) {
            const parsed = Number(storedValue);
            if (items.some((item) => item.id === parsed)) {
              return parsed;
            }
          }
        }

        return items[0]?.id ?? null;
      });

      return items;
    } catch (error) {
      const nextError = normalizeError(error, '加载知识库列表失败');
      setError(nextError);
      setIsPermissionDenied(isPermissionError(error));
      message.error(nextError);
      setBases([]);
      return [];
    } finally {
      setIsLoading(false);
    }
  }, []);

  const createBase = useCallback(
    async (payload: CreateKnowledgeBasePayload): Promise<KnowledgeBase> => {
      const created = (await apiClient.post(KB_ADMIN_API.CREATE_BASE, payload)) as KnowledgeBase;
      message.success(`知识库"${created.name}"已创建`);
      const items = await refreshBases();
      const latest = items.find((item) => item.id === created.id) ?? created;
      setSelectedBaseId(latest.id);
      return latest;
    },
    [refreshBases]
  );

  useEffect(() => {
    void refreshBases();
  }, [refreshBases]);

  const selectedBase = useMemo(
    () => bases.find((item) => item.id === selectedBaseId) ?? null,
    [bases, selectedBaseId]
  );

  const value = useMemo<KnowledgeBaseContextValue>(
    () => ({
      bases,
      selectedBase,
      isLoading,
      error,
      isPermissionDenied,
      refreshBases,
      createBase,
      setSelectedBaseId,
    }),
    [bases, selectedBase, isLoading, error, isPermissionDenied, refreshBases, createBase]
  );

  return <KnowledgeBaseContext.Provider value={value}>{children}</KnowledgeBaseContext.Provider>;
}

export function useKnowledgeBaseContext(): KnowledgeBaseContextValue {
  const context = useContext(KnowledgeBaseContext);

  if (!context) {
    throw new Error('useKnowledgeBaseContext must be used within KnowledgeBaseProvider');
  }

  return context;
}
