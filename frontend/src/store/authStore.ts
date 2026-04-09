import { create } from 'zustand';
import { api } from '../services/api';

interface User {
  id: string;
  email: string;
  role: string;
  permissions: string[];
}

interface AuthState {
  token: string | null;
  user: User | null;
  setToken: (token: string | null) => void;
  setUser: (user: User | null) => void;
  fetchUser: () => Promise<void>;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: localStorage.getItem('token'),
  user: null,
  setToken: (token) => {
    if (token) localStorage.setItem('token', token);
    else localStorage.removeItem('token');
    set({ token });
  },
  setUser: (user) => set({ user }),
  fetchUser: async () => {
    try {
      const response = await api.get('/auth/me');
      if (response.status === 200) {
        set({ user: response.data.data });
      } else {
        get().logout();
      }
    } catch (err) {
      console.error('Failed to fetch user', err);
    }
  },
  logout: () => {
    localStorage.removeItem('token');
    set({ token: null, user: null });
  },
}));
