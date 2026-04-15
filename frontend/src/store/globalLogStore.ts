import { create } from 'zustand';
import { api } from '../services/api';
import toast from 'react-hot-toast';

export interface OmnibusLogDTO {
    id: string;
    timestamp: string;
    source: 'audit' | 'application' | 'endpoint';
    severity: string;
    actor: string;
    action_msg: string;
    payload: Record<string, any>;
}

export interface GlobalLogFilters {
    search?: string;
    source?: string;
    severity?: string;
    limit: number;
}

interface GlobalLogStore {
    logs: OmnibusLogDTO[];
    isLoading: boolean;
    hasMore: boolean;
    nextCursor: string | null;
    filters: GlobalLogFilters;

    wsRef: WebSocket | null;

    setFilters: (newFilters: Partial<GlobalLogFilters>) => void;
    fetchLogs: (loadMore?: boolean) => Promise<void>;
    
    connectWS: () => void;
    disconnectWS: () => void;
}

export const useGlobalLogStore = create<GlobalLogStore>((set, get) => ({
    logs: [],
    isLoading: false,
    hasMore: true,
    nextCursor: null,
    filters: {
        limit: 100,
        search: '',
        source: '',
        severity: ''
    },
    wsRef: null,

    setFilters: (newFilters) => {
        set((state) => ({ 
            filters: { ...state.filters, ...newFilters },
            nextCursor: null, 
            hasMore: true 
        }));
        get().fetchLogs(false);
    },

    fetchLogs: async (loadMore = false) => {
        set({ isLoading: true });
        try {
            const { filters, nextCursor, logs } = get();
            const params = new URLSearchParams();
            
            params.append('limit', filters.limit.toString());
            
            if (filters.search) params.append('search', filters.search);
            if (filters.source) params.append('source', filters.source);
            if (filters.severity) params.append('severity', filters.severity);

            if (loadMore && nextCursor) {
                params.append('cursor', nextCursor);
            }

            const res = await api.get(`/logs/omnibus?${params.toString()}`);
            
            const payload = res.data.data;
            const logArray = Array.isArray(payload?.data) ? payload.data : [];

            set({ 
                logs: loadMore ? [...logs, ...logArray] : logArray,
                nextCursor: payload?.next_cursor || null,
                hasMore: !!payload?.next_cursor,
                isLoading: false 
            });
        } catch (err: any) {
            toast.error('Failed to load global logs');
            set({ isLoading: false });
        }
    },

    connectWS: () => {
        if (get().wsRef) return; // Already connected
        
        const token = localStorage.getItem('token');
        if (!token) return;

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//localhost:8080/api/v1/ws`;
        
        const ws = new WebSocket(wsUrl);
        
        ws.onopen = () => {
            ws.send(JSON.stringify({ type: 'auth', token }));
        };

        ws.onmessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                
                if (message.type === 'auth_ok') {
                    return;
                }

                // Handle Audit Log WS Frame
                if (message.type === 'audit_log_new') {
                    const partialLog = message.data;
                    const realtimeLog: OmnibusLogDTO = {
                        id: partialLog.id,
                        timestamp: partialLog.timestamp,
                        source: 'audit',
                        severity: partialLog.severity || 'info',
                        action_msg: partialLog.action || 'Unknown Event',
                        actor: partialLog.actor_label || 'System',
                        payload: { "_ws": "live stream placeholder" }
                    };

                    set((state) => ({ logs: [realtimeLog, ...state.logs] }));
                }

                // Handle App Log WS Frame (slog output)
                if (message.type === 'app_log_new') {
                    const partialLog = message.data;
                    const realtimeLog: OmnibusLogDTO = {
                        id: `app-ws-${Date.now()}`,
                        timestamp: partialLog.timestamp,
                        source: 'application',
                        severity: partialLog.level ? partialLog.level.toLowerCase() : 'info',
                        action_msg: partialLog.message || 'System Log',
                        actor: 'System',
                        payload: { "_ws": "live stream placeholder" }
                    };

                    set((state) => ({ logs: [realtimeLog, ...state.logs] }));
                }

                // Endpoint traffic bursting could be added here in the future
                // if (message.type === 'endpoint_log_new') { ... }

            } catch (err) {
                console.error("WS Parse Error: ", err);
            }
        };

        ws.onclose = () => {
            set({ wsRef: null });
        };

        set({ wsRef: ws });
    },

    disconnectWS: () => {
        const { wsRef } = get();
        if (wsRef) {
            wsRef.close();
            set({ wsRef: null });
        }
    }
}));
