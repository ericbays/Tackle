import { useQuery } from '@tanstack/react-query';
import { api } from '../services/api';

export const useAuditLogs = () => {
  return useQuery({
    queryKey: ['audit-logs'],
    queryFn: () => api.get('/logs/audit?limit=20').then(res => res.data.data),
  });
};

export const useTrends = () => {
  return useQuery({
    queryKey: ['trends'],
    queryFn: () => api.get('/metrics/trends').then(res => res.data.data),
  });
};

export const useOrganizationMetrics = () => {
  return useQuery({
    queryKey: ['org-metrics'],
    queryFn: () => api.get('/metrics/organization').then(res => res.data.data),
  });
};

export const useCampaigns = () => {
  return useQuery({
    queryKey: ['campaigns'],
    queryFn: () => api.get('/campaigns').then(res => res.data.data),
  });
};

export const useEndpoints = () => {
  return useQuery({
    queryKey: ['endpoints'],
    queryFn: () => api.get('/endpoints').then(res => res.data.data),
  });
};
