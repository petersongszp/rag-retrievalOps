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
    const url = config.url || '';
    const isAuthFree =
      url.includes('/user/register') ||
      url.includes('/user/login') ||
      url.includes('/user/logout') ||
      url.includes('/user/github/login') ||
      url.includes('/user/github/callback');
    const token = localStorage.getItem('token');
    if (token && !isAuthFree) {
      config.headers = (config.headers || {}) as any;
      (config.headers as any).Authorization = `Bearer ${token}`;
      (config.headers as any)['X-Auth-Token'] = token;
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
