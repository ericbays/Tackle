import { Plus, X, Users, AlertTriangle } from 'lucide-react';
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

            <section className="bg-amber-950/20 border-l-4 border-l-amber-500 border-y border-y-slate-800 border-r border-r-slate-800 rounded-lg p-6">
                <div className="flex items-start gap-4">
                    <AlertTriangle className="w-6 h-6 text-amber-500 shrink-0 mt-1" />
                    <div>
                        <h3 className="font-semibold text-amber-500 text-sm uppercase tracking-widest mb-3">Blocklist Check</h3>
                        <p className="text-slate-300 text-sm mb-4">2 targets match blocklist entries:</p>
                        
                        <ul className="space-y-2 text-sm text-slate-400 mb-6">
                            <li className="flex items-center gap-2">
                                <span className="w-1.5 h-1.5 rounded-full bg-amber-500/50" />
                                <span className="text-slate-300 font-medium">ceo@example.com</span> — "CEO exclusion per policy" (added Jan 15)
                            </li>
                            <li className="flex items-center gap-2">
                                <span className="w-1.5 h-1.5 rounded-full bg-amber-500/50" />
                                <span className="text-slate-300 font-medium">bp.legal@example.com</span> — "Legal exclusion compliance" (added Feb 10)
                            </li>
                        </ul>
                        
                        <p className="text-xs text-slate-500 leading-relaxed">
                            Blocklist matches do not prevent the campaign from proceeding.<br/>
                            An additional Administrator approval will be required before launch.
                        </p>
                    </div>
                </div>
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
                <div className="p-6">
                    <div className="flex items-center justify-between mb-2">
                        <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">Canary Targets</h2>
                        <button className="flex items-center gap-2 text-sm text-blue-500 hover:text-blue-400 px-3 py-1.5 transition-colors font-medium">
                            <Plus className="w-4 h-4" /> Add Canary
                        </button>
                    </div>
                    <p className="text-sm text-slate-500 mb-6">
                        Canary targets receive the phishing email but are controlled accounts used to verify delivery and rendering.
                    </p>

                    <div className="space-y-2">
                        {canaryTargets.map((email, i) => (
                            <div key={i} className="flex items-center justify-between text-sm py-2 group">
                                <div className="flex items-center gap-3">
                                    <div className="w-1.5 h-1.5 rounded-full bg-slate-700" />
                                    <span className="text-slate-300">{email}</span>
                                </div>
                                <button className="text-slate-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-all">
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
