import { ArrowRight, CheckCircle2, Circle, Rocket } from 'lucide-react';
import { useNavigate, useParams } from 'react-router-dom';
import { useState } from 'react';
import { api } from '../../../services/api';
import toast from 'react-hot-toast';

export default function OverviewTab() {
    const navigate = useNavigate();
    const { id } = useParams();
    const [isLaunching, setIsLaunching] = useState(false);

    const readinessCards = [
        { title: 'Targets', desc: 'No targets assigned', ready: false, link: 'targets' },
        { title: 'Email Templates', desc: 'No templates configured', ready: false, link: 'email' },
        { title: 'Landing Page', desc: 'No landing page selected', ready: false, link: 'landing-page' },
        { title: 'Infrastructure', desc: 'Not configured', ready: false, link: 'infrastructure' },
        { title: 'Schedule', desc: 'No dates set', ready: false, link: 'schedule' },
        { title: 'Approval', desc: 'Not required yet', ready: true, hideArrow: true }
    ];

    const handleLaunch = async () => {
        if (!id) return;
        setIsLaunching(true);
        try {
            await api.put(`/campaigns/${id}/status`, { status: 'ready' });
            toast.success('Campaign successfully moved to ready state and locked. Workers are provisioning infrastructure.', {
                duration: 5000,
                icon: '🚀'
            });
        } catch (err: any) {
            toast.error(err.response?.data?.message || 'Failed to launch campaign. Check network log.');
        } finally {
            setIsLaunching(false);
        }
    };

    return (
        <div className="space-y-8">
            <section className="flex items-center justify-between bg-slate-900 border border-slate-800 rounded-xl p-6">
                <div>
                    <h2 className="text-xl font-bold text-white mb-1">Launch Campaign</h2>
                    <p className="text-sm text-slate-400">Lock configurations and begin backend infrastructure provisioning.</p>
                </div>
                <button
                    onClick={handleLaunch}
                    disabled={isLaunching}
                    className="flex items-center gap-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white px-6 py-3 rounded-lg font-medium transition-all shadow-[0_0_15px_rgba(16,185,129,0.3)] hover:shadow-[0_0_25px_rgba(16,185,129,0.5)]"
                >
                    <Rocket className={`w-5 h-5 ${isLaunching ? 'animate-bounce' : ''}`} />
                    {isLaunching ? 'Initiating Launch...' : 'Launch Campaign'}
                </button>
            </section>

            <section>
                <div className="flex items-center justify-between mb-4">
                    <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">Readiness Check</h2>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {readinessCards.map((card, i) => (
                        <div 
                            key={i} 
                            className={`p-5 rounded-xl border flex flex-col bg-slate-900 transition-colors ${card.ready ? 'border-slate-700' : 'border-dashed border-slate-700'}`}
                        >
                            <div className="flex items-start gap-3 mb-2">
                                {card.ready ? (
                                    <CheckCircle2 className="w-5 h-5 text-emerald-500 shrink-0 mt-0.5" />
                                ) : (
                                    <Circle className="w-5 h-5 text-slate-600 shrink-0 mt-0.5" />
                                )}
                                <div>
                                    <h3 className="font-medium text-slate-200">{card.title}</h3>
                                    <p className="text-sm text-slate-400 mt-1">{card.desc}</p>
                                </div>
                            </div>
                            
                            <div className="mt-auto pt-4 flex justify-end">
                                {!card.hideArrow && (
                                    <button 
                                        onClick={() => navigate(card.link || '')}
                                        className="text-sm font-medium text-blue-500 hover:text-blue-400 flex items-center gap-1 group"
                                    >
                                        {card.ready ? 'View' : 'Configure'} 
                                        <ArrowRight className="w-4 h-4 ml-1 transition-transform group-hover:translate-x-0.5" />
                                    </button>
                                )}
                            </div>
                        </div>
                    ))}
                </div>
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl p-6">
                <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500 mb-6">Campaign Summary</h2>
                
                <div className="grid grid-cols-[140px_1fr] gap-y-4 text-sm">
                    <div className="text-slate-500">Name</div>
                    <div className="font-medium text-slate-200">Q1 Executive Spear Phish</div>
                    
                    <div className="text-slate-500">Description</div>
                    <div className="text-slate-300 max-w-2xl">
                        Test executive team resilience to credential harvesting attacks using spoofed board communications.
                    </div>
                    
                    <div className="text-slate-500 mt-4">State</div>
                    <div className="mt-4"><span className="bg-slate-800 text-slate-300 px-2 py-1 rounded text-xs uppercase font-semibold">Draft</span></div>
                    
                    <div className="text-slate-500">Created</div>
                    <div className="text-slate-400">March 1, 2026 by <span className="text-slate-300">Administrator</span></div>
                </div>
            </section>
        </div>
    );
}
