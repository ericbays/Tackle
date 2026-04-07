import { useQuery } from '@tanstack/react-query';
import { roleApi } from '../services/roleApi';
import { Shield, MoreHorizontal, Settings, Users } from 'lucide-react';
import { useState } from 'react';
import RoleForm from '../components/RoleForm';

export default function RoleTable() {
  const [isFormOpen, setIsFormOpen] = useState(false);
  const { data: roles, isLoading, isError, error } = useQuery({
    queryKey: ['roles'],
    queryFn: roleApi.getRoles,
  });

  if (isLoading) {
    return <div className="flex bg-[#12182b] border border-slate-800 rounded-lg p-10 items-center justify-center min-h-[400px]">
      <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-blue-500"></div>
    </div>;
  }

  if (isError) {
    return <div className="bg-red-900/20 border border-red-500/50 rounded-lg p-6 text-red-400">
      <h3 className="text-lg font-medium mb-2">Error loading roles</h3>
      <p>{error instanceof Error ? error.message : 'Unknown error occurred'}</p>
    </div>;
  }

  const builtinRoles = roles?.filter(r => r.is_builtin) || [];
  const customRoles = roles?.filter(r => !r.is_builtin) || [];

  return (
    <div className="space-y-6">
      <div className="flex justify-end">
        <button 
          onClick={() => setIsFormOpen(true)}
          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-md font-medium transition-colors flex items-center gap-2"
        >
          <Shield size={16} /> Create Custom Role
        </button>
      </div>

      <RoleForm isOpen={isFormOpen} onClose={() => setIsFormOpen(false)} />

      <div className="bg-[#12182b] border border-slate-800 rounded-lg overflow-hidden">
        <div className="p-4 border-b border-slate-800 bg-[#1a2235]">
          <h2 className="text-lg font-medium text-slate-200">Built-in Roles</h2>
          <p className="text-sm text-slate-400">Standard system roles. These cannot be deleted or modified.</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="bg-[#0f141f] text-slate-400 font-medium">
              <tr>
                <th className="px-6 py-3 w-1/4">Role Name</th>
                <th className="px-6 py-3">Description</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {builtinRoles.length === 0 ? (
                <tr><td colSpan={2} className="px-6 py-8 text-center text-slate-500">No built-in roles found.</td></tr>
              ) : (
                builtinRoles.map(role => (
                  <tr key={role.id} className="hover:bg-slate-800/30">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <div className="w-8 h-8 rounded bg-slate-800/50 flex items-center justify-center text-indigo-400">
                          <Settings size={16} />
                        </div>
                        <span className="font-semibold text-slate-200">{role.name}</span>
                        <span className="bg-indigo-500/10 text-indigo-400 text-xs px-2 py-0.5 rounded border border-indigo-500/20">System</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-slate-400">{role.description}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      <div className="bg-[#12182b] border border-slate-800 rounded-lg overflow-hidden mt-6">
        <div className="p-4 border-b border-slate-800 bg-[#1a2235]">
          <h2 className="text-lg font-medium text-slate-200">Custom Roles</h2>
          <p className="text-sm text-slate-400">Organization-specific roles with tailored permission sets.</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="bg-[#0f141f] text-slate-400 font-medium">
              <tr>
                <th className="px-6 py-3 w-1/4">Role Name</th>
                <th className="px-6 py-3">Description</th>
                <th className="px-6 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {customRoles.length === 0 ? (
                <tr><td colSpan={3} className="px-6 py-12 text-center text-slate-500">No custom roles created yes.</td></tr>
              ) : (
                customRoles.map(role => (
                  <tr key={role.id} className="hover:bg-slate-800/30">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <div className="w-8 h-8 rounded bg-slate-800/50 flex items-center justify-center text-emerald-400">
                          <Users size={16} />
                        </div>
                        <span className="font-semibold text-slate-200">{role.name}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-slate-400">{role.description}</td>
                    <td className="px-6 py-4 text-right">
                      <button className="text-slate-400 hover:text-white p-1 rounded hover:bg-slate-700 transition-colors">
                        <MoreHorizontal size={18} />
                      </button>
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
