import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Search, Table, Calendar, MoreVertical, ShieldAlert } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../services/api';

interface CampaignDesc {
    id: string;
    name: string;
    status: string;
    targets: number;
    launchDate: string | null;
    owner: string;
}

export default function CampaignList() {
    const [viewMode, setViewMode] = useState<'table' | 'calendar'>('table');
    const [search, setSearch] = useState('');
    const navigate = useNavigate();

    const { data: campaigns, isLoading } = useQuery({
        queryKey: ['campaigns'],
        queryFn: async () => {
            try {
                const res = await api.get('/campaigns');
                if (!res.data?.data) return [];
                
                return res.data.data.map((c: any) => ({
                    id: c.id,
                    name: c.name,
                    status: c.status,
                    targets: 0,
                    launchDate: c.start_date || null,
                    owner: c.created_by || 'Unknown'
                })) as CampaignDesc[];
            } catch (err) {
                console.error("Failed to load campaigns:", err);
                return [];
            }
        }
    });

    const filteredCampaigns = campaigns?.filter(c => c.name.toLowerCase().includes(search.toLowerCase())) || [];

    const handleCreateNew = async () => {
        // Implement modal later -> directly POST draft for MVP flow
        try {
            const res = await api.post('/campaigns', { name: 'New Campaign ' + Date.now() });
            if (res.data?.id) navigate(`/campaigns/${res.data.id}`);
        } catch(e) {
            console.error('Failed creating campaign', e);
            // fallback for missing POST
            navigate(`/campaigns/new-${Date.now()}`);
        }
    };

    return (
        <div className="max-w-7xl mx-auto space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-2xl font-bold text-white tracking-tight">Campaigns</h1>
                <button 
                    onClick={handleCreateNew}
                    className="flex items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white px-4 py-2 rounded-md font-medium transition-colors"
                >
                    <Plus className="w-5 h-5" />
                    New Campaign
                </button>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden flex flex-col">
                {/* List Toolbar */}
                <div className="p-4 border-b border-slate-800 flex items-center justify-between gap-4">
                    <div className="flex items-center gap-4 flex-1">
                        <div className="relative max-w-sm w-full">
                            <Search className="w-4 h-4 absolute left-3 top-2.5 text-slate-500" />
                            <input 
                                type="text"
                                placeholder="Search campaigns..."
                                className="w-full bg-slate-950 border border-slate-800 rounded-md pl-10 pr-4 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
                                value={search}
                                onChange={e => setSearch(e.target.value)}
                            />
                        </div>
                        <select className="bg-slate-950 border border-slate-800 rounded-md px-3 py-2 text-sm text-slate-300 focus:outline-none">
                            <option>Status: All Active</option>
                            <option>Status: Draft</option>
                            <option>Status: Completed</option>
                        </select>
                    </div>

                    <div className="flex bg-slate-950 p-1 rounded-md border border-slate-800">
                        <button 
                            className={`p-1.5 rounded flex items-center gap-2 text-sm px-3 ${viewMode === 'table' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-slate-200'}`}
                            onClick={() => setViewMode('table')}
                        >
                            <Table className="w-4 h-4" /> Table
                        </button>
                        <button 
                            className={`p-1.5 rounded flex items-center gap-2 text-sm px-3 ${viewMode === 'calendar' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-slate-200'}`}
                            onClick={() => setViewMode('calendar')}
                        >
                            <Calendar className="w-4 h-4" /> Cal
                        </button>
                    </div>
                </div>

                {/* Table Data Matrix */}
                {viewMode === 'table' && (
                    <div className="overflow-x-auto">
                        <table className="w-full text-left text-sm whitespace-nowrap">
                            <thead className="bg-slate-950/50 text-slate-400 border-b border-slate-800">
                                <tr>
                                    <th className="px-4 py-3 font-medium w-10">
                                        <input type="checkbox" className="rounded border-slate-700 bg-slate-900" />
                                    </th>
                                    <th className="px-4 py-3 font-medium">Name</th>
                                    <th className="px-4 py-3 font-medium">Status</th>
                                    <th className="px-4 py-3 font-medium text-right">Targets</th>
                                    <th className="px-4 py-3 font-medium">Launch</th>
                                    <th className="px-4 py-3 font-medium">Owner</th>
                                    <th className="px-4 py-3 font-medium w-12"></th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-800 break-words">
                                {isLoading ? (
                                    <tr><td colSpan={7} className="px-4 py-8 text-center text-slate-500">Loading pipelines...</td></tr>
                                ) : filteredCampaigns.length === 0 ? (
                                    <tr>
                                        <td colSpan={7} className="px-4 py-16 text-center text-slate-400">
                                            <div className="flex flex-col items-center justify-center">
                                                <ShieldAlert className="w-12 h-12 text-slate-600 mb-4" />
                                                <p className="text-lg text-slate-300 font-medium mb-1">No campaigns match your filters</p>
                                                <p className="text-sm">Create your first phishing campaign to start testing your organization.</p>
                                            </div>
                                        </td>
                                    </tr>
                                ) : (
                                    filteredCampaigns.map(camp => (
                                        <tr 
                                            key={camp.id} 
                                            className="hover:bg-slate-800/50 cursor-pointer transition-colors"
                                            onClick={() => navigate(`/campaigns/${camp.id}`)}
                                        >
                                            <td className="px-4 py-3" onClick={e => e.stopPropagation()}>
                                                <input type="checkbox" className="rounded border-slate-700 bg-slate-900" />
                                            </td>
                                            <td className="px-4 py-3 font-medium text-slate-200">{camp.name}</td>
                                            <td className="px-4 py-3 flex items-center gap-2">
                                                <span className={`w-2 h-2 rounded-full ${compStatusColor(camp.status)}`} />
                                                <span className="uppercase text-xs tracking-wider font-semibold text-slate-400">{camp.status}</span>
                                            </td>
                                            <td className="px-4 py-3 text-right text-slate-400">{camp.targets > 0 ? camp.targets : '—'}</td>
                                            <td className="px-4 py-3 text-slate-400">{camp.launchDate || '—'}</td>
                                            <td className="px-4 py-3 text-slate-400">{camp.owner}</td>
                                            <td className="px-4 py-3 text-slate-500 text-center" onClick={e => e.stopPropagation()}>
                                                <button className="p-1 hover:bg-slate-700 rounded"><MoreVertical className="w-4 h-4" /></button>
                                            </td>
                                        </tr>
                                    ))
                                )}
                            </tbody>
                        </table>
                    </div>
                )}
            </div>
        </div>
    );
}

function compStatusColor(s: string) {
    switch (s) {
        case 'active': return 'bg-blue-500';
        case 'draft': return 'bg-slate-500';
        case 'building': return 'bg-amber-500';
        case 'completed': return 'bg-green-500';
        default: return 'bg-slate-500';
    }
}
