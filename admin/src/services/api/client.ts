import axios, {
  AxiosHeaders,
  AxiosInstance,
  AxiosResponse,
  InternalAxiosRequestConfig,
} from 'axios';
import { getAccessToken } from '@/services/auth/session';

const apiClient: AxiosInstance = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_BASE_URL || '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const baseURL = config.baseURL || '';
    const requestUrl = config.url || '';
    if (baseURL.endsWith('/api') && requestUrl.startsWith('/api/')) {
      config.url = requestUrl.replace(/^\/api/, '');
    }

    const headers = config.headers instanceof AxiosHeaders ? config.headers : new AxiosHeaders(config.headers);
    config.headers = headers;

    if (typeof FormData !== 'undefined' && config.data instanceof FormData) {
      headers.delete('Content-Type');
      headers.delete('content-type');
    }

    const accessToken = getAccessToken();
    if (accessToken) {
      if (!headers.has('Authorization')) {
        headers.set('Authorization', `Bearer ${accessToken}`);
      }
    }

    return config;
  },
  (error) => Promise.reject(error)
);

apiClient.interceptors.response.use(
  (response: AxiosResponse) => {
    const payload = response?.data;
    if (payload && typeof payload === 'object' && 'code' in payload) {
      if (payload.code === 200) {
        let data = payload.data;
        if (data && typeof data === 'object' && 'data' in data && Object.keys(data).length === 1) {
          data = (data as { data: unknown }).data;
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
