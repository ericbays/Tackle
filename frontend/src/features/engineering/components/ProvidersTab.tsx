import React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { engineeringApi } from '../services/engineeringApi';
import { Cloud, Globe, Plus, Server, CheckCircle2, AlertCircle, Clock, Pencil, Trash2, AlertTriangle } from 'lucide-react';
import CloudCredentialModal from './CloudCredentialModal';
import DomainProviderModal from './DomainProviderModal';
import toast from 'react-hot-toast';

export default function ProvidersTab() {
  const [isCloudModalOpen, setIsCloudModalOpen] = React.useState(false);
  const [isDomainModalOpen, setIsDomainModalOpen] = React.useState(false);
  const [selectedCloudCred, setSelectedCloudCred] = React.useState<any>(null);
  const [selectedDomainProv, setSelectedDomainProv] = React.useState<any>(null);
  const [deleteConfirmId, setDeleteConfirmId] = React.useState<string | null>(null);

  const queryClient = useQueryClient();

  const deleteDomainProviderMutation = useMutation({
    mutationFn: engineeringApi.deleteDomainProvider,
    onSuccess: () => {
      toast.success('Provider deleted successfully');
      queryClient.invalidateQueries({ queryKey: ['domain-providers'] });
      setDeleteConfirmId(null);
    },
    onError: (err: any) => {
      toast.error(err?.response?.data?.error?.message || 'Failed to delete provider');
      setDeleteConfirmId(null);
    }
  });

  const { data: cloudCreds, isLoading: loadingCloud } = useQuery({
    queryKey: ['cloud-credentials'],
    queryFn: engineeringApi.getCloudCredentials,
  });

  const { data: domainProviders, isLoading: loadingDomain } = useQuery({
    queryKey: ['domain-providers'],
    queryFn: engineeringApi.getDomainProviders,
  });

  const StatusBadge = ({ status }: { status: string }) => {
    switch(status) {
      case 'active':
        return <span className="flex items-center gap-1 text-green-400 bg-green-500/10 px-2 py-0.5 rounded text-xs"><CheckCircle2 size={12}/> {status}</span>;
      case 'error':
        return <span className="flex items-center gap-1 text-red-400 bg-red-500/10 px-2 py-0.5 rounded text-xs"><AlertCircle size={12}/> {status}</span>;
      default:
        return <span className="flex items-center gap-1 text-yellow-400 bg-yellow-500/10 px-2 py-0.5 rounded text-xs"><Clock size={12}/> {status}</span>;
    }
  }

  return (
    <div className="space-y-8">
      {/* Cloud Credentials Section */}
      <section>
        <div className="flex justify-between items-center mb-4">
          <div className="flex items-center gap-2 text-lg font-semibold text-slate-200">
            <Cloud size={20} className="text-blue-400" /> Cloud Credentials
          </div>
          <button 
            onClick={() => {
              setSelectedCloudCred(null);
              setIsCloudModalOpen(true);
            }}
            className="flex items-center gap-2 bg-slate-800 hover:bg-slate-700 text-slate-200 px-3 py-1.5 rounded-md text-sm transition-colors border border-slate-700"
          >
            <Plus size={16} /> Add Cloud Provider
          </button>
        </div>
        
        <div className="bg-[#12182b] border border-slate-800 rounded-lg overflow-hidden">
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="bg-[#1a2235] text-slate-300 font-medium border-b border-slate-800">
              <tr>
                <th className="px-6 py-4">Provider</th>
                <th className="px-6 py-4">Display Name</th>
                <th className="px-6 py-4">Default Region</th>
                <th className="px-6 py-4">Status</th>
                <th className="px-6 py-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {loadingCloud ? (
                <tr><td colSpan={5} className="px-6 py-8 text-center text-slate-500">Loading...</td></tr>
              ) : cloudCreds?.length === 0 ? (
                <tr><td colSpan={5} className="px-6 py-12 text-center text-slate-500">No cloud credentials configured.</td></tr>
              ) : (
                cloudCreds?.map((cred) => (
                  <tr key={cred.id} className="hover:bg-slate-800/30 transition-colors">
                    <td className="px-6 py-4 text-slate-200 font-medium capitalize">{cred.provider_type}</td>
                    <td className="px-6 py-4 text-slate-300">{cred.display_name}</td>
                    <td className="px-6 py-4 text-slate-400">{cred.default_region}</td>
                    <td className="px-6 py-4"><StatusBadge status={cred.status} /></td>
                    <td className="px-6 py-4">
                      <div className="flex justify-center items-center gap-3">
                        <button 
                          onClick={() => {
                            setSelectedCloudCred(cred);
                            setIsCloudModalOpen(true);
                          }}
                          className="text-slate-500 hover:text-blue-400 transition-colors"
                          title="Edit"
                        >
                          <Pencil size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* Domain Providers Section */}
      <section>
        <div className="flex justify-between items-center mb-4">
          <div className="flex items-center gap-2 text-lg font-semibold text-slate-200">
            <Globe size={20} className="text-blue-400" /> Domain Registrars
          </div>
          <button 
            onClick={() => {
              setSelectedDomainProv(null);
              setIsDomainModalOpen(true);
            }}
            className="flex items-center gap-2 bg-slate-800 hover:bg-slate-700 text-slate-200 px-3 py-1.5 rounded-md text-sm transition-colors border border-slate-700"
          >
            <Plus size={16} /> Add Domain Registrar
          </button>
        </div>
        
        <div className="bg-[#12182b] border border-slate-800 rounded-lg overflow-hidden">
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="bg-[#1a2235] text-slate-300 font-medium border-b border-slate-800">
              <tr>
                <th className="px-6 py-4">Provider</th>
                <th className="px-6 py-4">Display Name</th>
                <th className="px-6 py-4">Status</th>
                <th className="px-6 py-4">Last Synced</th>
                <th className="px-6 py-4">Added</th>
                <th className="px-6 py-4 text-center">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {loadingDomain ? (
                <tr><td colSpan={4} className="px-6 py-8 text-center text-slate-500">Loading...</td></tr>
              ) : domainProviders?.length === 0 ? (
                <tr><td colSpan={6} className="px-6 py-12 text-center text-slate-500">No registrars configured.</td></tr>
              ) : (
                domainProviders?.map((prov) => (
                  <tr key={prov.id} className="hover:bg-slate-800/30 transition-colors">
                    <td className="px-6 py-4 text-slate-200 font-medium capitalize">{prov.provider_type}</td>
                    <td className="px-6 py-4 text-slate-300">{prov.display_name}</td>
                    <td className="px-6 py-4"><StatusBadge status={prov.status} /></td>
                    <td className="px-6 py-4 text-slate-400">
                      {prov.last_tested_at ? new Date(prov.last_tested_at).toLocaleString() : 'Never'}
                    </td>
                    <td className="px-6 py-4 text-slate-400">{new Date(prov.created_at).toLocaleDateString()}</td>
                    <td className="px-6 py-4">
                      <div className="flex justify-center items-center gap-3">
                        <button 
                          onClick={() => {
                            setSelectedDomainProv(prov);
                            setIsDomainModalOpen(true);
                          }}
                          className="text-slate-500 hover:text-blue-400 transition-colors"
                          title="Edit"
                        >
                          <Pencil size={16} />
                        </button>

                        {deleteConfirmId === prov.id ? (
                          <div className="flex items-center gap-2">
                            <span className="text-red-400 text-xs flex items-center gap-1"><AlertTriangle size={12}/> Sure?</span>
                            <button 
                              onClick={() => deleteDomainProviderMutation.mutate(prov.id)}
                              disabled={deleteDomainProviderMutation.isPending}
                              className="text-red-400 hover:text-red-300 bg-red-400/10 hover:bg-red-400/20 px-2 py-0.5 rounded text-xs transition-colors"
                            >
                              Yes
                            </button>
                            <button 
                              onClick={() => setDeleteConfirmId(null)}
                              className="text-slate-400 hover:text-slate-300 px-2 py-0.5 text-xs"
                            >
                              No
                            </button>
                          </div>
                        ) : (
                          <button 
                            onClick={() => setDeleteConfirmId(prov.id)}
                            className="text-slate-500 hover:text-red-400 transition-colors"
                            title="Delete Provider"
                          >
                            <Trash2 size={16} />
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      {isCloudModalOpen && <CloudCredentialModal onClose={() => setIsCloudModalOpen(false)} initialData={selectedCloudCred} />}
      {isDomainModalOpen && <DomainProviderModal onClose={() => setIsDomainModalOpen(false)} initialData={selectedDomainProv} />}
    </div>
  );
}
