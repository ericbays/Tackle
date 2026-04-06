import { Activity, ShieldAlert, Crosshair, ServerCrash, RefreshCw } from 'lucide-react';
import { StatCard } from '../components/dashboard/StatCard';
import { ActivityFeed, type ActivityLog } from '../components/dashboard/ActivityFeed';
import { CapturesLineChart, type TimeSeriesPoint } from '../components/dashboard/CapturesLineChart';
import { 
  useAuditLogs, 
  useTrends, 
  useOrganizationMetrics, 
  useCampaigns, 
  useEndpoints 
} from '../hooks/useDashboard';

export default function Dashboard() {
  const { data: rawLogs, isLoading: logsLoading } = useAuditLogs();
  const { data: rawTrends, isLoading: trendsLoading } = useTrends();
  const { data: orgMetrics, isLoading: orgLoading } = useOrganizationMetrics();
  const { data: campaigns, isLoading: campaignsLoading } = useCampaigns();
  const { data: endpointsData, isLoading: endpointsLoading } = useEndpoints();

  const isLoading = logsLoading || trendsLoading || orgLoading || campaignsLoading || endpointsLoading;

  // Compute StatCards
  // Active Campaigns
  let activeCampaigns = 0;
  let pendingApprovals = 0;
  if (Array.isArray(campaigns?.items)) {
    activeCampaigns = campaigns.items.filter((c: any) => c.current_state === 'active').length;
    pendingApprovals = campaigns.items.filter((c: any) => c.current_state === 'pending_approval').length;
  } else if (Array.isArray(campaigns)) {
    activeCampaigns = campaigns.filter((c: any) => c.current_state === 'active').length;
    pendingApprovals = campaigns.filter((c: any) => c.current_state === 'pending_approval').length;
  }

  // Endpoints
  let epsOnline = 0;
  let epsDegraded = 0;
  let endpointsIter = Array.isArray(endpointsData?.items) ? endpointsData.items : (Array.isArray(endpointsData) ? endpointsData : []);
  for (const ep of endpointsIter) {
    if (ep.status === 'online') epsOnline++;
    else if (ep.status === 'degraded') epsDegraded++;
  }

  // Transform Trends to TimeSeriesPoint
  let timeSeries: TimeSeriesPoint[] = [];
  if (Array.isArray(rawTrends)) {
    timeSeries = rawTrends.map((t: any) => ({
      date: t.completed_at || t.updated_at || new Date().toISOString(),
      value: (t.submit_rate || 0) * 10, // Just to give the chart some height
    }));
  }

  // Fallback to dummy data if DB is empty to keep chart from breaking empty,
  // but since we are fetching from DB, let's allow it to be empty.
  if (timeSeries.length === 0) {
    timeSeries = [
      { date: new Date().toISOString(), value: 0 }
    ];
  }

  // Transform Audit Logs to ActivityLog
  let mappedLogs: ActivityLog[] = [];
  const logArray = Array.isArray(rawLogs?.items) ? rawLogs.items : (Array.isArray(rawLogs) ? rawLogs : []);
  if (logArray.length > 0) {
    mappedLogs = logArray.slice(0, 15).map((l: any) => {
      let type: 'campaign' | 'infrastructure' | 'system' | 'security' = 'system';
      if (l.category === 'campaign' || l.category === 'email') type = 'campaign';
      else if (l.category === 'infrastructure') type = 'infrastructure';
      else if (l.category === 'security' || l.category === 'authentication') type = 'security';
      
      const ts = new Date(l.timestamp);
      const isToday = new Date().toDateString() === ts.toDateString();
      const timeStr = isToday ? ts.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : ts.toLocaleDateString();

      return {
        id: l.id || Math.random().toString(),
        type,
        timestamp: timeStr,
        actor: l.actor_label || 'system',
        description: l.action || 'Unknown event',
      };
    });
  }

  return (
    <div className="flex flex-col h-full gap-6 relative">
      {isLoading && (
        <div className="absolute inset-0 bg-slate-900/50 z-50 flex items-center justify-center backdrop-blur-sm rounded-xl">
          <RefreshCw className="w-8 h-8 text-blue-500 animate-spin" />
        </div>
      )}

      <div className="flex justify-between items-center mb-2">
        <h1 className="text-3xl font-bold bg-gradient-to-r from-slate-100 to-slate-400 bg-clip-text text-transparent">Dashboard</h1>
        <div className="flex items-center gap-3">
          <select className="bg-slate-900 border border-slate-700 text-sm rounded-md px-3 py-1.5 focus:outline-none focus:border-blue-500">
            <option>Last 7 days</option>
            <option>Last 30 days</option>
          </select>
          <button className="p-1.5 text-slate-400 hover:text-white bg-slate-800 rounded-md transition-colors" title="Refresh">
            <RefreshCw className="w-5 h-5" />
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard title="Active Campaigns" value={activeCampaigns.toString()} icon={Activity} />
        <StatCard title="Pending Approvals" value={pendingApprovals.toString()} icon={ShieldAlert} />
        <StatCard title="Org Susceptibility" value={orgMetrics?.susceptibility_score ? `${orgMetrics.susceptibility_score.toFixed(1)}%` : '0%'} icon={Crosshair} />
        <StatCard title="Endpoint Health" value={`${epsOnline} up / ${epsDegraded} deg`} icon={ServerCrash} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-1 min-h-[400px]">
        <div className="lg:col-span-2 glass-panel rounded-xl p-6 flex flex-col">
          <h3 className="text-lg font-bold text-slate-200 mb-4 border-b border-slate-800 pb-2">Captures Trend</h3>
          <div className="flex-1 -ml-4">
             <CapturesLineChart data={timeSeries} />
          </div>
        </div>

        <div className="max-h-[400px]">
          <ActivityFeed logs={mappedLogs.length > 0 ? mappedLogs : [
             { id: '1', type: 'system', timestamp: 'Now', actor: 'system', description: 'Real-time database feed initialized... waiting for events.' }
          ]} />
        </div>
      </div>
    </div>
  );
}
