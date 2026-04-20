import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { engineeringApi } from '../services/engineeringApi';
import { Globe, Plus, RefreshCw, CheckCircle2, AlertCircle, Clock, ChevronLeft, ChevronRight, Search } from 'lucide-react';
import ManageDnsModal from './ManageDnsModal';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Badge } from '../../../components/ui/Badge';
import { Card } from '../../../components/ui/Card';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '../../../components/ui/Table';

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
        return <Badge variant="success" className="gap-1.5"><CheckCircle2 size={12}/> Active</Badge>;
      case 'pending':
        return <Badge variant="warning" className="gap-1.5"><Clock size={12}/> Pending</Badge>;
      case 'error':
        return <Badge variant="destructive" className="gap-1.5"><AlertCircle size={12}/> Error</Badge>;
      default:
        return <Badge variant="secondary" className="gap-1.5"><Globe size={12}/> {status}</Badge>;
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
          <div className="relative flex items-center">
            <Search className="absolute left-3 text-slate-400 z-10" size={16} />
            <Input 
              type="text" 
              placeholder="Search domains..." 
              value={searchQuery}
              onChange={(e) => { setSearchQuery(e.target.value); setPage(1); }}
              className="pl-10 w-64 md:w-80 shadow-inner"
            />
          </div>
        </div>
        <div className="flex gap-3">
          <Button 
            onClick={() => syncMutation.mutate()}
            disabled={syncMutation.isPending}
            variant="secondary"
            className="flex items-center gap-2"
          >
            <RefreshCw size={16} className={syncMutation.isPending ? "animate-spin" : ""} /> 
            {syncMutation.isPending ? 'Syncing...' : 'Sync Providers'}
          </Button>
          <Button variant="primary" className="flex items-center gap-2 shadow-lg shadow-blue-500/20">
            <Plus size={16} /> Register Domain
          </Button>
        </div>
      </div>
      
      <Card className="rounded-lg overflow-hidden flex flex-col p-0 border-slate-800 shadow-none bg-[#12182b]">
        <div className="overflow-x-auto">
          <Table className="w-full text-left text-sm whitespace-nowrap border-0 shadow-none">
            <TableHeader className="bg-[#1a2235]">
              <TableRow className="text-slate-300 font-medium border-b border-slate-800 hover:bg-transparent">
                <TableHead>Domain Name</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Added</TableHead>
                <TableHead className="text-center">Campaigns</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="divide-y divide-slate-800/50">
              {isLoading ? (
                <TableRow><TableCell colSpan={6} className="py-8 text-center text-slate-500">Loading...</TableCell></TableRow>
              ) : filteredDomains.length === 0 ? (
                <TableRow><TableCell colSpan={6} className="py-12 text-center text-slate-500">
                  {searchQuery ? 'No domains match your search.' : 'No domains configured.'}
                </TableCell></TableRow>
              ) : (
                paginatedDomains?.map((domain: any) => (
                  <TableRow key={domain.id} className="hover:bg-slate-800/30 transition-colors">
                    <TableCell>
                      <div className="flex flex-col">
                        <span className="text-slate-200 font-medium">{domain.domain_name}</span>
                        <div className="flex gap-1 mt-1">
                          {domain.tags?.map((t: string) => (
                            <span key={t} className="px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 text-[10px]">{t}</span>
                          ))}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>{domain.registrar_connection_id ? getProviderName(domain.registrar_connection_id) : (domain.dns_provider_connection_id ? getProviderName(domain.dns_provider_connection_id) : <span className="text-slate-500">Internal</span>)}</TableCell>
                    <TableCell><StatusBadge status={domain.status} /></TableCell>
                    <TableCell className="text-slate-400">{domain.created_at ? new Date(domain.created_at).toLocaleDateString() : 'N/A'}</TableCell>
                    <TableCell className="text-center">
                      <span className="bg-slate-800/80 text-slate-300 px-2 py-0.5 rounded-full text-xs font-medium">
                        {domain.campaign_count || 0}
                      </span>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button 
                        variant="ghost"
                        onClick={() => setSelectedDomainForDns({ id: domain.id, name: domain.domain_name })} 
                        className="text-blue-400 hover:text-blue-300 text-xs font-medium transition-colors p-0 h-auto"
                      >
                        Manage DNS
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
        
        {/* Pagination Controls */}
        {!isLoading && filteredDomains.length > 0 && (
          <div className="border-t border-slate-800 px-6 py-4 flex items-center justify-between text-sm">
            <div className="text-slate-400">
              Showing <span className="font-medium text-slate-200">{(page - 1) * pageSize + 1}</span> to <span className="font-medium text-slate-200">{Math.min(page * pageSize, filteredDomains.length)}</span> of <span className="font-medium text-slate-200">{filteredDomains.length}</span> domains
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="secondary"
                size="icon"
                onClick={() => setPage(p => Math.max(1, p - 1))}
                disabled={page === 1}
              >
                <ChevronLeft size={16} />
              </Button>
              <span className="text-slate-400 px-2">Page {page} of {totalPages || 1}</span>
              <Button
                variant="secondary"
                size="icon"
                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
              >
                <ChevronRight size={16} />
              </Button>
            </div>
          </div>
        )}
      </Card>

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
