import { Plus, X, Users, AlertTriangle, ShieldCheck } from 'lucide-react';
import { useParams } from 'react-router-dom';
import { useCampaignStore } from '../../../store/campaignStore';



export default function TargetsTab() {
    const { id } = useParams();
    const { draft: { targetGroups, canaryTargets }, isSaving, saveCampaign } = useCampaignStore();

    const totalTargets = targetGroups.reduce((acc, curr) => acc + curr.count, 0);

    return (
        <div className="space-y-8">
            <section className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
                <div className="p-6">
                    <div className="flex items-center justify-between mb-6">
                        <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">Target Groups</h2>
                        <button className="flex items-center gap-2 text-sm bg-slate-800 hover:bg-slate-700 text-white px-3 py-1.5 rounded-md transition-colors">
                            <Plus className="w-4 h-4" /> Add Group
                        </button>
                    </div>

                    <div className="space-y-3">
                        {targetGroups.map(group => (
                            <div key={group.id} className="flex items-center justify-between bg-slate-950 border border-slate-800 p-4 rounded-lg">
                                <div className="flex items-center gap-4 text-slate-300">
                                    <button className="text-slate-600 hover:text-red-400 transition-colors">
                                        <X className="w-4 h-4" />
                                    </button>
                                    <span className="font-medium text-white">{group.name}</span>
                                </div>
                                <div className="flex items-center gap-6 text-sm text-slate-400">
                                    <span className="flex items-center gap-2"><Users className="w-4 h-4" /> {group.count} targets</span>
                                    <span>Added {group.added}</span>
                                </div>
                            </div>
                        ))}
                    </div>

                    <div className="mt-6 pt-6 border-t border-slate-800 text-sm text-slate-400">
                        Total: <span className="font-semibold text-white">{totalTargets}</span> unique targets <span className="text-slate-500">(0 duplicates removed)</span>
                    </div>
                </div>
            </section>

            <section className="bg-emerald-950/20 border-l-4 border-l-emerald-500 border-y border-y-slate-800 border-r border-r-slate-800 rounded-lg p-6">
                <div className="flex items-start gap-4">
                    <ShieldCheck className="w-6 h-6 text-emerald-500 shrink-0 mt-1" />
                    <div>
                        <h3 className="font-semibold text-emerald-500 text-sm uppercase tracking-widest mb-3">Blocklist Check</h3>
                        <p className="text-slate-300 text-sm mb-4">No targets match any active blocklist entries.</p>
                        <p className="text-xs text-slate-500 leading-relaxed">
                            All {totalTargets} targets have been cross-checked against your global blocklist policies.<br/>
                            This campaign is cleared for scheduling.
                        </p>
                    </div>
                </div>
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
                <div className="p-6">
                    <div className="flex items-center justify-between mb-2">
                        <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">Canary Targets</h2>
                        <button 
                            onClick={() => {
                                const email = prompt('Enter canary email address:');
                                if (email && email.includes('@')) {
                                    useCampaignStore.getState().updateCanaryTargets([...canaryTargets, email]);
                                }
                            }}
                            className="flex items-center gap-2 text-sm text-blue-500 hover:text-blue-400 px-3 py-1.5 transition-colors font-medium"
                        >
                            <Plus className="w-4 h-4" /> Add Canary
                        </button>
                    </div>
                    <p className="text-sm text-slate-500 mb-6">
                        Canary targets receive the phishing email but are controlled accounts used to verify delivery and rendering.
                    </p>

                    <div className="space-y-2">
                        {canaryTargets.length === 0 ? (
                            <div className="text-sm text-slate-500 italic py-2">No canary targets configured.</div>
                        ) : canaryTargets.map((email, i) => (
                            <div key={i} className="flex items-center justify-between text-sm py-2 group bg-slate-950/50 px-3 rounded-md border border-slate-800/50">
                                <div className="flex items-center gap-3">
                                    <div className="w-1.5 h-1.5 rounded-full bg-slate-700" />
                                    <span className="text-slate-300">{email}</span>
                                </div>
                                <button 
                                    onClick={() => {
                                        useCampaignStore.getState().updateCanaryTargets(canaryTargets.filter(t => t !== email));
                                    }}
                                    className="text-slate-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-all p-1"
                                >
                                    <X className="w-4 h-4" />
                                </button>
                            </div>
                        ))}
                    </div>
                </div>
            </section>

            <div className="flex justify-end pt-4">
                <button 
                    onClick={() => saveCampaign(id)}
                    disabled={isSaving}
                    className="bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white px-6 py-2.5 rounded-md font-medium transition-colors"
                >
                    {isSaving ? 'Saving...' : 'Save Targets'}
                </button>
            </div>
        </div>
    );
}
