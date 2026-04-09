import { create } from 'zustand';
import { api } from '../services/api';
import toast from 'react-hot-toast';

export interface TargetGroup {
    id: string;
    name: string;
    count: number;
    added: string;
}

export interface EmailVariant {
    id: string;
    label: string;
    percentage: number;
    template: string | null;
}

export interface Infrastructure {
    provider: string;
    region: string;
    instanceSize: string;
    domain: string;
}

export interface Schedule {
    startDate: string;
    startTime: string;
    pacingMode: string;
    maxBatchesPerHour: number;
    maxEmailsPerBatch: number;
}

export interface CampaignDraft {
    targetGroups: TargetGroup[];
    canaryTargets: string[];
    emailVariants: EmailVariant[];
    landingPageId: string | null;
    infrastructure: Infrastructure;
    schedule: Schedule;
}

export interface CampaignDetails {
    id: string;
    name: string;
    status: string;
    owner: string;
    createdAt: string;
    updatedAt: string;
}

interface CampaignStore {
    campaign: CampaignDetails | null;
    draft: CampaignDraft;
    isSaving: boolean;
    setDraft: (partialDraft: Partial<CampaignDraft>) => void;
    updateTargetGroups: (groups: CampaignDraft['targetGroups']) => void;
    updateEmailVariants: (variants: CampaignDraft['emailVariants']) => void;
    updateCanaryTargets: (targets: string[]) => void;
    updateLandingPage: (id: string | null) => void;
    updateInfrastructure: (infra: Partial<CampaignDraft['infrastructure']>) => void;
    updateSchedule: (schedule: Partial<CampaignDraft['schedule']>) => void;
    saveCampaign: (campaignId: string | undefined) => Promise<void>;
    fetchCampaign: (campaignId: string) => Promise<void>;
    isLoading: boolean;
}

const emptyInitialData: CampaignDraft = {
    targetGroups: [],
    canaryTargets: [],
    emailVariants: [
        { id: '1', label: 'Variant A', percentage: 100, template: null }
    ],
    landingPageId: null,
    infrastructure: {
        provider: '',
        region: '',
        instanceSize: '',
        domain: ''
    },
    schedule: {
        startDate: '',
        startTime: '',
        pacingMode: 'Staggered',
        maxBatchesPerHour: 20,
        maxEmailsPerBatch: 10
    }
};

export const useCampaignStore = create<CampaignStore>((set, get) => ({
    campaign: null,
    draft: emptyInitialData,
    isSaving: false,
    isLoading: false,
    
    setDraft: (partial) => set((state) => ({ draft: { ...state.draft, ...partial } })),
    
    updateTargetGroups: (groups) => set((state) => ({ draft: { ...state.draft, targetGroups: groups } })),
    
    updateEmailVariants: (variants) => set((state) => ({ draft: { ...state.draft, emailVariants: variants } })),
    
    updateCanaryTargets: (targets) => set((state) => ({ draft: { ...state.draft, canaryTargets: targets } })),

    updateLandingPage: (id) => set((state) => ({ draft: { ...state.draft, landingPageId: id } })),
    
    updateInfrastructure: (infra) => set((state) => ({ 
        draft: { ...state.draft, infrastructure: { ...state.draft.infrastructure, ...infra } } 
    })),
    
    updateSchedule: (schedule) => set((state) => ({ 
        draft: { ...state.draft, schedule: { ...state.draft.schedule, ...schedule } } 
    })),
    
    saveCampaign: async (campaignId) => {
        if (!campaignId) return;
        set({ isSaving: true });
        try {
            const draft = get().draft;
            
            // 1. Update Core Campaign (Includes Infrastructure, Schedule, and Canaries in Config)
            const updatePayload = {
               landing_page_id: draft.landingPageId || undefined,
               cloud_provider: draft.infrastructure.provider || undefined,
               region: draft.infrastructure.region || undefined,
               instance_type: draft.infrastructure.instanceSize || undefined,
               endpoint_domain_id: draft.infrastructure.domain || undefined,
               start_date: draft.schedule.startDate ? new Date(draft.schedule.startDate).toISOString() : undefined,
               configuration: {
                   canary_targets: draft.canaryTargets,
                   schedule: draft.schedule
               }
            };
            
            if (Object.keys(updatePayload).some(key => updatePayload[key as keyof typeof updatePayload] !== undefined)) {
                await api.put(`/campaigns/${campaignId}`, updatePayload);
            }

            // 2. Update Template Variants
            if (draft.emailVariants.length > 0) {
                const variantsPayload = draft.emailVariants
                    .filter(v => v.template)
                    .map(v => ({
                        template_id: v.template,
                        split_ratio: Number(v.percentage),
                        label: v.label
                    }));
                
                if (variantsPayload.length > 0) {
                    await api.put(`/campaigns/${campaignId}/template-variants`, { variants: variantsPayload });
                }
            }
            
            // 3. Assign Target Groups
            if (draft.targetGroups.length > 0) {
                for (const group of draft.targetGroups) {
                    try {
                        await api.post(`/campaigns/${campaignId}/target-groups`, { group_id: group.id });
                    } catch (e: any) {
                        // Ignore 409 Conflict if already assigned
                        if (e.response?.status !== 409) {
                            throw e;
                        }
                    }
                }
            }
            
            toast.success('Campaign configurations saved automatically.');
        } catch (err) {
            console.error('Failed to save campaign configurations:', err);
            toast.error('Failed to save campaign. Please try again.');
        } finally {
            set({ isSaving: false });
        }
    },
    
    fetchCampaign: async (campaignId) => {
        set({ isLoading: true });
        try {
            const res = await api.get(`/campaigns/${campaignId}`);
            const data = res.data?.data;
            if (!data) return;

            const variantsRes = await api.get(`/campaigns/${campaignId}/template-variants`);
            const variants = variantsRes.data?.data || [];

            set(() => ({
                campaign: {
                    id: data.id,
                    name: data.name,
                    status: data.status || 'draft',
                    owner: data.created_by || 'Unknown',
                    createdAt: data.created_at || new Date().toISOString(),
                    updatedAt: data.updated_at || new Date().toISOString(),
                },
                draft: {
                    ...emptyInitialData,
                    landingPageId: data.landing_page_id || null,
                    infrastructure: {
                        provider: data.cloud_provider || '',
                        region: data.region || '',
                        instanceSize: data.instance_type || '',
                        domain: data.endpoint_domain_id || ''
                    },
                    canaryTargets: data.configuration?.canary_targets || emptyInitialData.canaryTargets,
                    schedule: {
                        ...emptyInitialData.schedule,
                        ...(data.configuration?.schedule || {}),
                        startDate: data.start_date ? data.start_date.split('T')[0] : '',
                    },
                    emailVariants: variants.length > 0 ? variants.map((v: any) => ({
                        id: v.id,
                        label: v.label,
                        percentage: v.split_ratio,
                        template: v.template_id
                    })) : emptyInitialData.emailVariants
                }
            }));
        } catch (err) {
            console.error('Failed to fetch campaign:', err);
        } finally {
            set({ isLoading: false });
        }
    }
}));
