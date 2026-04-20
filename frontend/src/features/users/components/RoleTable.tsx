import { useQuery } from '@tanstack/react-query';
import { roleApi } from '../services/roleApi';
import { Shield, MoreHorizontal, Settings, Users } from 'lucide-react';
import { useState } from 'react';
import { Button } from '../../../components/ui/Button';
import { Badge } from '../../../components/ui/Badge';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../../../components/ui/Card';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/Table';
import RoleForm from '../components/RoleForm';
import PermissionGate from '../../../components/auth/PermissionGate';

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
        <PermissionGate permission="roles:create">
          <Button onClick={() => setIsFormOpen(true)} variant="primary" className="flex items-center gap-2">
            <Shield size={16} /> Create Custom Role
          </Button>
        </PermissionGate>
      </div>

      <RoleForm isOpen={isFormOpen} onClose={() => setIsFormOpen(false)} />

      <Card>
        <CardHeader className="bg-[#1a2235] border-b border-slate-800 rounded-t-xl">
          <CardTitle>Built-in Roles</CardTitle>
          <CardDescription>Standard system roles. These cannot be deleted or modified.</CardDescription>
        </CardHeader>
        <Table className="border-0 shadow-none rounded-t-none">
          <TableHeader>
            <TableRow>
              <TableHead className="w-1/4 bg-[#0f141f]">Role Name</TableHead>
              <TableHead className="bg-[#0f141f]">Description</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {builtinRoles.length === 0 ? (
              <TableRow><TableCell colSpan={2} className="py-8 text-center text-slate-500">No built-in roles found.</TableCell></TableRow>
            ) : (
              builtinRoles.map(role => (
                <TableRow key={role.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <div className="w-8 h-8 rounded bg-slate-800/50 flex items-center justify-center text-indigo-400">
                        <Settings size={16} />
                      </div>
                      <span className="font-semibold text-slate-200">{role.name}</span>
                      <Badge variant="outline" className="bg-indigo-500/10 text-indigo-400 border-indigo-500/20">System</Badge>
                    </div>
                  </TableCell>
                  <TableCell className="text-slate-400">{role.description}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Card>

      <Card className="mt-6">
        <CardHeader className="bg-[#1a2235] border-b border-slate-800 rounded-t-xl">
          <CardTitle>Custom Roles</CardTitle>
          <CardDescription>Organization-specific roles with tailored permission sets.</CardDescription>
        </CardHeader>
        <Table className="border-0 shadow-none rounded-t-none">
          <TableHeader>
            <TableRow>
              <TableHead className="w-1/4 bg-[#0f141f]">Role Name</TableHead>
              <TableHead className="bg-[#0f141f]">Description</TableHead>
              <TableHead className="bg-[#0f141f] text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {customRoles.length === 0 ? (
              <TableRow><TableCell colSpan={3} className="py-12 text-center text-slate-500">No custom roles created yet.</TableCell></TableRow>
            ) : (
              customRoles.map(role => (
                <TableRow key={role.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <div className="w-8 h-8 rounded bg-slate-800/50 flex items-center justify-center text-emerald-400">
                        <Users size={16} />
                      </div>
                      <span className="font-semibold text-slate-200">{role.name}</span>
                    </div>
                  </TableCell>
                  <TableCell className="text-slate-400">{role.description}</TableCell>
                  <TableCell className="text-right">
                    <Button variant="ghost" size="icon">
                      <MoreHorizontal size={18} />
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Card>

    </div>
  );
}
