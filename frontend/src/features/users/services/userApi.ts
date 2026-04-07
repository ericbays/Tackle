import { api } from '../../../services/api';

export interface User {
  id: string;
  email: string;
  username: string;
  display_name: string;
  is_initial_admin: boolean;
  auth_provider: string;
  status: string;
  force_password_change: boolean;
  created_at: string;
  updated_at: string;
  roles?: Role[];
}

export interface Role {
  id: string;
  name: string;
  description: string;
  is_builtin: boolean;
  permissions?: string[];
}

export const userApi = {
  getUsers: async (): Promise<User[]> => {
    const res = await api.get('/users');
    return res.data.data?.data || res.data.data || [];
  },
  getUser: async (id: string): Promise<User> => {
    const res = await api.get(`/users/${id}`);
    return res.data.data;
  },
  createUser: async (data: Partial<User>): Promise<User> => {
    const res = await api.post('/users', data);
    return res.data.data;
  },
  updateUser: async (id: string, data: Partial<User>): Promise<User> => {
    const res = await api.put(`/users/${id}`, data);
    return res.data.data;
  },
  deleteUser: async (id: string): Promise<void> => {
    await api.delete(`/users/${id}`);
  },
  getUserRoles: async (id: string): Promise<Role[]> => {
    const res = await api.get(`/users/${id}/roles`);
    return res.data.data;
  },
  updateUserRoles: async (id: string, roleIds: string[]): Promise<void> => {
    await api.put(`/users/${id}/roles`, { role_ids: roleIds });
  }
};
