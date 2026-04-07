import { api } from '../../../services/api';
import type { Role } from './userApi'; // re-use Role interface

export interface Permission {
  permission: string;
  resource: string;
  action: string;
}

export const roleApi = {
  getRoles: async (): Promise<Role[]> => {
    const res = await api.get('/roles');
    return Array.isArray(res.data.data) ? res.data.data : [];
  },
  getRole: async (id: string): Promise<Role> => {
    const res = await api.get(`/roles/${id}`);
    return res.data.data;
  },
  createRole: async (data: Partial<Role>): Promise<Role> => {
    const res = await api.post('/roles', data);
    return res.data.data;
  },
  updateRole: async (id: string, data: Partial<Role>): Promise<Role> => {
    const res = await api.put(`/roles/${id}`, data);
    return res.data.data;
  },
  deleteRole: async (id: string): Promise<void> => {
    await api.delete(`/roles/${id}`);
  },
  getPermissions: async (): Promise<Permission[]> => {
    const res = await api.get('/permissions');
    return res.data.data;
  }
};
