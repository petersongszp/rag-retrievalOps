'use client';

import { useEffect, useState } from 'react';

import { INTERVIEW_API } from '@/config/api';
import apiClient from '@/services/api/client';

export interface ASRCapabilityState {
  enabled: boolean;
  reason?: string;
  provider?: string;
  model?: string;
  loading: boolean;
}

const initialState: ASRCapabilityState = {
  enabled: false,
  loading: true,
};

export function useASRCapability(): ASRCapabilityState {
  const [state, setState] = useState<ASRCapabilityState>(initialState);

  useEffect(() => {
    let cancelled = false;

    const loadCapability = async () => {
      try {
        const data: any = await apiClient.get(INTERVIEW_API.ASR_CAPABILITY);
        if (cancelled) {
          return;
        }

        setState({
          enabled: Boolean(data?.enabled),
          reason: data?.reason,
          provider: data?.provider,
          model: data?.model,
          loading: false,
        });
      } catch (error) {
        if (cancelled) {
          return;
        }

        setState({
          enabled: false,
          reason: 'CHECK_FAILED',
          loading: false,
        });
      }
    };

    void loadCapability();

    return () => {
      cancelled = true;
    };
  }, []);

  return state;
}
