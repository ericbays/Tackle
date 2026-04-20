import React, { useState } from 'react';
import { X, Plus, Trash2, Globe, AlertTriangle, Pencil, Check } from 'lucide-react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { engineeringApi } from '../services/engineeringApi';
import toast from 'react-hot-toast';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Button } from '../../../components/ui/Button';

interface ManageDnsModalProps {
  onClose: () => void;
  domainId: string;
  domainName: string;
}

export default function ManageDnsModal({ onClose, domainId, domainName }: ManageDnsModalProps) {
  const queryClient = useQueryClient();
  
  const [newType, setNewType] = useState('A');
  const [newName, setNewName] = useState('@');
  const [newValue, setNewValue] = useState('');
  const [newTtl, setNewTtl] = useState('300');
  
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [editingRecordId, setEditingRecordId] = useState<string | null>(null);
  const [editData, setEditData] = useState({ type: 'A', name: '@', value: '', ttl: '300' });

  const { data: records, isLoading } = useQuery({
    queryKey: ['dns-records', domainId],
    queryFn: () => engineeringApi.getDomainDnsRecords(domainId),
  });

  const createMutation = useMutation({
    mutationFn: (record: any) => engineeringApi.createDomainDnsRecord({ domainId, record }),
    onSuccess: () => {
      toast.success('DNS record created successfully');
      queryClient.invalidateQueries({ queryKey: ['dns-records', domainId] });
      setNewName('@');
      setNewValue('');
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.error?.message || 'Failed to create DNS record');
    }
  });

  const updateMutation = useMutation({
    mutationFn: (params: { recordId: string, record: any }) => engineeringApi.updateDomainDnsRecord({ domainId, recordId: params.recordId, record: params.record }),
    onSuccess: () => {
      toast.success('DNS record updated successfully');
      queryClient.invalidateQueries({ queryKey: ['dns-records', domainId] });
      setEditingRecordId(null);
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.error?.message || 'Failed to update DNS record');
    }
  });

  const deleteMutation = useMutation({
    mutationFn: (recordId: string) => engineeringApi.deleteDomainDnsRecord({ domainId, recordId }),
    onSuccess: () => {
      toast.success('DNS record deleted successfully');
      queryClient.invalidateQueries({ queryKey: ['dns-records', domainId] });
      setDeleteConfirmId(null);
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.error?.message || 'Failed to delete DNS record');
      setDeleteConfirmId(null);
    }
  });

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newValue) return;
    createMutation.mutate({
      type: newType,
      name: newName,
      value: newValue,
      ttl: parseInt(newTtl, 10) || 300
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="bg-[#12182b] border border-slate-700 w-full max-w-5xl rounded-xl shadow-2xl flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex justify-between items-center p-6 border-b border-slate-800">
          <div>
            <h2 className="text-xl font-semibold text-slate-100 flex items-center gap-2">
              <Globe className="text-blue-400" size={20} />
              Manage DNS Records
            </h2>
            <p className="text-slate-400 text-sm mt-1">Editing zone file for <span className="text-slate-200 font-medium">{domainName}</span></p>
          </div>
          <Button onClick={onClose}  variant="ghost">
            <X size={20} />
          </Button>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto flex-1 space-y-6">
          <div className="bg-[#1a2235] border border-slate-700 rounded-lg overflow-hidden">
            <table className="w-full text-left text-sm whitespace-nowrap">
              <thead className="bg-slate-800 text-slate-300 font-medium">
                <tr>
                  <th className="px-6 py-3 w-32">Type</th>
                  <th className="px-6 py-3">Host / Name</th>
                  <th className="px-6 py-3">Value / Target</th>
                  <th className="px-6 py-3 w-24">TTL</th>
                  <th className="px-6 py-3 w-24 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {/* New Record Row */}
                <tr className="bg-slate-800/30">
                  <td className="px-4 py-3">
                    <Select 
                      value={newType} 
                      onChange={(e) => setNewType(e.target.value)}
                      className="w-full bg-[#12182b] border border-slate-700 rounded px-2 py-1.5 text-slate-200 outline-none focus:border-blue-500 text-sm font-mono"
                    >
                      <option value="A">A</option>
                      <option value="AAAA">AAAA</option>
                      <option value="CNAME">CNAME</option>
                      <option value="TXT">TXT</option>
                      <option value="MX">MX</option>
                      <option value="NS">NS</option>
                    </Select>
                  </td>
                  <td className="px-4 py-3">
                    <Input 
                      type="text" 
                      value={newName} 
                      onChange={(e) => setNewName(e.target.value)}
                      placeholder="@"
                      className="w-full bg-[#12182b] border border-slate-700 rounded px-3 py-1.5 text-slate-200 outline-none focus:border-blue-500 text-sm font-mono"
                    />
                  </td>
                  <td className="px-4 py-3">
                    <Input 
                      type="text" 
                      value={newValue} 
                      onChange={(e) => setNewValue(e.target.value)}
                      placeholder="192.168.1.1"
                      className="w-full bg-[#12182b] border border-slate-700 rounded px-3 py-1.5 text-slate-200 outline-none focus:border-blue-500 text-sm font-mono"
                    />
                  </td>
                  <td className="px-4 py-3">
                    <Input 
                      type="text" 
                      value={newTtl} 
                      onChange={(e) => setNewTtl(e.target.value)}
                      className="w-full bg-[#12182b] border border-slate-700 rounded px-3 py-1.5 text-slate-200 outline-none focus:border-blue-500 text-sm font-mono"
                    />
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Button 
                      onClick={handleCreate}
                      disabled={!newValue || createMutation.isPending}
                      
                     variant="primary" size="sm">
                      {createMutation.isPending ? 'Adding...' : <><Plus size={14} /> Add</>}
                    </Button>
                  </td>
                </tr>

                {/* Existing Records */}
                {isLoading ? (
                  <tr><td colSpan={5} className="px-6 py-8 text-center text-slate-500">Loading records from provider...</td></tr>
                ) : records?.length === 0 ? (
                  <tr><td colSpan={5} className="px-6 py-8 text-center text-slate-500">No records found.</td></tr>
                ) : (
                  records?.map((record: any) => (
                    editingRecordId === record.id ? (
                      <tr key={record.id} className="bg-slate-800/30">
                        <td className="px-4 py-3">
                          <Select 
                            value={editData.type} 
                            onChange={(e) => setEditData({...editData, type: e.target.value})}
                            className="w-full bg-[#12182b] border border-slate-700 rounded px-2 py-1 text-slate-200 outline-none focus:border-blue-500 text-sm font-mono"
                          >
                            <option value="A">A</option>
                            <option value="AAAA">AAAA</option>
                            <option value="CNAME">CNAME</option>
                            <option value="TXT">TXT</option>
                            <option value="MX">MX</option>
                            <option value="NS">NS</option>
                          </Select>
                        </td>
                        <td className="px-4 py-3">
                          <Input 
                            type="text" 
                            value={editData.name} 
                            onChange={(e) => setEditData({...editData, name: e.target.value})}
                            className="w-full bg-[#12182b] border border-slate-700 rounded px-2 py-1 text-slate-200 outline-none focus:border-blue-500 text-sm font-mono"
                          />
                        </td>
                        <td className="px-4 py-3">
                          <Input 
                            type="text" 
                            value={editData.value} 
                            onChange={(e) => setEditData({...editData, value: e.target.value})}
                            className="w-full bg-[#12182b] border border-slate-700 rounded px-2 py-1 text-slate-200 outline-none focus:border-blue-500 text-sm font-mono"
                          />
                        </td>
                        <td className="px-4 py-3">
                          <Input 
                            type="text" 
                            value={editData.ttl} 
                            onChange={(e) => setEditData({...editData, ttl: e.target.value})}
                            className="w-full bg-[#12182b] border border-slate-700 rounded px-2 py-1 text-slate-200 outline-none focus:border-blue-500 text-sm font-mono"
                          />
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex items-center justify-end gap-2">
                            <Button variant="outline" 
                              onClick={ () => {
                                updateMutation.mutate({
                                  recordId: record.id,
                                  record: {
                                    type: editData.type,
                                    name: editData.name,
                                    value: editData.value,
                                    ttl: parseInt(editData.ttl, 10) || parseInt(record.ttl, 10) || 300
                                  }
                                });
                              }}
                              disabled={!editData.value || updateMutation.isPending}
                              className="text-green-500 hover:text-green-400 p-1 rounded transition-colors disabled:opacity-50"
                              title="Save"
                            >
                              <Check size={16} />
                            </Button>
                            <Button variant="outline" 
                              onClick={ () => setEditingRecordId(null)}
                              className="text-slate-500 hover:text-slate-300 p-1 rounded transition-colors"
                              title="Cancel"
                            >
                              <X size={16} />
                            </Button>
                          </div>
                        </td>
                      </tr>
                    ) : (
                      <tr key={record.id} className="hover:bg-slate-800/30 transition-colors">
                        <td className="px-6 py-3">
                          <span className="font-mono text-blue-400 font-medium">{record.type}</span>
                        </td>
                        <td className="px-6 py-3 text-slate-300 font-mono text-sm">{record.name}</td>
                        <td className="px-6 py-3 text-slate-400 font-mono text-sm truncate max-w-xs" title={record.value}>{record.value}</td>
                        <td className="px-6 py-3 text-slate-500 font-mono text-sm">{record.ttl}</td>
                        <td className="px-6 py-3 text-right">
                          {deleteConfirmId === record.id ? (
                            <div className="flex items-center justify-end gap-2">
                              <span className="text-red-400 text-xs flex items-center gap-1"><AlertTriangle size={12}/> Sure?</span>
                              <Button variant="outline" 
                                onClick={ () => deleteMutation.mutate(record.id)}
                                disabled={deleteMutation.isPending}
                                className="text-red-400 hover:text-red-300 bg-red-400/10 hover:bg-red-400/20 px-2 py-1 rounded text-xs transition-colors"
                              >
                                Yes
                              </Button>
                              <Button variant="outline" 
                                onClick={ () => setDeleteConfirmId(null)}
                                className="text-slate-400 hover:text-slate-300 px-2 py-1 text-xs"
                              >
                                No
                              </Button>
                            </div>
                          ) : (
                            <div className="flex items-center justify-end gap-2">
                              <Button variant="outline" 
                                onClick={ () => {
                                  setEditData({ type: record.type, name: record.name, value: record.value, ttl: String(record.ttl) });
                                  setEditingRecordId(record.id);
                                }}
                                className="text-slate-500 hover:text-blue-400 p-1 rounded transition-colors"
                                title="Edit Record"
                              >
                                <Pencil size={16} />
                              </Button>
                              <Button variant="outline" 
                                onClick={ () => setDeleteConfirmId(record.id)}
                                className="text-slate-500 hover:text-red-400 p-1 rounded transition-colors"
                                title="Delete Record"
                              >
                                <Trash2 size={16} />
                              </Button>
                            </div>
                          )}
                        </td>
                      </tr>
                    )
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}
