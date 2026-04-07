import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { userApi } from '../services/userApi';
import type { User } from '../services/userApi';
import { MoreHorizontal, ShieldCheck, User as UserIcon, Shield } from 'lucide-react';
import UserCreateModal from '../components/UserCreateModal';
import UserRoleAssignModal from '../components/UserRoleAssignModal';

export default function UserTable() {
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [roleAssignUser, setRoleAssignUser] = useState<User | null>(null);
  const { data: users, isLoading, isError, error } = useQuery({
    queryKey: ['users'],
    queryFn: userApi.getUsers,
  });

  if (isLoading) {
    return <div className="flex bg-[#12182b] border border-slate-800 rounded-lg p-10 items-center justify-center min-h-[400px]">
      <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-blue-500"></div>
    </div>;
  }

  if (isError) {
    return <div className="bg-red-900/20 border border-red-500/50 rounded-lg p-6 text-red-400">
      <h3 className="text-lg font-medium mb-2">Error loading users</h3>
      <p>{error instanceof Error ? error.message : 'Unknown error occurred'}</p>
    </div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-end">
        <button 
          onClick={() => setIsCreateModalOpen(true)}
          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-md font-medium transition-colors"
        >
          Create User
        </button>
      </div>

      <UserCreateModal isOpen={isCreateModalOpen} onClose={() => setIsCreateModalOpen(false)} />
      <UserRoleAssignModal user={roleAssignUser} onClose={() => setRoleAssignUser(null)} />

      <div className="bg-[#12182b] border border-slate-800 rounded-lg overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="bg-[#1a2235] text-slate-300 font-medium border-b border-slate-800">
              <tr>
                <th className="px-6 py-4">Username</th>
                <th className="px-6 py-4">Display Name</th>
                <th className="px-6 py-4">Email</th>
                <th className="px-6 py-4">Provider</th>
                <th className="px-6 py-4">Status</th>
                <th className="px-6 py-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {users?.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-slate-500">
                    No users found.
                  </td>
                </tr>
              ) : (
                users?.map((user) => (
                  <tr key={user.id} className="hover:bg-slate-800/30 transition-colors">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <div className="w-8 h-8 rounded bg-slate-800 flex items-center justify-center text-blue-400">
                          <UserIcon size={16} />
                        </div>
                        <span className="font-medium text-slate-200">{user.username}</span>
                        {user.is_initial_admin && (
                          <span className="bg-purple-500/20 text-purple-400 text-xs px-2 py-0.5 rounded flex items-center gap-1">
                            <ShieldCheck size={12} /> Admin
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="px-6 py-4 text-slate-300">{user.display_name}</td>
                    <td className="px-6 py-4 text-slate-400 font-mono text-xs">{user.email}</td>
                    <td className="px-6 py-4">
                      <span className="bg-slate-800 text-slate-300 text-xs px-2 py-1 rounded">
                        {user.auth_provider || 'local'}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <span className={`text-xs px-2 py-1 rounded inline-flex items-center gap-1 ${
                        user.status === 'active' ? 'bg-green-500/10 text-green-400' : 'bg-yellow-500/10 text-yellow-500'
                      }`}>
                        <div className={`w-1.5 h-1.5 rounded-full ${user.status === 'active' ? 'bg-green-400' : 'bg-yellow-500'}`}></div>
                        {user.status || 'active'}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-right">
                      <div className="flex justify-end gap-2 text-slate-400">
                        <button 
                          onClick={() => setRoleAssignUser(user)}
                          className="hover:text-blue-400 p-1.5 rounded bg-slate-800/50 hover:bg-slate-800 transition-colors"
                          title="Assign Roles"
                        >
                          <Shield size={16} />
                        </button>
                        <button className="hover:text-white p-1.5 rounded hover:bg-slate-700 transition-colors">
                          <MoreHorizontal size={18} />
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
