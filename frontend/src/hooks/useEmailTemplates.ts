import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../services/api';

export interface EmailTemplate {
    id: string;
    name: string;
    description: string;
    subject: string;
    html_body: string;
    category: string;
    tags: string[];
    sender_name: string;
    sender_email: string;
    is_shared: boolean;
    created_at: string;
    updated_at: string;
}

export const useEmailTemplates = (filters?: Record<string, string | number>) => {
    return useQuery({
        queryKey: ['email-templates', filters],
        queryFn: async () => {
            const res = await api.get('/email-templates', { params: filters });
            return {
                items: res.data?.data || [],
                pagination: res.data?.pagination || null
            };
        }
    });
};

export const useEmailTemplate = (id: string | null) => {
    return useQuery({
        queryKey: ['email-templates', id],
        queryFn: async () => {
            if (!id) return null;
            const res = await api.get(`/email-templates/${id}`);
            return res.data?.data as EmailTemplate;
        },
        enabled: !!id
    });
};

export const useSaveEmailTemplate = () => {
    const queryClient = useQueryClient();
    
    return useMutation({
        mutationFn: async (vars: { id?: string; payload: Partial<EmailTemplate> }) => {
            if (vars.id) {
                const res = await api.put(`/email-templates/${vars.id}`, vars.payload);
                return res.data?.data;
            } else {
                const res = await api.post('/email-templates', vars.payload);
                return res.data?.data;
            }
        },
        onSuccess: (data, vars) => {
            // Instantly update the cache so navigating away and back is seamless
            if (data && data.id) {
                queryClient.setQueryData(['email-templates', data.id], data);
            }
            queryClient.invalidateQueries({ queryKey: ['email-templates'] });
        }
    });
};

export const useDuplicateTemplate = () => {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (id: string) => {
            const res = await api.post(`/email-templates/${id}/clone`);
            return res.data?.data;
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['email-templates'] });
        }
    });
};

export const useDeleteTemplate = () => {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (id: string) => {
            await api.delete(`/email-templates/${id}`);
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['email-templates'] });
        }
    });
};

export const usePreviewTemplate = () => {
    return useMutation({
        mutationFn: async (vars: { id?: string; subject: string; html_body: string }) => {
            const previewId = vars.id || 'draft';
            const res = await api.post(`/email-templates/${previewId}/preview`, {
                subject: vars.subject, 
                html_body: vars.html_body 
            });
            // The Go backend struct 'PreviewResult' uses 'html_body' json tag
            return res.data?.data?.html_body || '';
        }
    });
};
