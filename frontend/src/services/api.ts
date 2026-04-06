import axios from 'axios';
import { useAuthStore } from '../store/authStore';

export const api = axios.create({
  baseURL: 'http://localhost:8080/api/v1',
  withCredentials: true,
  xsrfCookieName: 'tackle_csrf',
  xsrfHeaderName: 'X-CSRF-Token',
  headers: {
    'Content-Type': 'application/json',
  },
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  
  // Extract tackle_csrf cookie and set X-CSRF-Token header manually
  const match = document.cookie.match(/(?:^|; )tackle_csrf=([^;]+)/);
  if (match && config.headers) {
    config.headers['X-CSRF-Token'] = match[1];
  }
  
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      useAuthStore.getState().logout();
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);
