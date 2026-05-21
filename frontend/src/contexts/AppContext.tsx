import React, { createContext, useContext, useState, ReactNode } from 'react';
import type { UserInfo, PointsResponse, BillingSettingsResponse } from '@/types/api';
import type { UserTheme, BusyAction } from '@/types/app';
import { getStoredToken, clearStoredToken, APIError } from '@/api/client';

export function shouldForceLogout(error: unknown) {
  return error instanceof APIError && (error.status === 401 || error.status === 403);
}

interface AppContextType {
  userTheme: UserTheme;
  setUserTheme: (theme: UserTheme) => void;
  token: string | null;
  setToken: (token: string | null) => void;
  currentUser: UserInfo | null;
  setCurrentUser: (user: UserInfo | null) => void;
  points: PointsResponse | null;
  setPoints: (points: PointsResponse | null) => void;
  billingSettings: BillingSettingsResponse | null;
  setBillingSettings: (settings: BillingSettingsResponse | null) => void;
  busy: BusyAction;
  setBusy: (busy: BusyAction) => void;
  message: string;
  setMessage: (message: string) => void;
  logout: () => void;
  handleApiError: (error: unknown, fallback: string) => void;
}

export const AppContext = createContext<AppContextType | null>(null);

export function AppProvider({ children }: { children: ReactNode }) {
  const [userTheme, setUserTheme] = useState<UserTheme>('dark');
  const [token, setToken] = useState<string | null>(getStoredToken());
  const [currentUser, setCurrentUser] = useState<UserInfo | null>(null);
  const [points, setPoints] = useState<PointsResponse | null>(null);
  const [billingSettings, setBillingSettings] = useState<BillingSettingsResponse | null>(null);
  const [busy, setBusy] = useState<BusyAction>(null);
  const [message, setMessage] = useState('');

  const logout = () => {
    setToken(null);
    setCurrentUser(null);
    clearStoredToken();
  };

  const handleApiError = (error: unknown, fallback: string) => {
    if (shouldForceLogout(error)) {
      logout();
      setMessage(error instanceof APIError && error.status === 403 ? '账号已被禁用，请联系管理员。' : '登录已过期，请重新登录。');
      return;
    }
    setMessage(error instanceof Error ? error.message : fallback);
  };

  return (
    <AppContext.Provider value={{
      userTheme, setUserTheme,
      token, setToken,
      currentUser, setCurrentUser,
      points, setPoints,
      billingSettings, setBillingSettings,
      busy, setBusy,
      message, setMessage,
      logout, handleApiError
    }}>
      {children}
    </AppContext.Provider>
  );
}

export function useAppContext() {
  const context = useContext(AppContext);
  if (!context) throw new Error('useAppContext must be used within AppProvider');
  return context;
}
