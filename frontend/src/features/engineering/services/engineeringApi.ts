import { api } from '../../../services/api';

// DTO Interfaces based on backend structs
export interface CloudCredential {
  id: string;
  provider_type: 'aws' | 'azure';
  display_name: string;
  default_region: string;
  status: 'active' | 'error' | 'pending';
  last_tested_at?: string;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export interface DomainProvider {
  id: string;
  provider_type: 'namecheap' | 'route53' | 'godaddy' | 'azure_dns';
  display_name: string;
  status: string;
  status_message?: string;
  last_tested_at?: string;
  created_at: string;
}

export interface Endpoint {
  id: string;
  campaign_id: string;
  provider_id: string;
  cloud_provider: string;
  instance_id: string;
  public_ip: string;
  status: 'provisioning' | 'running' | 'stopped' | 'terminated' | 'error';
  health_status: 'healthy' | 'unhealthy' | 'unknown';
  created_at: string;
  updated_at: string;
}

// Basic service client for Infrastructure / Engineering
export const engineeringApi = {
  // Cloud Credentials
  getCloudCredentials: async (): Promise<CloudCredential[]> => {
    const res = await api.get('/settings/cloud-credentials');
    return Array.isArray(res.data.data) ? res.data.data : [];
  },
  createCloudCredential: async (data: any): Promise<CloudCredential> => {
    const res = await api.post('/settings/cloud-credentials', data);
    return res.data.data;
  },
  updateCloudCredential: async (id: string, data: any): Promise<CloudCredential> => {
    const res = await api.put(`/settings/cloud-credentials/${id}`, data);
    return res.data.data;
  },

  // Domain Providers
  getDomainProviders: async (): Promise<DomainProvider[]> => {
    const res = await api.get('/settings/domain-providers');
    return Array.isArray(res.data.data) ? res.data.data : [];
  },
  createDomainProvider: async (data: any): Promise<DomainProvider> => {
    const res = await api.post('/settings/domain-providers', data);
    return res.data.data;
  },
  updateDomainProvider: async (id: string, data: any): Promise<DomainProvider> => {
    const res = await api.put(`/settings/domain-providers/${id}`, data);
    return res.data.data;
  },
  deleteDomainProvider: async (id: string): Promise<void> => {
    await api.delete(`/settings/domain-providers/${id}`);
  },
  syncDomainProviders: async (): Promise<{ status: string }> => {
    const res = await api.post('/domains/sync');
    return res.data.data;
  },

  // Endpoints
  getEndpoints: async (): Promise<Endpoint[]> => {
    const res = await api.get('/endpoints');
    return Array.isArray(res.data.data) ? res.data.data : [];
  },

  // Domains
  getDomains: async (): Promise<any[]> => {
    try {
      const res = await api.get('/domains');
      if (res.data?.data?.items && Array.isArray(res.data.data.items)) {
        return res.data.data.items;
      }
      return Array.isArray(res.data?.data) ? res.data.data : [];
    } catch (e) {
      return [];
    }
  },

  // DNS Records
  getDomainDnsRecords: async (domainId: string): Promise<any[]> => {
    const res = await api.get(`/domains/${domainId}/dns-records`);
    return Array.isArray(res.data?.data) ? res.data.data : [];
  },

  createDomainDnsRecord: async (params: { domainId: string, record: any }): Promise<any> => {
    const res = await api.post(`/domains/${params.domainId}/dns-records`, params.record);
    return res.data?.data;
  },

  updateDomainDnsRecord: async (params: { domainId: string, recordId: string, record: any }): Promise<any> => {
    const res = await api.put(`/domains/${params.domainId}/dns-records/${params.recordId}`, params.record);
    return res.data?.data;
  },

  deleteDomainDnsRecord: async (params: { domainId: string, recordId: string }): Promise<any> => {
    const res = await api.delete(`/domains/${params.domainId}/dns-records/${params.recordId}`);
    return res.data?.data;
  }
};
