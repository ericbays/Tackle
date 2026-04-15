import { useEffect } from 'react';
import { Link, Routes, Route, useParams, useLocation } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';
import OverviewTab from './tabs/OverviewTab';
import TargetsTab from './tabs/TargetsTab';
import EmailTemplateTab from './tabs/EmailTemplateTab';
import InfrastructureTab from './tabs/InfrastructureTab';
import ScheduleTab from './tabs/ScheduleTab';
import LandingPageTab from './tabs/LandingPageTab';
import { useCampaignStore } from '../../store/campaignStore';

export default function WorkspaceLayout() {
    const { id } = useParams();
    const location = useLocation();
    const { fetchCampaign, isLoading, campaign } = useCampaignStore();

    useEffect(() => {
        if (id) {
            fetchCampaign(id);
        }
    }, [id, fetchCampaign]);

    // The active tab is determined by the URL
    const getTabClass = (path: string) => {
        const isActive = location.pathname.endsWith(path) || (path === '' && location.pathname.endsWith(id || ''));
        return `px-4 py-3 font-medium text-sm transition-all border-b-2 ${
            isActive ? 'border-blue-500 text-blue-400' : 'border-transparent text-slate-400 hover:text-slate-200'
        }`;
    };

    const basePath = `/campaigns/${id}`;

    if (isLoading || !campaign) {
        return <div className="text-white text-center py-12">Loading campaign workspace...</div>;
    }

    const { name, status, owner, createdAt, updatedAt } = campaign;
    const formattedCreated = new Date(createdAt).toLocaleDateString();

    const getStatusColor = (s: string) => {
        switch (s.toLowerCase()) {
            case 'active': return 'bg-blue-500/20 text-blue-400 border-blue-500/30';
            case 'building': return 'bg-amber-500/20 text-amber-400 border-amber-500/30';
            case 'completed': return 'bg-green-500/20 text-green-400 border-green-500/30';
            default: return 'bg-slate-700 text-slate-200 border-slate-600';
        }
    };

    return (
        <div className="max-w-7xl mx-auto space-y-6">
            {/* Header Area */}
            <div className="space-y-4">
                <div className="flex items-center text-sm font-medium text-slate-400">
                    <Link to="/campaigns" className="hover:text-white transition-colors">Campaigns</Link>
                    <ChevronRight className="w-4 h-4 mx-2" />
                    <span className="text-slate-200">{name}</span>
                </div>

                <div className="flex items-start justify-between">
                    <div>
                        <div className="flex items-center gap-3">
                            <h1 className="text-3xl font-bold text-white tracking-tight">{name}</h1>
                            <span className={`text-xs uppercase tracking-widest font-semibold px-2.5 py-1 rounded-md border ${getStatusColor(status)}`}>
                                {status}
                            </span>
                        </div>
                        <p className="text-slate-400 mt-2 max-w-2xl">
                            Configure phishing targets, email templates, and landing applications to test executive team resilience to credential harvesting.
                        </p>
                        <p className="text-xs text-slate-500 mt-3">Created {formattedCreated} by {owner}</p>
                    </div>
                </div>
            </div>

            {/* Tab Navigation */}
            <div className="border-b border-slate-800">
                <nav className="flex gap-2">
                    <Link to={basePath} className={getTabClass('')}>Overview</Link>
                    <Link to={`${basePath}/targets`} className={getTabClass('targets')}>Targets</Link>
                    <Link to={`${basePath}/email`} className={getTabClass('email')}>Email Templates</Link>
                    <Link to={`${basePath}/landing-page`} className={getTabClass('landing-page')}>Landing Page</Link>
                    <Link to={`${basePath}/infrastructure`} className={getTabClass('infrastructure')}>Infrastructure</Link>
                    <Link to={`${basePath}/schedule`} className={getTabClass('schedule')}>Schedule</Link>
                </nav>
            </div>

            {/* Sub-routing context */}
            <div className="pt-4">
                <Routes>
                    <Route path="" element={<OverviewTab />} />
                    <Route path="targets" element={<TargetsTab />} />
                    <Route path="email" element={<EmailTemplateTab />} />
                    <Route path="landing-page" element={<LandingPageTab />} />
                    <Route path="infrastructure" element={<InfrastructureTab />} />
                    <Route path="schedule" element={<ScheduleTab />} />
                </Routes>
            </div>
        </div>
    );
}
