import axios, { AxiosInstance, InternalAxiosRequestConfig, AxiosResponse } from 'axios';

// 创建axios实例
const apiClient: AxiosInstance = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_BASE_URL || '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 请求拦截器
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const baseURL = config.baseURL || '';
    const requestUrl = config.url || '';
    if (baseURL.endsWith('/api') && requestUrl.startsWith('/api/')) {
      config.url = requestUrl.replace(/^\/api/, '');
    }

    if (typeof FormData !== 'undefined' && config.data instanceof FormData) {
      config.headers = (config.headers || {}) as any;
      delete (config.headers as any)['Content-Type'];
      delete (config.headers as any)['content-type'];
    }
    return config;
  },
  (error) => Promise.reject(error)
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
      return Promise.reject({ response, message: payload.message, code: payload.code });
    }
    return payload;
  },
  (error) => Promise.reject(error)
);

export default apiClient;
