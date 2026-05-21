import React, { createContext, useContext, ReactNode } from 'react';
import type { UserInfo, PointsResponse, BillingSettingsResponse } from '@/types/api';
import type { UserTheme, BusyAction } from '@/types/app';

export interface AppContextType {
  userTheme: UserTheme;
  setUserTheme: (theme: UserTheme) => void;
  token: string;
  setToken: (token: string) => void;
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

/**
 * AppProvider now receives its value from outside (i.e., from App.tsx).
 * This ensures a single source of truth for all shared state.
 */
export function AppProvider({ value, children }: { value: AppContextType; children: ReactNode }) {
  return (
    <AppContext.Provider value={value}>
      {children}
    </AppContext.Provider>
  );
}

export function useAppContext() {
  const context = useContext(AppContext);
  if (!context) throw new Error('useAppContext must be used within AppProvider');
  return context;
}
