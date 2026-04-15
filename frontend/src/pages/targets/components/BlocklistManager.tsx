import { useEffect, useState } from 'react';
import { useBlocklistStore } from '../../../store/blocklistStore';
import { ShieldCheck, ShieldAlert, Plus, Search, Loader2 } from 'lucide-react';

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
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 shadow-2xl">
                <div className="flex justify-between items-start mb-6">
                    <div>
                        <h2 className="text-lg font-bold text-white flex items-center gap-2">
                            <ShieldAlert className="w-5 h-5 text-red-500" />
                            Global Target Suppression
                        </h2>
                        <p className="text-sm text-slate-400 mt-1">Configure RegEx patterns or exact email addresses to globally enforce bounce/blocklists across all campaigns.</p>
                    </div>

                    <button 
                        onClick={() => setIsCreating(true)}
                        className="bg-red-500/10 text-red-400 hover:bg-red-500/20 px-4 py-2 rounded-lg text-sm font-semibold transition-colors flex items-center gap-2"
                    >
                        <Plus className="w-4 h-4" /> Add Rule
                    </button>
                </div>

                {isCreating && (
                    <div className="bg-slate-800 border border-slate-700 rounded-lg p-4 mb-6 flex gap-4 items-end animate-in fade-in slide-in-from-top-4">
                        <div className="flex-1 space-y-1.5">
                            <label className="text-xs font-semibold text-slate-400">Match Pattern (RegEx or exact match)</label>
                            <input 
                                type="text" 
                                autoFocus
                                className="w-full bg-slate-900/50 border border-slate-700/50 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-red-500"
                                placeholder="*@domain.com or exact@email.com..."
                                value={newPattern}
                                onChange={(e) => setNewPattern(e.target.value)}
                            />
                        </div>
                        <div className="flex-1 space-y-1.5">
                            <label className="text-xs font-semibold text-slate-400">Administrative Reason</label>
                            <input 
                                type="text" 
                                className="w-full bg-slate-900/50 border border-slate-700/50 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-red-500"
                                placeholder="Requested by HR..."
                                value={newReason}
                                onChange={(e) => setNewReason(e.target.value)}
                            />
                        </div>
                        <div className="flex gap-2">
                            <button 
                                onClick={() => setIsCreating(false)}
                                className="px-4 py-2 rounded-lg text-sm font-semibold text-slate-400 hover:text-white transition-colors"
                            >
                                Cancel
                            </button>
                            <button 
                                onClick={handleCreate}
                                disabled={!newPattern}
                                className="bg-red-600 hover:bg-red-500 text-white px-4 py-2 rounded-lg text-sm font-semibold transition-colors disabled:opacity-50"
                            >
                                Enforce Pattern
                            </button>
                        </div>
                    </div>
                )}

                <div className="flex gap-4 mb-4">
                    <div className="relative flex-1">
                        <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
                        <input 
                            type="text" 
                            className="w-full bg-slate-800 border border-slate-700 rounded-lg pl-9 pr-3 py-2 text-sm text-white focus:outline-none focus:border-red-500"
                            placeholder="Search existing patterns..."
                            value={filters.pattern}
                            onChange={(e) => setFilters({ pattern: e.target.value })}
                        />
                    </div>
                    <select 
                        className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-red-500"
                        value={filters.is_active || ''}
                        onChange={(e) => setFilters({ is_active: e.target.value })}
                    >
                        <option value="">All States</option>
                        <option value="true">Active Only</option>
                        <option value="false">Inactive Only</option>
                    </select>
                </div>

                <div className="overflow-x-auto border border-slate-800 rounded-lg">
                    <table className="w-full text-left text-sm text-slate-300">
                        <thead className="bg-slate-800/50 text-xs uppercase font-semibold text-slate-400 border-b border-slate-800">
                            <tr>
                                <th className="px-6 py-4">Threat Pattern</th>
                                <th className="px-6 py-4">Type</th>
                                <th className="px-6 py-4">Reason</th>
                                <th className="px-6 py-4">Status</th>
                                <th className="px-6 py-4 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800/50">
                            {isLoading ? (
                                <tr>
                                    <td colSpan={5} className="px-6 py-12 text-center text-slate-500">
                                        <Loader2 className="w-8 h-8 animate-spin mx-auto text-red-500 mb-2" />
                                        Loading suppression rules...
                                    </td>
                                </tr>
                            ) : rules.length === 0 ? (
                                <tr>
                                    <td colSpan={5} className="px-6 py-12 text-center text-slate-500">No blocklist patterns established.</td>
                                </tr>
                            ) : (
                                rules.map((rule) => (
                                    <tr key={rule.id} className="hover:bg-slate-800/50 transition-colors">
                                        <td className="px-6 py-4 font-mono text-slate-200">
                                            {rule.pattern}
                                        </td>
                                        <td className="px-6 py-4 text-xs tracking-wider uppercase text-slate-500">
                                            {rule.pattern_type}
                                        </td>
                                        <td className="px-6 py-4">
                                            <div className="text-slate-300">{rule.reason || <span className="italic text-slate-600">No justification</span>}</div>
                                            <div className="text-slate-500 text-[10px] mt-1 uppercase">By: {rule.created_by}</div>
                                        </td>
                                        <td className="px-6 py-4">
                                            {rule.is_active ? (
                                                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[10px] uppercase font-bold border border-red-500/20 text-red-400 bg-red-500/10">
                                                    <ShieldAlert className="w-3 h-3" /> Active Block
                                                </span>
                                            ) : (
                                                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[10px] uppercase font-bold border border-slate-600/30 text-slate-400 bg-slate-800/30">
                                                    <ShieldCheck className="w-3 h-3" /> Off/Bypassed
                                                </span>
                                            )}
                                        </td>
                                        <td className="px-6 py-4 text-right">
                                            <button 
                                                onClick={() => toggleRule(rule.id, rule.is_active)}
                                                className={`text-xs font-semibold px-3 py-1.5 rounded transition-colors ${rule.is_active ? 'bg-slate-800 hover:bg-slate-700 text-slate-300' : 'bg-red-500/10 hover:bg-red-500/20 text-red-500'}`}
                                            >
                                                {rule.is_active ? 'Deactivate' : 'Reactivate'}
                                            </button>
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
                
                {rules.length > 0 && (
                    <div className="pt-4 flex items-center justify-between">
                        <div className="text-xs text-slate-400">
                            Page {filters.page} of {totalPages}
                        </div>
                        <div className="flex gap-2">
                            <button
                                disabled={filters.page === 1}
                                onClick={() => setFilters({ page: (filters.page || 1) - 1 })}
                                className="px-3 py-1.5 text-xs font-semibold bg-slate-800 text-slate-300 rounded hover:bg-slate-700 disabled:opacity-50"
                            >
                                Prev
                            </button>
                            <button
                                disabled={filters.page === totalPages}
                                onClick={() => setFilters({ page: (filters.page || 1) + 1 })}
                                className="px-3 py-1.5 text-xs font-semibold bg-slate-800 text-slate-300 rounded hover:bg-slate-700 disabled:opacity-50"
                            >
                                Next
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}
