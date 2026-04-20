import React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { engineeringApi } from '../services/engineeringApi';
import { Cloud, Globe, Plus, Server, CheckCircle2, AlertCircle, Clock, Pencil, Trash2, AlertTriangle, Play } from 'lucide-react';
import CloudCredentialModal from './CloudCredentialModal';
import DomainProviderModal from './DomainProviderModal';
import toast from 'react-hot-toast';
import { Button } from '../../../components/ui/Button';
import { Badge } from '../../../components/ui/Badge';
import { Card } from '../../../components/ui/Card';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '../../../components/ui/Table';

export default function ProvidersTab() {
  const [isCloudModalOpen, setIsCloudModalOpen] = React.useState(false);
  const [isDomainModalOpen, setIsDomainModalOpen] = React.useState(false);
  const [selectedCloudCred, setSelectedCloudCred] = React.useState<any>(null);
  const [selectedDomainProv, setSelectedDomainProv] = React.useState<any>(null);
  const [deleteConfirmId, setDeleteConfirmId] = React.useState<string | null>(null);
  const [deleteCloudConfirmId, setDeleteCloudConfirmId] = React.useState<string | null>(null);

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

  const deleteCloudCredentialMutation = useMutation({
    mutationFn: engineeringApi.deleteCloudCredential,
    onSuccess: () => {
      toast.success('Cloud credential deleted successfully');
      queryClient.invalidateQueries({ queryKey: ['cloud-credentials'] });
      setDeleteCloudConfirmId(null);
    },
    onError: (err: any) => {
      toast.error(err?.response?.data?.error?.message || 'Failed to delete cloud credential');
      setDeleteCloudConfirmId(null);
    }
  });

  const testCloudCredentialMutation = useMutation({
    mutationFn: engineeringApi.testCloudCredential,
    onSuccess: (data: any) => {
      if (data && data.success === false) {
        toast.error(`Test Failed: ${data.message || 'Invalid credentials'}`);
      } else {
        toast.success('Connection test fully valid! Connection established.');
      }
      queryClient.invalidateQueries({ queryKey: ['cloud-credentials'] });
    },
    onError: (err: any) => {
      toast.error(err?.response?.data?.error?.message || 'Failed to trigger test');
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
        return <Badge variant="success" className="gap-1.5"><CheckCircle2 size={12}/> {status}</Badge>;
      case 'error':
        return <Badge variant="destructive" className="gap-1.5"><AlertCircle size={12}/> {status}</Badge>;
      default:
        return <Badge variant="warning" className="gap-1.5"><Clock size={12}/> {status}</Badge>;
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
          <Button 
            onClick={() => {
              setSelectedCloudCred(null);
              setIsCloudModalOpen(true);
            }}
            variant="secondary"
            className="flex items-center gap-2"
          >
            <Plus size={16} /> Add Cloud Provider
          </Button>
        </div>
        
        <Card className="rounded-lg overflow-hidden p-0 border-slate-800 shadow-none bg-[#12182b]">
          <Table className="w-full text-left text-sm whitespace-nowrap border-0 shadow-none">
            <TableHeader className="bg-[#1a2235]">
              <TableRow className="text-slate-300 font-medium border-b border-slate-800 hover:bg-transparent">
                <TableHead>Provider</TableHead>
                <TableHead>Display Name</TableHead>
                <TableHead>Default Region</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="divide-y divide-slate-800/50">
              {loadingCloud ? (
                <TableRow><TableCell colSpan={5} className="py-8 text-center text-slate-500">Loading...</TableCell></TableRow>
              ) : cloudCreds?.length === 0 ? (
                <TableRow><TableCell colSpan={5} className="py-12 text-center text-slate-500">No cloud credentials configured.</TableCell></TableRow>
              ) : (
                cloudCreds?.map((cred) => (
                  <TableRow key={cred.id} className="hover:bg-slate-800/30 transition-colors">
                    <TableCell className="font-medium capitalize text-slate-200">{cred.provider_type}</TableCell>
                    <TableCell className="text-slate-300">{cred.display_name}</TableCell>
                    <TableCell className="text-slate-400">{cred.default_region}</TableCell>
                    <TableCell><StatusBadge status={cred.status} /></TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end items-center gap-3">
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => testCloudCredentialMutation.mutate(cred.id)}
                          disabled={testCloudCredentialMutation.isPending}
                          title="Test Connection"
                          className="text-emerald-400 hover:text-emerald-300 hover:bg-emerald-400/10"
                        >
                          <Play size={16} />
                        </Button>
                        <Button 
                          variant="ghost"
                          size="icon"
                          onClick={() => {
                            setSelectedCloudCred(cred);
                            setIsCloudModalOpen(true);
                          }}
                          title="Edit"
                        >
                          <Pencil size={16} />
                        </Button>

                        {deleteCloudConfirmId === cred.id ? (
                          <div className="flex items-center gap-2">
                            <span className="text-red-400 text-xs flex items-center gap-1"><AlertTriangle size={12}/> Sure?</span>
                            <Button
                              variant="destructive"
                              size="sm"
                              onClick={() => deleteCloudCredentialMutation.mutate(cred.id)}
                              disabled={deleteCloudCredentialMutation.isPending}
                            >
                              Yes
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => setDeleteCloudConfirmId(null)}
                            >
                              No
                            </Button>
                          </div>
                        ) : (
                          <Button 
                            variant="ghost"
                            size="icon"
                            onClick={() => setDeleteCloudConfirmId(cred.id)}
                            title="Delete Provider"
                            className="text-red-400 hover:text-red-300 hover:bg-red-400/10"
                          >
                            <Trash2 size={16} />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </Card>
      </section>

      {/* Domain Providers Section */}
      <section>
        <div className="flex justify-between items-center mb-4">
          <div className="flex items-center gap-2 text-lg font-semibold text-slate-200">
            <Globe size={20} className="text-blue-400" /> Domain Registrars
          </div>
          <Button 
            onClick={() => {
              setSelectedDomainProv(null);
              setIsDomainModalOpen(true);
            }}
            variant="secondary"
            className="flex items-center gap-2"
          >
            <Plus size={16} /> Add Domain Registrar
          </Button>
        </div>
        
        <Card className="rounded-lg overflow-hidden p-0 border-slate-800 shadow-none bg-[#12182b]">
          <Table className="w-full text-left text-sm whitespace-nowrap border-0 shadow-none">
            <TableHeader className="bg-[#1a2235]">
              <TableRow className="text-slate-300 font-medium border-b border-slate-800 hover:bg-transparent">
                <TableHead>Provider</TableHead>
                <TableHead>Display Name</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Last Synced</TableHead>
                <TableHead>Added</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="divide-y divide-slate-800/50">
              {loadingDomain ? (
                <TableRow><TableCell colSpan={6} className="py-8 text-center text-slate-500">Loading...</TableCell></TableRow>
              ) : domainProviders?.length === 0 ? (
                <TableRow><TableCell colSpan={6} className="py-12 text-center text-slate-500">No registrars configured.</TableCell></TableRow>
              ) : (
                domainProviders?.map((prov) => (
                  <TableRow key={prov.id} className="hover:bg-slate-800/30 transition-colors">
                    <TableCell className="text-slate-200 font-medium capitalize">{prov.provider_type}</TableCell>
                    <TableCell className="text-slate-300">{prov.display_name}</TableCell>
                    <TableCell><StatusBadge status={prov.status} /></TableCell>
                    <TableCell className="text-slate-400">
                      {prov.last_tested_at ? new Date(prov.last_tested_at).toLocaleString() : 'Never'}
                    </TableCell>
                    <TableCell className="text-slate-400">{new Date(prov.created_at).toLocaleDateString()}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end items-center gap-3">
                        <Button 
                          variant="ghost"
                          size="icon"
                          onClick={() => {
                            setSelectedDomainProv(prov);
                            setIsDomainModalOpen(true);
                          }}
                          title="Edit"
                        >
                          <Pencil size={16} />
                        </Button>

                        {deleteConfirmId === prov.id ? (
                          <div className="flex items-center gap-2">
                            <span className="text-red-400 text-xs flex items-center gap-1"><AlertTriangle size={12}/> Sure?</span>
                            <Button
                              variant="destructive"
                              size="sm"
                              onClick={() => deleteDomainProviderMutation.mutate(prov.id)}
                              disabled={deleteDomainProviderMutation.isPending}
                            >
                              Yes
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => setDeleteConfirmId(null)}
                            >
                              No
                            </Button>
                          </div>
                        ) : (
                          <Button 
                            variant="ghost"
                            size="icon"
                            onClick={() => setDeleteConfirmId(prov.id)}
                            title="Delete Provider"
                            className="text-red-400 hover:text-red-300 hover:bg-red-400/10"
                          >
                            <Trash2 size={16} />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </Card>
      </section>

      {isCloudModalOpen && <CloudCredentialModal onClose={() => setIsCloudModalOpen(false)} initialData={selectedCloudCred} />}
      {isDomainModalOpen && <DomainProviderModal onClose={() => setIsDomainModalOpen(false)} initialData={selectedDomainProv} />}
    </div>
  );
}
