import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { userApi } from '../services/userApi';
import type { User } from '../services/userApi';
import { MoreHorizontal, ShieldCheck, User as UserIcon, Shield } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { Badge } from '../../../components/ui/Badge';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/Table';
import UserCreateModal from '../components/UserCreateModal';
import UserRoleAssignModal from '../components/UserRoleAssignModal';
import PermissionGate from '../../../components/auth/PermissionGate';

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
        <PermissionGate permission="users:create">
          <Button onClick={() => setIsCreateModalOpen(true)} variant="primary">
            Create User
          </Button>
        </PermissionGate>
      </div>

      <UserCreateModal isOpen={isCreateModalOpen} onClose={() => setIsCreateModalOpen(false)} />
      <UserRoleAssignModal user={roleAssignUser} onClose={() => setRoleAssignUser(null)} />

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Username</TableHead>
            <TableHead>Display Name</TableHead>
            <TableHead>Email</TableHead>
            <TableHead>Provider</TableHead>
            <TableHead>Status</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users?.length === 0 ? (
            <TableRow>
              <TableCell colSpan={6} className="py-12 text-center text-slate-500">
                No users found.
              </TableCell>
            </TableRow>
          ) : (
            users?.map((user) => (
              <TableRow key={user.id}>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <div className="w-8 h-8 rounded bg-slate-800 flex items-center justify-center text-blue-400">
                      <UserIcon size={16} />
                    </div>
                    <span className="font-medium text-slate-200">{user.username}</span>
                    {user.is_initial_admin && (
                      <Badge variant="secondary" className="bg-purple-500/10 text-purple-400 font-mono">
                        <ShieldCheck size={12} className="mr-1" /> Admin
                      </Badge>
                    )}
                  </div>
                </TableCell>
                <TableCell className="text-slate-300">{user.display_name}</TableCell>
                <TableCell className="text-slate-400 font-mono">{user.email}</TableCell>
                <TableCell>
                  <Badge variant="secondary">{user.auth_provider || 'local'}</Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={user.status === 'active' ? 'success' : 'warning'}>
                    <div className={`w-1.5 h-1.5 mr-1.5 rounded-full ${user.status === 'active' ? 'bg-emerald-400' : 'bg-orange-400'}`}></div>
                    {user.status || 'active'}
                  </Badge>
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex justify-end gap-2 text-slate-400">
                    <PermissionGate permission="users:update">
                      <Button 
                        variant="ghost" 
                        size="icon"
                        onClick={() => setRoleAssignUser(user)}
                        title="Assign Roles"
                      >
                        <Shield size={16} />
                      </Button>
                    </PermissionGate>
                    <Button variant="ghost" size="icon">
                      <MoreHorizontal size={18} />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  );
}
