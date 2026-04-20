import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Search, Table as TableIcon, Calendar, MoreVertical, ShieldAlert } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../services/api';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Badge } from '../../components/ui/Badge';
import { Card } from '../../components/ui/Card';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '../../components/ui/Table';

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
                <Button onClick={handleCreateNew} variant="primary"  variant="outline">
                    <Plus className="w-5 h-5" />
                    New Campaign
                </Button>
            </div>

            <Card className="flex flex-col shadow-xl">
                {/* List Toolbar */}
                <div className="p-4 border-b border-slate-800 flex items-center justify-between gap-4">
                    <div className="flex items-center gap-4 flex-1">
                        <div className="relative max-w-sm w-full">
                            <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 z-10" />
                            <Input 
                                type="text"
                                placeholder="Search campaigns..."
                                
                                value={search}
                                onChange={e => setSearch(e.target.value)}
                            />
                        </div>
                        <div className="w-48">
                            <Select>
                                <option>Status: All Active</option>
                                <option>Status: Draft</option>
                                <option>Status: Completed</option>
                            </Select>
                        </div>
                    </div>

                    <div className="flex bg-slate-950 p-1 rounded-md border border-slate-800">
                        <Button variant="ghost" 
                            className={`p-1.5 rounded flex items-center gap-2 text-sm px-3 ${viewMode === 'table' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-slate-200'}`}
                            onClick={ () => setViewMode('table')}
                        >
                            <TableIcon className="w-4 h-4" /> Table
                        </Button>
                        <Button variant="ghost" 
                            className={`p-1.5 rounded flex items-center gap-2 text-sm px-3 ${viewMode === 'calendar' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-slate-200'}`}
                            onClick={ () => setViewMode('calendar')}
                        >
                            <Calendar className="w-4 h-4" /> Cal
                        </Button>
                    </div>
                </div>

                {/* Table Data Matrix */}
                {viewMode === 'table' && (
                    <Table className="border-0 shadow-none rounded-none rounded-b-xl border-t-0 p-0">
                        <TableHeader className="bg-slate-950/50">
                            <TableRow className="hover:bg-transparent">
                                <TableHead className="w-10">
                                    <input type="checkbox" className="rounded border-slate-700 bg-slate-900" />
                                </TableHead>
                                <TableHead>Name</TableHead>
                                <TableHead>Status</TableHead>
                                <TableHead className="text-right">Targets</TableHead>
                                <TableHead>Launch</TableHead>
                                <TableHead>Owner</TableHead>
                                <TableHead className="w-12"></TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {isLoading ? (
                                <TableRow><TableCell colSpan={7} className="py-8 text-center text-slate-500">Loading pipelines...</TableCell></TableRow>
                            ) : filteredCampaigns.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={7} className="py-16 text-center text-slate-400">
                                        <div className="flex flex-col items-center justify-center">
                                            <ShieldAlert className="w-12 h-12 text-slate-600 mb-4" />
                                            <p className="text-lg text-slate-300 font-medium mb-1">No campaigns match your filters</p>
                                            <p className="text-sm">Create your first phishing campaign to start testing your organization.</p>
                                        </div>
                                    </TableCell>
                                </TableRow>
                            ) : (
                                filteredCampaigns.map(camp => (
                                    <TableRow 
                                        key={camp.id} 
                                        className="cursor-pointer"
                                        onClick={() => navigate(`/campaigns/${camp.id}`)}
                                    >
                                        <TableCell onClick={e => e.stopPropagation()}>
                                            <input type="checkbox" className="rounded border-slate-700 bg-slate-900" />
                                        </TableCell>
                                        <TableCell className="font-medium text-slate-200">{camp.name}</TableCell>
                                        <TableCell>
                                            <Badge variant={compStatusVariant(camp.status) as any}>
                                                <div className={`w-1.5 h-1.5 mr-1.5 rounded-full ${compStatusColor(camp.status)}`} />
                                                {camp.status}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="text-right text-slate-400">{camp.targets > 0 ? camp.targets : '—'}</TableCell>
                                        <TableCell className="text-slate-400">{camp.launchDate || '—'}</TableCell>
                                        <TableCell className="text-slate-400">{camp.owner}</TableCell>
                                        <TableCell className="text-center" onClick={e => e.stopPropagation()}>
                                            <Button variant="ghost" size="icon" variant="outline">
                                                <MoreVertical className="w-4 h-4" />
                                            </Button>
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                )}
            </Card>
        </div>
    );
}

function compStatusVariant(s: string) {
    switch (s) {
        case 'active': return 'default';
        case 'draft': return 'secondary';
        case 'building': return 'warning';
        case 'completed': return 'success';
        default: return 'outline';
    }
}

function compStatusColor(s: string) {
    switch (s) {
        case 'active': return 'bg-blue-400';
        case 'draft': return 'bg-slate-400';
        case 'building': return 'bg-amber-400';
        case 'completed': return 'bg-green-400';
        default: return 'bg-slate-400';
    }
}
