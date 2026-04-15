import { create } from 'zustand';
import { api } from '../services/api';
import toast from 'react-hot-toast';

export interface BlocklistDTO {
    id: string;
    pattern: string;
    pattern_type: string;
    is_active: boolean;
    reason?: string;
    created_by: string;
    created_at: string;
    updated_at: string;
}

export interface BlocklistFilters {
    page: number;
    per_page: number;
    pattern?: string;
    is_active?: string; // 'true' | 'false' | ''
}

interface BlocklistStore {
    rules: BlocklistDTO[];
    total: number;
    totalPages: number;
    isLoading: boolean;
    filters: BlocklistFilters;

    setFilters: (newFilters: Partial<BlocklistFilters>) => void;
    fetchRules: () => Promise<void>;
    createRule: (pattern: string, reason?: string) => Promise<boolean>;
    deactivateRule: (id: string) => Promise<boolean>;
    reactivateRule: (id: string) => Promise<boolean>;
}

export const useBlocklistStore = create<BlocklistStore>((set, get) => ({
    rules: [],
    total: 0,
    totalPages: 1,
    isLoading: false,
    filters: {
        page: 1,
        per_page: 25,
        pattern: '',
        is_active: ''
    },

    setFilters: (newFilters) => {
        set((state) => ({ filters: { ...state.filters, ...newFilters, page: newFilters.page || 1 } }));
        get().fetchRules();
    },

    fetchRules: async () => {
        set({ isLoading: true });
        try {
            const { filters } = get();
            const params = new URLSearchParams();
            
            params.append('page', filters.page.toString());
            params.append('per_page', filters.per_page.toString());
            
            if (filters.pattern) params.append('pattern', filters.pattern);
            if (filters.is_active) params.append('is_active', filters.is_active);

            const res = await api.get(`/blocklist?${params.toString()}`);
            
            set({ 
                rules: res.data.data || [], 
                total: res.data.meta?.total || 0,
                totalPages: res.data.meta?.total_pages || 1,
                isLoading: false 
            });
        } catch (err: any) {
            toast.error('Failed to load blocklist rules');
            set({ isLoading: false });
        }
    },

    createRule: async (pattern: string, reason?: string) => {
        try {
            await api.post('/blocklist', { pattern, reason });
            toast.success('Blocklist rule actively enforced');
            get().fetchRules();
            return true;
        } catch (err: any) {
            toast.error(err.response?.data?.error?.message || 'Failed to create blocklist rule');
            return false;
        }
    },

    deactivateRule: async (id: string) => {
        try {
            await api.put(`/blocklist/${id}/deactivate`);
            toast.success('Rule deactivated');
            get().fetchRules();
            return true;
        } catch (err: any) {
            toast.error('Failed to deactivate rule');
            return false;
        }
    },

    reactivateRule: async (id: string) => {
        try {
            await api.put(`/blocklist/${id}/reactivate`);
            toast.success('Rule reactivated');
            get().fetchRules();
            return true;
        } catch (err: any) {
            toast.error('Failed to reactivate rule');
            return false;
        }
    }
}));
