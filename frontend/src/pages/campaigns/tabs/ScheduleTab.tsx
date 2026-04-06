import { Clock, CalendarDays, Timer, PlaneTakeoff } from 'lucide-react';
import { useParams } from 'react-router-dom';
import { useCampaignStore } from '../../../store/campaignStore';

export default function ScheduleTab() {
    const { id } = useParams();
    const { draft: { schedule }, updateSchedule, isSaving, saveCampaign } = useCampaignStore();

    return (
        <div className="space-y-8">
            <section className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden p-6">
                <div className="flex items-center gap-3 mb-6">
                    <PlaneTakeoff className="w-5 h-5 text-blue-400" />
                    <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">Launch Sequence</h2>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                    <div className="space-y-4">
                        <label className="block text-sm font-medium text-slate-400">Launch Date</label>
                        <div className="relative">
                            <CalendarDays className="absolute left-3 top-2.5 w-5 h-5 text-slate-500" />
                            <input 
                                type="date" 
                                value={schedule.startDate}
                                onChange={(e) => updateSchedule({ startDate: e.target.value })}
                                className="w-full bg-slate-950 border border-slate-800 rounded-md pl-10 pr-4 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
                            />
                        </div>
                    </div>
                    <div className="space-y-4">
                        <label className="block text-sm font-medium text-slate-400">Launch Time (UTC)</label>
                        <div className="relative">
                            <Clock className="absolute left-3 top-2.5 w-5 h-5 text-slate-500" />
                            <input 
                                type="time" 
                                value={schedule.startTime}
                                onChange={(e) => updateSchedule({ startTime: e.target.value })}
                                className="w-full bg-slate-950 border border-slate-800 rounded-md pl-10 pr-4 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
                            />
                        </div>
                    </div>
                </div>
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden p-6">
                <div className="flex items-center gap-3 mb-6">
                    <Timer className="w-5 h-5 text-emerald-400" />
                    <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">Delivery Velocity</h2>
                </div>

                <div className="space-y-6 max-w-2xl">
                    <p className="text-sm text-slate-400">
                        Configure how quickly the emails will be dispatched to your targets. Staggered sending helps bypass initial spam velocity checks.
                    </p>

                    <div>
                        <label className="block text-sm font-medium text-slate-400 mb-2">Pacing Mode</label>
                        <select 
                            value={schedule.pacingMode}
                            onChange={(e) => updateSchedule({ pacingMode: e.target.value })}
                            className="w-full bg-slate-950 border border-slate-800 rounded-md px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
                        >
                            <option>Staggered (Recommended)</option>
                            <option>Immediate Spurt</option>
                        </select>
                    </div>

                    <div className="grid grid-cols-2 gap-4 pt-2">
                        <div>
                            <label className="block text-xs font-medium text-slate-500 mb-2">Max batches per hour</label>
                            <input 
                                type="number" 
                                value={schedule.maxBatchesPerHour} 
                                onChange={(e) => updateSchedule({ maxBatchesPerHour: parseInt(e.target.value) || 0 })}
                                className="w-full bg-slate-950 border border-slate-800 rounded-md px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500" 
                            />
                        </div>
                        <div>
                            <label className="block text-xs font-medium text-slate-500 mb-2">Max emails per batch</label>
                            <input 
                                type="number" 
                                value={schedule.maxEmailsPerBatch} 
                                onChange={(e) => updateSchedule({ maxEmailsPerBatch: parseInt(e.target.value) || 0 })}
                                className="w-full bg-slate-950 border border-slate-800 rounded-md px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500" 
                            />
                        </div>
                    </div>

                    <div className="bg-slate-950/50 rounded-lg p-4 text-sm text-slate-500 border border-slate-800">
                        Based on your configuration, 142 targets will take approximately <strong>1.5 hours</strong> to fully deploy.
                    </div>
                </div>
            </section>

            <div className="flex justify-end pt-4">
                <button 
                    onClick={() => saveCampaign(id)}
                    disabled={isSaving}
                    className="bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white px-6 py-2.5 rounded-md font-medium transition-colors"
                >
                    {isSaving ? 'Saving...' : 'Save Schedule'}
                </button>
            </div>
        </div>
    );
}
