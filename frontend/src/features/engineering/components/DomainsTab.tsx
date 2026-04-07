import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { engineeringApi } from '../services/engineeringApi';
import { Globe, Plus, RefreshCw, CheckCircle2, AlertCircle, Clock, ChevronLeft, ChevronRight, Search } from 'lucide-react';
import ManageDnsModal from './ManageDnsModal';

export default function DomainsTab() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedDomainForDns, setSelectedDomainForDns] = useState<{ id: string, name: string } | null>(null);
  const pageSize = 10;

  const { data: domains, isLoading } = useQuery({
    queryKey: ['domains'],
    queryFn: engineeringApi.getDomains,
  });

  const { data: domainProviders } = useQuery({
    queryKey: ['domain-providers'],
    queryFn: engineeringApi.getDomainProviders,
  });

  const providerMap = new Map();
  domainProviders?.forEach((p: any) => {
    providerMap.set(p.id, p);
  });

  const syncMutation = useMutation({
    mutationFn: engineeringApi.syncDomainProviders,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['domains'] });
      queryClient.invalidateQueries({ queryKey: ['domain-providers'] });
      alert('Providers synced successfully.');
    },
    onError: (err: any) => {
      alert('Error syncing providers: ' + err.message);
    }
  });

  const StatusBadge = ({ status }: { status: string }) => {
    switch(status?.toLowerCase()) {
      case 'active':
        return <span className="flex w-fit items-center gap-1 text-green-400 bg-green-500/10 px-2 py-0.5 rounded text-xs"><CheckCircle2 size={12}/> Active</span>;
      case 'pending':
        return <span className="flex w-fit items-center gap-1 text-yellow-400 bg-yellow-500/10 px-2 py-0.5 rounded text-xs"><Clock size={12}/> Pending</span>;
      case 'error':
        return <span className="flex w-fit items-center gap-1 text-red-400 bg-red-500/10 px-2 py-0.5 rounded text-xs"><AlertCircle size={12}/> Error</span>;
      default:
        return <span className="flex w-fit items-center gap-1 text-slate-400 bg-slate-500/10 px-2 py-0.5 rounded text-xs"><Globe size={12}/> {status}</span>;
    }
  };

  const getProviderName = (id: string) => {
    if (!id) return;
    const provider = providerMap.get(id);
    if (!provider) return id;
    return (
      <div className="flex flex-col">
        <span className="text-slate-200">{provider.display_name}</span>
        <span className="text-slate-500 text-xs capitalize">{provider.provider_type}</span>
      </div>
    );
  };

  const filteredDomains = domains?.filter((d: any) => {
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    
    if (d.domain_name?.toLowerCase().includes(q)) return true;
    if (d.status?.toLowerCase().includes(q)) return true;
    if (d.tags?.some((t: string) => t.toLowerCase().includes(q))) return true;
    
    const provId = d.registrar_connection_id || d.dns_provider_connection_id;
    if (provId) {
      const p = providerMap.get(provId);
      if (p?.display_name?.toLowerCase().includes(q)) return true;
      if (p?.provider_type?.toLowerCase().includes(q)) return true;
    }
    
    return false;
  }) || [];

  const totalPages = Math.ceil(filteredDomains.length / pageSize);
  const paginatedDomains = filteredDomains.slice((page - 1) * pageSize, page * pageSize);

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-6">
          <div className="flex items-center gap-2 text-lg font-semibold text-slate-200">
            <Globe size={20} className="text-blue-400" /> Active Domains
          </div>
          <div className="relative relative flex items-center">
            <Search className="absolute left-3 text-slate-400" size={16} />
            <input 
              type="text" 
              placeholder="Search domains..." 
              value={searchQuery}
              onChange={(e) => { setSearchQuery(e.target.value); setPage(1); }}
              className="bg-[#1a2235] border border-slate-700 text-slate-200 text-sm rounded-md pl-10 pr-4 py-1.5 focus:outline-none focus:border-blue-500 w-64 md:w-80 transition-colors placeholder:text-slate-500 shadow-inner"
            />
          </div>
        </div>
        <div className="flex gap-3">
          <button 
            onClick={() => syncMutation.mutate()}
            disabled={syncMutation.isPending}
            className="flex items-center gap-2 bg-slate-800 hover:bg-slate-700 text-slate-200 px-3 py-1.5 rounded-md text-sm transition-colors border border-slate-700 disabled:opacity-50"
          >
            <RefreshCw size={16} className={syncMutation.isPending ? "animate-spin" : ""} /> 
            {syncMutation.isPending ? 'Syncing...' : 'Sync Providers'}
          </button>
          <button className="flex items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white px-3 py-1.5 rounded-md text-sm transition-colors shadow-lg shadow-blue-500/20">
            <Plus size={16} /> Register Domain
          </button>
        </div>
      </div>
      
      <div className="bg-[#12182b] border border-slate-800 rounded-lg overflow-hidden flex flex-col">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="bg-[#1a2235] text-slate-300 font-medium border-b border-slate-800">
              <tr>
                <th className="px-6 py-4">Domain Name</th>
                <th className="px-6 py-4">Provider</th>
                <th className="px-6 py-4">Status</th>
                <th className="px-6 py-4">Added</th>
                <th className="px-6 py-4 text-center">Campaigns</th>
                <th className="px-6 py-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {isLoading ? (
                <tr><td colSpan={6} className="px-6 py-8 text-center text-slate-500">Loading...</td></tr>
              ) : filteredDomains.length === 0 ? (
                <tr><td colSpan={6} className="px-6 py-12 text-center text-slate-500">
                  {searchQuery ? 'No domains match your search.' : 'No domains configured.'}
                </td></tr>
              ) : (
                paginatedDomains?.map((domain: any) => (
                  <tr key={domain.id} className="hover:bg-slate-800/30 transition-colors">
                    <td className="px-6 py-4">
                      <div className="flex flex-col">
                        <span className="text-slate-200 font-medium">{domain.domain_name}</span>
                        <div className="flex gap-1 mt-1">
                          {domain.tags?.map((t: string) => (
                            <span key={t} className="px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 text-[10px]">{t}</span>
                          ))}
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4">{domain.registrar_connection_id ? getProviderName(domain.registrar_connection_id) : (domain.dns_provider_connection_id ? getProviderName(domain.dns_provider_connection_id) : <span className="text-slate-500">Internal</span>)}</td>
                    <td className="px-6 py-4"><StatusBadge status={domain.status} /></td>
                    <td className="px-6 py-4 text-slate-400">{domain.created_at ? new Date(domain.created_at).toLocaleDateString() : 'N/A'}</td>
                    <td className="px-6 py-4 text-center">
                      <span className="bg-slate-800/80 text-slate-300 px-2 py-0.5 rounded-full text-xs font-medium">
                        {domain.campaign_count || 0}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-right">
                      <button 
                        onClick={() => setSelectedDomainForDns({ id: domain.id, name: domain.domain_name })} 
                        className="text-blue-400 hover:text-blue-300 text-xs font-medium transition-colors"
                      >
                        Manage DNS
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        
        {/* Pagination Controls */}
        {!isLoading && filteredDomains.length > 0 && (
          <div className="border-t border-slate-800 px-6 py-4 flex items-center justify-between text-sm">
            <div className="text-slate-400">
              Showing <span className="font-medium text-slate-200">{(page - 1) * pageSize + 1}</span> to <span className="font-medium text-slate-200">{Math.min(page * pageSize, filteredDomains.length)}</span> of <span className="font-medium text-slate-200">{filteredDomains.length}</span> domains
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setPage(p => Math.max(1, p - 1))}
                disabled={page === 1}
                className="p-1.5 rounded bg-slate-800/50 hover:bg-slate-700 text-slate-300 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronLeft size={16} />
              </button>
              <span className="text-slate-400 px-2">Page {page} of {totalPages || 1}</span>
              <button
                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
                className="p-1.5 rounded bg-slate-800/50 hover:bg-slate-700 text-slate-300 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronRight size={16} />
              </button>
            </div>
          </div>
        )}
      </div>

      {selectedDomainForDns && (
        <ManageDnsModal 
          onClose={() => setSelectedDomainForDns(null)} 
          domainId={selectedDomainForDns.id} 
          domainName={selectedDomainForDns.name} 
        />
      )}
    </div>
  );
}
