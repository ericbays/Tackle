import { useEffect, useState } from 'react';
import { useBlocklistStore } from '../../../store/blocklistStore';
import { ShieldCheck, ShieldAlert, Plus, Search, Loader2 } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Badge } from '../../../components/ui/Badge';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../../../components/ui/Card';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '../../../components/ui/Table';

export default function BlocklistManager() {
    const { rules, isLoading, fetchRules, filters, setFilters, totalPages, createRule, deactivateRule, reactivateRule } = useBlocklistStore();
    
    const [isCreating, setIsCreating] = useState(false);
    const [newPattern, setNewPattern] = useState('');
    const [newReason, setNewReason] = useState('');

    useEffect(() => {
        fetchRules();
    }, [fetchRules]);

    const handleCreate = async () => {
        if (!newPattern.trim()) return;
        const success = await createRule(newPattern, newReason);
        if (success) {
            setIsCreating(false);
            setNewPattern('');
            setNewReason('');
        }
    };

    const toggleRule = (id: string, currentlyActive: boolean) => {
        if (currentlyActive) {
            deactivateRule(id);
        } else {
            reactivateRule(id);
        }
    };

    return (
        <div className="space-y-6">
            <Card className="shadow-2xl flex flex-col p-6">
                <div className="flex justify-between items-start mb-6 border-b border-slate-800 pb-4">
                    <div>
                        <CardTitle className="text-lg font-bold text-white flex items-center gap-2">
                            <ShieldAlert className="w-5 h-5 text-red-500" />
                            Global Target Suppression
                        </CardTitle>
                        <CardDescription className="text-sm text-slate-400 mt-1">Configure RegEx patterns or exact email addresses to globally enforce bounce/blocklists across all campaigns.</CardDescription>
                    </div>

                    <Button 
                        onClick={() => setIsCreating(true)}
                        className="bg-red-500/10 text-red-400 hover:bg-red-500/20 px-4 py-2 flex items-center gap-2"
                    >
                        <Plus className="w-4 h-4" /> Add Rule
                    </Button>
                </div>

                {isCreating && (
                    <div className="bg-slate-800 border border-slate-700 rounded-lg p-4 mb-6 flex gap-4 items-end animate-in fade-in slide-in-from-top-4">
                        <div className="flex-1 space-y-1.5">
                            <label className="text-xs font-semibold text-slate-400">Match Pattern (RegEx or exact match)</label>
                            <Input 
                                type="text" 
                                autoFocus
                                className="w-full bg-slate-900/50 focus:border-red-500"
                                placeholder="*@domain.com or exact@email.com..."
                                value={newPattern}
                                onChange={(e) => setNewPattern(e.target.value)}
                            />
                        </div>
                        <div className="flex-1 space-y-1.5">
                            <label className="text-xs font-semibold text-slate-400">Administrative Reason</label>
                            <Input 
                                type="text" 
                                className="w-full bg-slate-900/50 focus:border-red-500"
                                placeholder="Requested by HR..."
                                value={newReason}
                                onChange={(e) => setNewReason(e.target.value)}
                            />
                        </div>
                        <div className="flex gap-2">
                            <Button 
                                onClick={() => setIsCreating(false)}
                                variant="ghost"
                            >
                                Cancel
                            </Button>
                            <Button 
                                onClick={handleCreate}
                                disabled={!newPattern}
                                className="bg-red-600 hover:bg-red-500 text-white"
                            >
                                Enforce Pattern
                            </Button>
                        </div>
                    </div>
                )}

                <div className="flex gap-4 mb-4">
                    <div className="relative flex-1">
                        <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 z-10" />
                        <Input 
                            type="text" 
                            className="bg-slate-800 focus:border-red-500 pl-9"
                            placeholder="Search existing patterns..."
                            value={filters.pattern || ''}
                            onChange={(e) => setFilters({ pattern: e.target.value })}
                        />
                    </div>
                    <div className="w-48">
                        <Select 
                            value={filters.is_active || ''}
                            onChange={(e) => setFilters({ is_active: e.target.value })}
                            className="bg-slate-800 focus:border-red-500"
                        >
                            <option value="">All States</option>
                            <option value="true">Active Only</option>
                            <option value="false">Inactive Only</option>
                        </Select>
                    </div>
                </div>

                <div className="overflow-x-auto border border-slate-800 rounded-xl">
                    <Table className="w-full border-0 shadow-none rounded-none text-left text-sm text-slate-300">
                        <TableHeader className="bg-slate-800/50">
                            <TableRow className="hover:bg-transparent text-xs uppercase font-semibold text-slate-400 border-b border-slate-800">
                                <TableHead>Threat Pattern</TableHead>
                                <TableHead>Type</TableHead>
                                <TableHead>Reason</TableHead>
                                <TableHead>Status</TableHead>
                                <TableHead className="text-right">Actions</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {isLoading ? (
                                <TableRow>
                                    <TableCell colSpan={5} className="py-12 text-center text-slate-500">
                                        <Loader2 className="w-8 h-8 animate-spin mx-auto text-red-500 mb-2" />
                                        Loading suppression rules...
                                    </TableCell>
                                </TableRow>
                            ) : rules.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={5} className="py-12 text-center text-slate-500">No blocklist patterns established.</TableCell>
                                </TableRow>
                            ) : (
                                rules.map((rule) => (
                                    <TableRow key={rule.id}>
                                        <TableCell className="font-mono text-slate-200">
                                            {rule.pattern}
                                        </TableCell>
                                        <TableCell className="text-xs tracking-wider uppercase text-slate-500">
                                            {rule.pattern_type}
                                        </TableCell>
                                        <TableCell>
                                            <div className="text-slate-300">{rule.reason || <span className="italic text-slate-600">No justification</span>}</div>
                                            <div className="text-slate-500 text-[10px] mt-1 uppercase">By: {rule.created_by}</div>
                                        </TableCell>
                                        <TableCell>
                                            {rule.is_active ? (
                                                <Badge variant="destructive" className="bg-red-500/10 text-red-400 border-red-500/20 font-bold uppercase text-[10px] gap-1.5 flex items-center max-w-fit">
                                                    <ShieldAlert className="w-3 h-3" /> Active Block
                                                </Badge>
                                            ) : (
                                                <Badge variant="secondary" className="bg-slate-800/30 text-slate-400 border-slate-600/30 font-bold uppercase text-[10px] gap-1.5 flex items-center max-w-fit">
                                                    <ShieldCheck className="w-3 h-3" /> Off/Bypassed
                                                </Badge>
                                            )}
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <Button 
                                                onClick={() => toggleRule(rule.id, rule.is_active)}
                                                variant={rule.is_active ? "ghost" : "destructive"}
                                                className={`text-xs font-semibold px-3 py-1.5 ${rule.is_active ? '' : 'bg-red-500/10 hover:bg-red-500/20 text-red-500'}`}
                                            >
                                                {rule.is_active ? 'Deactivate' : 'Reactivate'}
                                            </Button>
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </div>
                
                {rules.length > 0 && (
                    <div className="pt-4 flex items-center justify-between border-t border-slate-800 mt-4">
                        <div className="text-xs text-slate-400">
                            Page {filters.page} of {totalPages}
                        </div>
                        <div className="flex gap-2">
                            <Button
                                variant="secondary"
                                disabled={filters.page === 1}
                                onClick={() => setFilters({ page: (filters.page || 1) - 1 })}
                            >
                                Prev
                            </Button>
                            <Button
                                variant="secondary"
                                disabled={filters.page === totalPages}
                                onClick={() => setFilters({ page: (filters.page || 1) + 1 })}
                            >
                                Next
                            </Button>
                        </div>
                    </div>
                )}
            </Card>
        </div>
    );
}
