import { useQuery } from '@tanstack/react-query';
import { api } from '../services/api';

export interface CloudCredential {
    id: string;
    display_name: string;
    provider_type: string;
    default_region: string;
}

export interface Domain {
    id: string;
    domain_name: string;
    status: string;
}

export const useCloudCredentials = () => {
    return useQuery<CloudCredential[]>({
        queryKey: ['cloud-credentials'],
        queryFn: async () => {
            try {
                const res = await api.get('/settings/cloud-credentials');
                return res.data?.data || [];
            } catch (err) {
                console.error("Failed to fetch cloud credentials:", err);
                return [];
            }
        }
    });
};

export const useDomains = () => {
    return useQuery<Domain[]>({
        queryKey: ['domains'],
        queryFn: async () => {
            try {
                const res = await api.get('/domains'); 
                return res.data?.data?.items || res.data?.data || [];
            } catch (err) {
                console.error("Failed to fetch domains:", err);
                return [];
            }
        }
    });
};

export interface LandingPage {
    id: string;
    name: string;
    template_type: string;
    views: number;
    updated_at: string;
}

export const useLandingPages = () => {
    return useQuery<LandingPage[]>({
        queryKey: ['landing-pages'],
        queryFn: async () => {
            try {
                const res = await api.get('/landing-pages');
                return res.data?.data || [];
            } catch (err) {
                console.error("Failed to fetch landing pages:", err);
                return [];
            }
        }
    });
};

export interface EmailTemplate {
    id: string;
    name: string;
}

export const useEmailTemplates = () => {
    return useQuery<EmailTemplate[]>({
        queryKey: ['email-templates'],
        queryFn: async () => {
            try {
                const res = await api.get('/email-templates');
                return res.data?.data || [];
            } catch (err) {
                console.error("Failed to fetch email templates:", err);
                return [];
            }
        }
    });
};

export interface SmtpProfile {
    id: string;
    name: string;
}

export const useSmtpProfiles = () => {
    return useQuery<SmtpProfile[]>({
        queryKey: ['settings', 'smtp-profiles'],
        queryFn: async () => {
            try {
                const res = await api.get('/smtp-profiles');
                return res.data?.data || [];
            } catch (err) {
                console.error("Failed to fetch SMTP profiles:", err);
                return [];
            }
        }
    });
};
