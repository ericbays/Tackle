import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { engineeringApi } from '../services/engineeringApi';
import { Server, Activity, Power, RefreshCw, Trash2 } from 'lucide-react';

export default function InfrastructureTab() {
  const { data: endpoints, isLoading } = useQuery({
    queryKey: ['endpoints'],
    queryFn: engineeringApi.getEndpoints,
  });

  const getStatusColor = (status: string, health: string) => {
    if (status === 'error') return 'text-red-400 bg-red-500/10';
    if (status === 'provisioning') return 'text-blue-400 bg-blue-500/10';
    if (status === 'stopped') return 'text-slate-400 bg-slate-500/10';
    if (health === 'unhealthy') return 'text-yellow-400 bg-yellow-500/10';
    return 'text-green-400 bg-green-500/10'; // running & healthy
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-2 text-lg font-semibold text-slate-200">
          <Server size={20} className="text-blue-400" /> Active Infrastructure (Endpoints)
        </div>
      </div>
      
      <div className="bg-[#12182b] border border-slate-800 rounded-lg overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="bg-[#1a2235] text-slate-300 font-medium border-b border-slate-800">
              <tr>
                <th className="px-6 py-4">Instance ID</th>
                <th className="px-6 py-4">Campaign</th>
                <th className="px-6 py-4">Provider</th>
                <th className="px-6 py-4">Public IP</th>
                <th className="px-6 py-4">Status</th>
                <th className="px-6 py-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {isLoading ? (
                <tr><td colSpan={6} className="px-6 py-8 text-center text-slate-500">Loading...</td></tr>
              ) : endpoints?.length === 0 ? (
                <tr><td colSpan={6} className="px-6 py-12 text-center text-slate-500">No active endpoints found for any campaigns.</td></tr>
              ) : (
                endpoints?.map((ep) => (
                  <tr key={ep.id} className="hover:bg-slate-800/30 transition-colors">
                    <td className="px-6 py-4 text-slate-200 font-mono text-xs">{ep.instance_id || 'Pending'}</td>
                    <td className="px-6 py-4 text-slate-300">{ep.campaign_id}</td>
                    <td className="px-6 py-4 text-slate-400 capitalize">{ep.cloud_provider}</td>
                    <td className="px-6 py-4 text-slate-300 font-mono text-xs">{ep.public_ip || '---.---.---.---'}</td>
                    <td className="px-6 py-4">
                      <span className={`px-2 py-1 rounded text-xs inline-flex items-center gap-1.5 ${getStatusColor(ep.status, ep.health_status)}`}>
                        {ep.status === 'running' && ep.health_status === 'healthy' ? <Activity size={12}/> : <span className="w-1.5 h-1.5 rounded-full bg-current"></span>}
                        <span className="capitalize">{ep.status === 'running' ? ep.health_status : ep.status}</span>
                      </span>
                    </td>
                    <td className="px-6 py-4 text-right">
                      <div className="flex justify-end gap-2 text-slate-400">
                        <button className="hover:text-amber-400 p-1.5 rounded bg-slate-800/50 hover:bg-slate-800 transition-colors" title="Restart">
                          <RefreshCw size={14} />
                        </button>
                        <button className="hover:text-slate-200 p-1.5 rounded bg-slate-800/50 hover:bg-slate-800 transition-colors" title="Stop">
                          <Power size={14} />
                        </button>
                        <button className="hover:text-red-400 p-1.5 rounded bg-slate-800/50 hover:bg-slate-800 transition-colors" title="Terminate">
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
