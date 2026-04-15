import { create } from 'zustand';
import { api } from '../services/api';
import toast from 'react-hot-toast';

export interface SMTPProfileDTO {
    id: string;
    name: string;
    description?: string;
    host: string;
    port: number;
    auth_type: string;
    has_username: boolean;
    has_password: boolean;
    tls_mode: string;
    tls_skip_verify: boolean;
    from_address: string;
    from_name?: string;
    reply_to?: string;
    custom_helo?: string;
    max_send_rate?: number;
    max_connections: number;
    timeout_connect: number;
    timeout_send: number;
    status: string;
    status_message?: string;
    last_tested_at?: string;
    created_by: string;
    created_at: string;
    updated_at: string;
}

export interface TestResult {
    success: boolean;
    stage_reached: string;
    tls_version?: string;
    server_banner?: string;
    error_detail?: string;
}

interface SMTPStore {
    profiles: SMTPProfileDTO[];
    currentProfile: SMTPProfileDTO | null;
    isLoading: boolean;
    isTesting: boolean;
    error: string | null;

    fetchProfiles: (status?: string, nameSearch?: string) => Promise<void>;
    getProfile: (id: string) => Promise<SMTPProfileDTO | null>;
    createProfile: (data: any) => Promise<SMTPProfileDTO | null>;
    updateProfile: (id: string, data: any) => Promise<SMTPProfileDTO | null>;
    deleteProfile: (id: string) => Promise<boolean>;
    duplicateProfile: (id: string, newName: string) => Promise<SMTPProfileDTO | null>;
    testProfile: (id: string) => Promise<TestResult | null>;
    clearCurrent: () => void;
}

export const useSMTPStore = create<SMTPStore>((set, get) => ({
    profiles: [],
    currentProfile: null,
    isLoading: false,
    isTesting: false,
    error: null,

    fetchProfiles: async (status, nameSearch) => {
        set({ isLoading: true, error: null });
        try {
            const params = new URLSearchParams();
            if (status) params.append('status', status);
            if (nameSearch) params.append('name', nameSearch);
            
            const res = await api.get(`/smtp-profiles?${params.toString()}`);
            set({ profiles: res.data.data || [], isLoading: false });
        } catch (err: any) {
            set({ error: err.response?.data?.error?.message || 'Failed to fetch profiles', isLoading: false });
            toast.error('Failed to load SMTP profiles');
        }
    },

    getProfile: async (id: string) => {
        set({ isLoading: true, error: null });
        try {
            const res = await api.get(`/smtp-profiles/${id}`);
            const profile = res.data.data;
            set({ currentProfile: profile, isLoading: false });
            return profile;
        } catch (err: any) {
            set({ error: err.response?.data?.error?.message || 'Failed to fetch profile', isLoading: false });
            toast.error('Failed to load SMTP profile');
            return null;
        }
    },

    createProfile: async (data: any) => {
        set({ isLoading: true, error: null });
        try {
            const res = await api.post('/smtp-profiles', data);
            const newProfile = res.data.data;
            set((state) => ({ 
                profiles: [newProfile, ...state.profiles], 
                isLoading: false 
            }));
            toast.success('SMTP profile created securely');
            return newProfile;
        } catch (err: any) {
            const msg = err.response?.data?.error?.message || 'Failed to create profile';
            set({ error: msg, isLoading: false });
            toast.error(msg);
            return null;
        }
    },

    updateProfile: async (id: string, data: any) => {
        set({ isLoading: true, error: null });
        try {
            const res = await api.put(`/smtp-profiles/${id}`, data);
            const updated = res.data.data;
            set((state) => ({
                profiles: state.profiles.map(p => p.id === id ? updated : p),
                currentProfile: updated,
                isLoading: false
            }));
            toast.success('SMTP profile updated');
            return updated;
        } catch (err: any) {
            const msg = err.response?.data?.error?.message || 'Failed to update profile';
            set({ error: msg, isLoading: false });
            toast.error(msg);
            return null;
        }
    },

    deleteProfile: async (id: string) => {
        set({ isLoading: true, error: null });
        try {
            await api.delete(`/smtp-profiles/${id}`);
            set((state) => ({
                profiles: state.profiles.filter(p => p.id !== id),
                isLoading: false
            }));
            toast.success('SMTP profile deleted');
            return true;
        } catch (err: any) {
            const msg = err.response?.data?.error?.message || 'Failed to delete profile';
            set({ error: msg, isLoading: false });
            toast.error(msg);
            return false;
        }
    },

    duplicateProfile: async (id: string, newName: string) => {
        set({ isLoading: true, error: null });
        try {
            const res = await api.post(`/smtp-profiles/${id}/duplicate`, { new_name: newName });
            const dup = res.data.data;
            set((state) => ({
                profiles: [dup, ...state.profiles],
                isLoading: false
            }));
            toast.success('Profile duplicated successfully');
            return dup;
        } catch (err: any) {
            const msg = err.response?.data?.error?.message || 'Failed to duplicate profile';
            set({ error: msg, isLoading: false });
            toast.error(msg);
            return null;
        }
    },

    testProfile: async (id: string) => {
        set({ isTesting: true });
        try {
            const res = await api.post(`/smtp-profiles/${id}/test`);
            const result = res.data.data;
            
            if (result.success) {
                toast.success('Connection successful!');
            } else {
                toast.error(`Test failed at stage: ${result.stage_reached}`);
            }

            // After a test, we should refresh the profiles list to get the new status
            await get().fetchProfiles();
            if (get().currentProfile?.id === id) {
                await get().getProfile(id);
            }

            set({ isTesting: false });
            return result;
        } catch (err: any) {
            const msg = err.response?.data?.error?.message || 'Network error during test';
            toast.error(msg);
            set({ isTesting: false });
            return null;
        }
    },

    clearCurrent: () => set({ currentProfile: null })
}));
