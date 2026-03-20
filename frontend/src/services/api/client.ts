import axios, { AxiosInstance, InternalAxiosRequestConfig, AxiosResponse } from 'axios';
import { useAuthStore } from '@/store/authStore';

// 创建axios实例
const apiClient: AxiosInstance = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_BASE_URL || '/api',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 请求拦截器
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    // Some callers pass '/api/...' while baseURL already ends with '/api'.
    // Normalize here to avoid requests like '/api/api/user/register'.
    const baseURL = config.baseURL || '';
    const requestUrl = config.url || '';
    if (baseURL.endsWith('/api') && requestUrl.startsWith('/api/')) {
      config.url = requestUrl.replace(/^\/api/, '');
    }

    const url = config.url || '';
    const isAuthFree =
      url.includes('/user/register') ||
      url.includes('/user/login') ||
      url.includes('/user/logout') ||
      url.includes('/user/github/login') ||
      url.includes('/user/github/callback') ||
      url.includes('/user/google/login') ||
      url.includes('/user/google/callback');
    const token = localStorage.getItem('token');
    if (token && !isAuthFree) {
      config.headers = (config.headers || {}) as any;
      (config.headers as any).Authorization = `Bearer ${token}`;
      (config.headers as any)['X-Auth-Token'] = token;
    }
    if (typeof FormData !== 'undefined' && config.data instanceof FormData) {
      config.headers = (config.headers || {}) as any;
      delete (config.headers as any)['Content-Type'];
      delete (config.headers as any)['content-type'];
    }
    // 为面试评估和答题记录接口设置 3 分钟超时
    if (url.includes('/interview/evaluation') || url.includes('/interview/answer-record')) {
      config.timeout = 180000; // 3 分钟 = 180 秒
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// 响应拦截器
apiClient.interceptors.response.use(
  (response: AxiosResponse<any>) => {
    const payload = response?.data;
    if (payload && typeof payload === 'object' && 'code' in payload) {
      if (payload.code === 200) {
        let data = payload.data;
        if (data && typeof data === 'object' && 'data' in data && Object.keys(data).length === 1) {
          data = (data as any).data;
        }
        return data;
      }
      if (payload.code === 401) {
        localStorage.removeItem('token');
        localStorage.removeItem('user');
        useAuthStore.getState().logout();
      }
      return Promise.reject({ response, message: payload.message, code: payload.code });
    }
    return payload;
  },
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      useAuthStore.getState().logout();
    }
    return Promise.reject(error);
  }
);

export default apiClient;
