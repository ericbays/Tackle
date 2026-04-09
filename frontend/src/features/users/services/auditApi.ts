import { api } from '../../../services/api';

export interface AuditLog {
  id: string;
  category: string;
  severity: string;
  actor_type: string;
  actor_id: string;
  actor_label: string;
  action: string;
  target_type: string;
  target_id: string;
  target_label: string;
  source_ip: string;
  status: string;
  timestamp: string;
}

export interface AuditLogQuery {
  limit?: number;
  cursor?: string;
  category?: string;
  search?: string;
}

export interface AuditLogResponse {
  data: AuditLog[];
  next_cursor?: string;
}

export const auditApi = {
  getLogs: async (params?: AuditLogQuery): Promise<AuditLogResponse> => {
    const response = await api.get('/logs/audit', { params });
    return response.data;
  },
};
