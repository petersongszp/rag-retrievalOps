import { useAuthStore } from '@/store/authStore';

// 认证相关的自定义Hook
export const useAuth = () => {
  const { user, isAuthenticated, login, logout } = useAuthStore();

  return {
    user,
    isAuthenticated,
    login,
    logout,
  };
};

export default useAuth;
