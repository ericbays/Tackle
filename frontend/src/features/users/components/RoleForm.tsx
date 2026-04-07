import React, { useState, useEffect } from 'react';
import { useMutation, useQueryClient, useQuery } from '@tanstack/react-query';
import { roleApi } from '../services/roleApi';
import { X, Check } from 'lucide-react';
import toast from 'react-hot-toast';
import type { Role } from '../services/userApi';

interface RoleFormProps {
  isOpen: boolean;
  onClose: () => void;
  roleToEdit?: Role;
}

export default function RoleForm({ isOpen, onClose, roleToEdit }: RoleFormProps) {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState({
    name: roleToEdit?.name || '',
    description: roleToEdit?.description || '',
  });

  const [selectedPermissions, setSelectedPermissions] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (roleToEdit?.permissions) {
      setSelectedPermissions(new Set(roleToEdit.permissions));
    } else {
      setSelectedPermissions(new Set());
    }
    setFormData({
      name: roleToEdit?.name || '',
      description: roleToEdit?.description || '',
    });
  }, [roleToEdit, isOpen]);

  const { data: permissionsList, isLoading: loadingPerms } = useQuery({
    queryKey: ['permissions'],
    queryFn: roleApi.getPermissions,
    enabled: isOpen,
  });

  const mutation = useMutation({
    mutationFn: (data: Partial<Role>) => 
      roleToEdit ? roleApi.updateRole(roleToEdit.id, data) : roleApi.createRole(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      toast.success(roleToEdit ? 'Role updated successfully' : 'Role created successfully');
      onClose();
      if (!roleToEdit) {
        setFormData({ name: '', description: '' });
        setSelectedPermissions(new Set());
      }
    },
    onError: (err: any) => {
      toast.error(`Failed to save role: ${err.response?.data?.message || err.message}`);
    }
  });

  if (!isOpen) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    mutation.mutate({
      ...formData,
      permissions: Array.from(selectedPermissions)
    });
  };

  const togglePermission = (permId: string) => {
    const newSet = new Set(selectedPermissions);
    if (newSet.has(permId)) {
      newSet.delete(permId);
    } else {
      newSet.add(permId);
    }
    setSelectedPermissions(newSet);
  };

  // Group permissions by resource
  const groupedPermissions = permissionsList?.reduce((acc, curr) => {
    if (!acc[curr.resource]) {
      acc[curr.resource] = [];
    }
    acc[curr.resource].push(curr);
    return acc;
  }, {} as Record<string, typeof permissionsList>) || {};

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-[#12182b] border border-slate-800 rounded-xl shadow-2xl w-full max-w-2xl overflow-hidden flex flex-col max-h-[90vh]">
        <div className="flex justify-between items-center p-6 border-b border-slate-800 shrink-0">
          <h2 className="text-xl font-bold text-slate-100">{roleToEdit ? 'Edit Role' : 'Create Custom Role'}</h2>
          <button onClick={onClose} className="text-slate-400 hover:text-white transition-colors">
            <X size={20} />
          </button>
        </div>
        
        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 flex flex-col gap-6">
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">Role Name</label>
              <input 
                type="text" 
                required
                className="w-full bg-[#0a0f1a] border border-slate-700 rounded-lg px-4 py-2 text-slate-200 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                value={formData.name}
                onChange={e => setFormData({...formData, name: e.target.value})}
                disabled={roleToEdit?.is_builtin}
              />
            </div>
            
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">Description</label>
              <textarea 
                required
                className="w-full bg-[#0a0f1a] border border-slate-700 rounded-lg px-4 py-2 text-slate-200 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 min-h-[60px]"
                value={formData.description}
                onChange={e => setFormData({...formData, description: e.target.value})}
                disabled={roleToEdit?.is_builtin}
              />
            </div>
          </div>

          <div>
            <h3 className="text-sm font-medium text-slate-300 mb-3 border-b border-slate-800 pb-2">Permissions Matrix</h3>
            {loadingPerms ? (
              <div className="flex justify-center p-8">
                <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-blue-500"></div>
              </div>
            ) : (
              <div className="space-y-4">
                {Object.entries(groupedPermissions).map(([resource, perms]) => (
                  <div key={resource} className="bg-[#0f141f] border border-slate-800 rounded-lg overflow-hidden">
                    <div className="bg-[#1a2235] px-4 py-2 border-b border-slate-800 text-sm font-semibold text-slate-200 capitalize">
                      {resource}
                    </div>
                    <div className="p-3 grid grid-cols-2 sm:grid-cols-3 gap-2">
                      {perms.map(p => {
                        const isSelected = selectedPermissions.has(p.permission);
                        return (
                          <label 
                            key={p.permission}
                            className={`flex items-center gap-2 p-2 rounded border cursor-pointer select-none transition-colors ${
                              isSelected ? 'bg-blue-500/10 border-blue-500/50 text-blue-400' : 'bg-[#0a0f1a] border-slate-800 text-slate-400 hover:border-slate-700'
                            }`}
                          >
                            <div className={`w-4 h-4 rounded border flex items-center justify-center shrink-0 ${
                              isSelected ? 'bg-blue-500 border-blue-500 text-white' : 'border-slate-600'
                            }`}>
                              {isSelected && <Check size={12} strokeWidth={3} />}
                            </div>
                            <span className="text-xs uppercase font-medium">{p.action}</span>
                          </label>
                        )
                      })}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="pt-4 mt-auto shrink-0 flex justify-end gap-3 border-t border-slate-800">
            <button 
              type="button" 
              onClick={onClose}
              className="px-4 py-2 mt-4 rounded-lg text-slate-300 hover:text-white hover:bg-slate-800 transition-colors"
            >
              Cancel
            </button>
            <button 
              type="submit" 
              disabled={mutation.isPending || roleToEdit?.is_builtin}
              className="mt-4 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white px-6 py-2 rounded-lg font-medium transition-colors"
            >
              {mutation.isPending ? 'Saving...' : 'Save Role'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
