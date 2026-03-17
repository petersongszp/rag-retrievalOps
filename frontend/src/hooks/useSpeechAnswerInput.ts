'use client';

import { useEffect, useRef, useState } from 'react';

import { message } from 'antd';

import { INTERVIEW_API } from '@/config/api';
import apiClient from '@/services/api/client';

export type SpeechStatus = 'idle' | 'recording' | 'stopping' | 'transcribing' | 'error';

interface UseSpeechAnswerInputOptions {
  enabled: boolean;
  sessionId: string | null;
  interviewType?: string;
  domain?: string;
  onTranscript: (text: string) => void;
}

const MAX_RECORDING_MS = 90 * 1000;
const STOP_TIMEOUT_MS = 1000;

function pickSupportedMimeType(): string {
  if (typeof MediaRecorder === 'undefined') {
    return '';
  }

  const candidates = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4'];
  for (const candidate of candidates) {
    if (MediaRecorder.isTypeSupported(candidate)) {
      return candidate;
    }
  }

  return '';
}

export function useSpeechAnswerInput({
  enabled,
  sessionId,
  interviewType,
  domain,
  onTranscript,
}: UseSpeechAnswerInputOptions) {
  const [status, setStatus] = useState<SpeechStatus>('idle');
  const statusRef = useRef<SpeechStatus>('idle');
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const mediaStreamRef = useRef<MediaStream | null>(null);
  const recordingChunksRef = useRef<Blob[]>([]);
  const mimeTypeRef = useRef<string>('');
  const stopTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const stopAckTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const destroyedRef = useRef(false);
  const stopFinalizedRef = useRef(false);
  const recorderSessionRef = useRef(0);
  const activeStopSessionRef = useRef<number | null>(null);

  const setSpeechStatus = (nextStatus: SpeechStatus) => {
    statusRef.current = nextStatus;
    setStatus(nextStatus);
  };

  const debugLog = (event: string, payload?: Record<string, unknown>) => {
    if (process.env.NODE_ENV === 'production') {
      return;
    }

    console.debug('[SpeechInput]', event, {
      status: statusRef.current,
      recorderState: mediaRecorderRef.current?.state,
      ...payload,
    });
  };

  const clearStopTimer = () => {
    if (stopTimerRef.current) {
      clearTimeout(stopTimerRef.current);
      stopTimerRef.current = null;
    }
  };

  const clearStopAckTimer = () => {
    if (stopAckTimerRef.current) {
      clearTimeout(stopAckTimerRef.current);
      stopAckTimerRef.current = null;
    }
  };

  const cleanupStream = () => {
    clearStopAckTimer();
    clearStopTimer();
    if (mediaStreamRef.current) {
      mediaStreamRef.current.getTracks().forEach((track) => track.stop());
      mediaStreamRef.current = null;
    }
  };

  const detachRecorderHandlers = (recorder?: MediaRecorder | null) => {
    if (!recorder) {
      return;
    }

    recorder.ondataavailable = null;
    recorder.onerror = null;
    recorder.onstop = null;
  };

  const uploadAudio = async (blob: Blob) => {
    debugLog('upload_start', {
      size: blob.size,
      type: blob.type,
    });

    try {
      const extension = blob.type.includes('mp4') ? 'm4a' : 'webm';
      const file = new File([blob], `answer-${Date.now()}.${extension}`, {
        type: blob.type || 'audio/webm',
      });
      const formData = new FormData();
      formData.append('file', file);
      if (sessionId) {
        formData.append('session_id', sessionId);
      }
      if (interviewType) {
        formData.append('interview_type', interviewType);
      }
      if (domain) {
        formData.append('domain', domain);
      }

      const data: any = await apiClient.post(INTERVIEW_API.ASR_TRANSCRIBE, formData, {
        timeout: 120000,
      });

      const transcript = typeof data?.text === 'string' ? data.text.trim() : '';
      if (!transcript) {
        debugLog('upload_success_empty', {
          size: blob.size,
        });
        setSpeechStatus('error');
        message.warning('未识别到清晰的语音内容，请重试');
        return;
      }

      debugLog('upload_success', {
        textLength: transcript.length,
      });
      onTranscript(transcript);
      setSpeechStatus('idle');
    } catch (error: any) {
      debugLog('upload_error', {
        error:
          error instanceof Error
            ? error.message
            : typeof error === 'string'
              ? error
              : JSON.stringify(error),
      });
      setSpeechStatus('error');

      if (error?.message === 'RATE_LIMIT_EXCEEDED' || error?.code === 429) {
        message.warning('语音请求过于频繁，请稍后重试');
        return;
      }
      if (error?.message === 'ASR_NOT_CONFIGURED') {
        message.warning('语音识别暂不可用');
        return;
      }
      if (error?.message === 'ASR_UNAVAILABLE' || error?.code === 503) {
        message.error('语音识别服务暂时不可用，请稍后重试');
        return;
      }

      message.error('语音识别失败，请稍后重试');
    }
  };

  const finalizeRecorderStop = async (
    source: string,
    recorderSessionId: number,
    recorder?: MediaRecorder | null
  ) => {
    if (recorderSessionId != recorderSessionRef.current) {
      debugLog('stop_finalize_stale_recorder', {
        source,
        recorderSessionId,
        currentRecorderSessionId: recorderSessionRef.current,
      });
      return;
    }
    if (
      activeStopSessionRef.current !== null &&
      activeStopSessionRef.current !== recorderSessionId
    ) {
      debugLog('stop_finalize_stale', {
        source,
        recorderSessionId,
        activeStopSessionId: activeStopSessionRef.current,
      });
      return;
    }
    if (stopFinalizedRef.current) {
      debugLog('stop_finalize_ignored', { source });
      return;
    }

    try {
      stopFinalizedRef.current = true;
      activeStopSessionRef.current = null;

      debugLog('stop_finalize', { source, chunkCount: recordingChunksRef.current.length });

      if (destroyedRef.current) {
        debugLog('stop_finalize_destroyed', { source });
        detachRecorderHandlers(recorder);
        cleanupStream();
        return;
      }

      const chunks = [...recordingChunksRef.current];
      recordingChunksRef.current = [];
      const blob = new Blob(chunks, {
        type: mimeTypeRef.current || 'audio/webm',
      });

      debugLog('final_blob_ready', {
        source,
        chunkCount: chunks.length,
        size: blob.size,
        type: blob.type,
      });

      setSpeechStatus('transcribing');
      detachRecorderHandlers(recorder);
      cleanupStream();

      if (blob.size === 0) {
        setSpeechStatus('error');
        message.error('未录到有效语音，请重试');
        return;
      }

      await uploadAudio(blob);
    } catch (error) {
      debugLog('stop_finalize_failed', {
        source,
        error:
          error instanceof Error
            ? error.message
            : typeof error === 'string'
              ? error
              : JSON.stringify(error),
      });
      detachRecorderHandlers(recorder);
      cleanupStream();
      setSpeechStatus('error');
      message.error('处理录音结果失败，请重试');
    }
  };

  const scheduleFinalizeStop = (
    source: string,
    recorderSessionId: number,
    recorder?: MediaRecorder | null
  ) => {
    setTimeout(() => {
      void finalizeRecorderStop(source, recorderSessionId, recorder);
    }, 0);
  };

  const stopRecording = (showLimitMessage = false) => {
    const recorder = mediaRecorderRef.current;
    const recorderState = recorder?.state;
    const recorderSessionId = recorderSessionRef.current;

    debugLog('stop_click');

    if (!recorder || recorderState === 'inactive') {
      debugLog('stop_ignored');
      return;
    }
    if (statusRef.current === 'stopping' || statusRef.current === 'transcribing') {
      debugLog('stop_ignored_busy');
      return;
    }

    setSpeechStatus('stopping');
    activeStopSessionRef.current = recorderSessionId;
    clearStopTimer();
    clearStopAckTimer();

    if (showLimitMessage) {
      message.info('单次录音已达 90 秒，已自动停止');
    }

    try {
      recorder.requestData();
    } catch {
      // Some browsers do not support requestData during stop.
    }

    stopAckTimerRef.current = setTimeout(() => {
      const latestRecorderState = recorder.state;
      const chunkCount = recordingChunksRef.current.length;
      debugLog('stop_timeout', { latestRecorderState, chunkCount });

      if (latestRecorderState === 'inactive' || chunkCount > 0) {
        mediaRecorderRef.current = null;
        scheduleFinalizeStop('timeout_fallback', recorderSessionId, recorder);
        return;
      }

      activeStopSessionRef.current = null;
      stopFinalizedRef.current = true;
      mediaRecorderRef.current = null;
      detachRecorderHandlers(recorder);
      cleanupStream();
      setSpeechStatus('error');
      message.error('停止录音超时，请重试');
    }, STOP_TIMEOUT_MS);

    try {
      recorder.stop();
    } catch (error) {
      debugLog('stop_failed', { error });
      clearStopAckTimer();
      activeStopSessionRef.current = null;
      cleanupStream();
      setSpeechStatus('error');
      message.error('停止录音失败，请重试');
      return;
    }

    setTimeout(() => {
      if (
        statusRef.current === 'stopping' &&
        recorder.state === 'inactive' &&
        !stopFinalizedRef.current
      ) {
        debugLog('recorder_inactive_without_onstop', {
          chunkCount: recordingChunksRef.current.length,
        });
        mediaRecorderRef.current = null;
        scheduleFinalizeStop('inactive_poll', recorderSessionId, recorder);
      }
    }, 0);
  };

  const startRecording = async () => {
    debugLog('start_click');

    if (!enabled) {
      message.warning('语音识别暂不可用');
      return;
    }
    if (
      statusRef.current === 'recording' ||
      statusRef.current === 'stopping' ||
      statusRef.current === 'transcribing'
    ) {
      debugLog('start_ignored_busy');
      return;
    }
    if (!sessionId) {
      message.warning('请等待面试会话建立后再使用语音输入');
      return;
    }
    if (
      typeof navigator === 'undefined' ||
      !navigator.mediaDevices?.getUserMedia ||
      typeof MediaRecorder === 'undefined'
    ) {
      setSpeechStatus('error');
      message.error('当前浏览器不支持语音输入');
      return;
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      });

      const mimeType = pickSupportedMimeType();
      const recorder = mimeType
        ? new MediaRecorder(stream, { mimeType })
        : new MediaRecorder(stream);
      const recorderSessionId = recorderSessionRef.current + 1;

      stopFinalizedRef.current = false;
      recorderSessionRef.current = recorderSessionId;
      activeStopSessionRef.current = null;
      recordingChunksRef.current = [];
      mimeTypeRef.current = recorder.mimeType || mimeType || 'audio/webm';
      mediaRecorderRef.current = recorder;
      mediaStreamRef.current = stream;

      recorder.ondataavailable = (event) => {
        if (recorderSessionId !== recorderSessionRef.current) {
          debugLog('data_stale_recorder_ignored', {
            recorderSessionId,
            currentRecorderSessionId: recorderSessionRef.current,
          });
          return;
        }
        if (stopFinalizedRef.current) {
          debugLog('data_after_finalize_ignored', {
            size: event.data?.size ?? 0,
          });
          return;
        }

        if (event.data && event.data.size > 0) {
          recordingChunksRef.current.push(event.data);
        }

        if (statusRef.current === 'stopping' && recorder.state === 'inactive') {
          debugLog('recorder_inactive_after_data', {
            chunkCount: recordingChunksRef.current.length,
            recorderSessionId,
          });
          mediaRecorderRef.current = null;
          scheduleFinalizeStop('dataavailable_inactive', recorderSessionId, recorder);
        }
      };
      recorder.onerror = () => {
        debugLog('recorder_onerror');
        activeStopSessionRef.current = null;
        cleanupStream();
        mediaRecorderRef.current = null;
        setSpeechStatus('error');
        message.error('录音失败，请稍后重试');
      };
      recorder.onstop = () => {
        debugLog('recorder_onstop', { recorderSessionId });
        mediaRecorderRef.current = null;
        scheduleFinalizeStop('onstop', recorderSessionId, recorder);
      };

      recorder.start();
      debugLog('recorder_onstart');
      setSpeechStatus('recording');
      stopTimerRef.current = setTimeout(() => stopRecording(true), MAX_RECORDING_MS);
    } catch (error: any) {
      debugLog('start_failed', { error });
      cleanupStream();
      setSpeechStatus('error');

      if (error?.name === 'NotAllowedError') {
        message.warning('未授予麦克风权限，请在浏览器设置中开启');
        return;
      }

      message.error('无法启用麦克风，请检查设备后重试');
    }
  };

  const handleMicClick = () => {
    const recorderState = mediaRecorderRef.current?.state;
    debugLog('mic_click', { recorderState });

    if (
      statusRef.current === 'recording' ||
      statusRef.current === 'stopping' ||
      recorderState === 'recording'
    ) {
      stopRecording();
      return;
    }
    if (statusRef.current === 'transcribing') {
      return;
    }

    void startRecording();
  };

  useEffect(() => {
    destroyedRef.current = false;

    return () => {
      destroyedRef.current = true;
      if (mediaRecorderRef.current && mediaRecorderRef.current.state !== 'inactive') {
        activeStopSessionRef.current = null;
        stopFinalizedRef.current = true;
        detachRecorderHandlers(mediaRecorderRef.current);
        mediaRecorderRef.current.stop();
      }
      clearStopAckTimer();
      clearStopTimer();
      if (mediaStreamRef.current) {
        mediaStreamRef.current.getTracks().forEach((track) => track.stop());
        mediaStreamRef.current = null;
      }
    };
  }, []);

  return {
    status,
    isRecording: status === 'recording',
    isStopping: status === 'stopping',
    isTranscribing: status === 'transcribing',
    handleMicClick,
  };
}
