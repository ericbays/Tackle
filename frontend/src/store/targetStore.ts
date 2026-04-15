import { create } from 'zustand';
import { api } from '../services/api';
import toast from 'react-hot-toast';

export interface TargetDTO {
    id: string;
    email: string;
    first_name?: string;
    last_name?: string;
    department?: string;
    title?: string;
    custom_fields: Record<string, any>;
    created_by: string;
    deleted_at?: string;
    created_at: string;
    updated_at: string;
}

export interface TargetFilters {
    page: number;
    per_page: number;
    email?: string;
    first_name?: string;
    last_name?: string;
    department?: string;
}

interface TargetStore {
    targets: TargetDTO[];
    total: number;
    totalPages: number;
    isLoading: boolean;
    filters: TargetFilters;
    
    setFilters: (newFilters: Partial<TargetFilters>) => void;
    fetchTargets: () => Promise<void>;
    createTarget: (data: any) => Promise<boolean>;
    updateTarget: (id: string, data: any) => Promise<boolean>;
    bulkDelete: (ids: string[]) => Promise<boolean>;
    bulkExport: (ids?: string[]) => Promise<void>;
    uploadCSV: (file: File) => Promise<boolean>;
}

export const useTargetStore = create<TargetStore>((set, get) => ({
    targets: [],
    total: 0,
    totalPages: 1,
    isLoading: false,
    filters: {
        page: 1,
        per_page: 25,
        email: '',
        first_name: '',
        last_name: '',
        department: ''
    },

    setFilters: (newFilters) => {
        set((state) => ({ filters: { ...state.filters, ...newFilters, page: newFilters.page || 1 } }));
        get().fetchTargets();
    },

    fetchTargets: async () => {
        set({ isLoading: true });
        try {
            const { filters } = get();
            const params = new URLSearchParams();
            
            params.append('page', filters.page.toString());
            params.append('per_page', filters.per_page.toString());
            
            if (filters.email) params.append('email', filters.email);
            if (filters.first_name) params.append('first_name', filters.first_name);
            if (filters.last_name) params.append('last_name', filters.last_name);
            if (filters.department) params.append('department', filters.department);

            const res = await api.get(`/targets?${params.toString()}`);
            
            set({ 
                targets: res.data.data || [], 
                total: res.data.meta?.total || 0,
                totalPages: res.data.meta?.total_pages || 1,
                isLoading: false 
            });
        } catch (err: any) {
            toast.error('Failed to load targets');
            set({ isLoading: false });
        }
    },

    createTarget: async (data: any) => {
        try {
            await api.post('/targets', data);
            toast.success('Target created successfully');
            get().fetchTargets();
            return true;
        } catch (err: any) {
            toast.error(err.response?.data?.error?.message || 'Failed to create target');
            return false;
        }
    },

    updateTarget: async (id: string, data: any) => {
        try {
            await api.put(`/targets/${id}`, data);
            toast.success('Target updated successfully');
            
            // Instantly update local array without a full refetch to prevent re-rendering the whole page
            const updatedTargets = get().targets.map(t => t.id === id ? { ...t, ...data, updated_at: new Date().toISOString() } : t);
            set({ targets: updatedTargets });
            
            return true;
        } catch (err: any) {
            toast.error(err.response?.data?.error?.message || 'Failed to update target');
            return false;
        }
    },

    bulkDelete: async (ids: string[]) => {
        try {
            // Backend BulkDelete payload: { target_ids: [], confirm: true }
            await api.post('/targets/bulk/delete', {
                target_ids: ids,
                confirm: true
            });
            toast.success(`Successfully deleted ${ids.length} targets`);
            get().fetchTargets();
            return true;
        } catch (err: any) {
            toast.error(err.response?.data?.error?.message || 'Failed to delete targets');
            return false;
        }
    },

    bulkExport: async (ids?: string[]) => {
        try {
            const res = await api.post('/targets/bulk/export', 
                { target_ids: ids || [] },
                { responseType: 'blob' }
            );
            
            // Create a blob link to download
            const url = window.URL.createObjectURL(new Blob([res.data]));
            const link = document.createElement('a');
            link.href = url;
            link.setAttribute('download', 'targets-export.csv');
            document.body.appendChild(link);
            link.click();
            link.remove();
            
            toast.success('Export downloaded');
        } catch (err) {
            toast.error('Failed to export targets');
        }
    },

    uploadCSV: async (file: File) => {
        try {
            const formData = new FormData();
            formData.append('file', file);
            
            // Step 1: Upload
            const uploadRes = await api.post('/targets/import/upload', formData, {
                headers: { 'Content-Type': 'multipart/form-data' }
            });
            const uploadId = uploadRes.data.data.upload_id;
            
            // Step 2: Submit strict mapping layout
            await api.post(`/targets/import/${uploadId}/mapping`, {
                mapping: {
                    email: "email",
                    first_name: "first_name",
                    last_name: "last_name",
                    department: "department",
                    title: "title"
                }
            });

            // Step 3: Validate
            await api.post(`/targets/import/${uploadId}/validate`);

            // Step 4: Commit
            await api.post(`/targets/import/${uploadId}/commit`);
            
            toast.success('CSV Import successful');
            get().fetchTargets();
            return true;
        } catch (err: any) {
            toast.error(err.response?.data?.error?.message || 'Failed to import CSV');
            return false;
        }
    }
}));
