import React, { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { userApi } from '../services/userApi';
import type { User } from '../services/userApi';
import { roleApi } from '../services/roleApi';
import { X, Shield } from 'lucide-react';
import toast from 'react-hot-toast';
import { Button } from '../../../components/ui/Button';

interface UserRoleAssignModalProps {
  user: User | null;
  onClose: () => void;
}

export default function UserRoleAssignModal({ user, onClose }: UserRoleAssignModalProps) {
  const queryClient = useQueryClient();
  const [selectedRoleIds, setSelectedRoleIds] = useState<Set<string>>(new Set());

  const { data: allRoles } = useQuery({
    queryKey: ['roles'],
    queryFn: roleApi.getRoles,
    enabled: !!user,
  });

  const { data: userRoles, isLoading: loadingUserRoles } = useQuery({
    queryKey: ['userRoles', user?.id],
    queryFn: () => userApi.getUserRoles(user!.id),
    enabled: !!user,
  });

  useEffect(() => {
    if (userRoles) {
      setSelectedRoleIds(new Set(userRoles.map(r => r.id)));
    }
  }, [userRoles]);

  const mutation = useMutation({
    mutationFn: (roleIds: string[]) => userApi.updateUserRoles(user!.id, roleIds),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      queryClient.invalidateQueries({ queryKey: ['userRoles', user?.id] });
      toast.success('Roles updated successfully');
      onClose();
    },
    onError: (err: any) => {
      toast.error(`Failed to update roles: ${err.response?.data?.message || err.message}`);
    }
  });

  if (!user) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    mutation.mutate(Array.from(selectedRoleIds));
  };

  const toggleRole = (roleId: string) => {
    const newSet = new Set(selectedRoleIds);
    if (newSet.has(roleId)) {
      newSet.delete(roleId);
    } else {
      newSet.add(roleId);
    }
    setSelectedRoleIds(newSet);
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-[#12182b] border border-slate-800 rounded-xl shadow-2xl w-full max-w-md overflow-hidden flex flex-col max-h-[80vh]">
        <div className="flex justify-between items-center p-6 border-b border-slate-800 shrink-0">
          <div>
            <h2 className="text-xl font-bold text-slate-100">Assign Roles</h2>
            <p className="text-slate-400 text-sm mt-1">Configuring access for <span className="text-blue-400">{user.username}</span></p>
          </div>
          <Button onClick={onClose}  variant="ghost">
            <X size={20} />
          </Button>
        </div>
        
        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 flex flex-col">
          {loadingUserRoles ? (
            <div className="flex justify-center p-10">
              <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-blue-500"></div>
            </div>
          ) : (
            <div className="space-y-3">
              {allRoles?.map(role => (
                <div 
                  key={role.id}
                  onClick={() => toggleRole(role.id)}
                  className={`p-3 rounded-lg border cursor-pointer transition-all flex items-start gap-3 ${
                    selectedRoleIds.has(role.id) 
                      ? 'border-blue-500 bg-blue-500/10' 
                      : 'border-slate-700 bg-[#0a0f1a] hover:border-slate-500'
                  }`}
                >
                  <div className="mt-0.5 text-blue-400">
                    <Shield size={18} className={selectedRoleIds.has(role.id) ? 'block' : 'opacity-0'} />
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <span className={`font-medium ${selectedRoleIds.has(role.id) ? 'text-blue-400' : 'text-slate-200'}`}>
                        {role.name}
                      </span>
                      {role.is_builtin && <span className="bg-indigo-500/20 text-indigo-400 text-[10px] px-1.5 py-0.5 rounded">Built-in</span>}
                    </div>
                    <p className="text-xs text-slate-400 mt-1">{role.description}</p>
                  </div>
                </div>
              ))}
              
              {allRoles?.length === 0 && (
                <p className="text-slate-500 text-center py-4">No roles available.</p>
              )}
            </div>
          )}

          <div className="pt-6 mt-auto shrink-0 flex justify-end gap-3 border-t border-slate-800">
            <Button 
              type="button" 
              onClick={onClose}
              
             variant="ghost">
              Cancel
            </Button>
            <Button 
              type="submit" 
              disabled={mutation.isPending || loadingUserRoles}
              
             variant="primary">
              {mutation.isPending ? 'Saving...' : 'Save Roles'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
