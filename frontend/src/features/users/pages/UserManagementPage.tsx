import React, { useState } from 'react';
import { Users, Shield, UserPlus, Activity } from 'lucide-react';
import UserTable from '../components/UserTable';
import RoleTable from '../components/RoleTable';
import AuditLogTable from '../components/AuditLogTable';
import { Button } from '../../../components/ui/Button';

export default function UserManagementPage() {
  const [activeTab, setActiveTab] = useState<'users' | 'roles' | 'audit'>('users');

  return (
    <div className="space-y-6">
      {/* Title */}
      <div>
        <h1 className="text-2xl font-bold text-slate-100">User Management</h1>
        <p className="text-slate-400 mt-1">Manage system access, authentication accounts, and custom RBAC roles.</p>
      </div>

      {/* Tabs Layout */}
      <div className="flex border-b border-slate-800">
        <Button variant="outline"
          onClick={ () => setActiveTab('users')}
          className={`flex items-center gap-2 px-6 py-3 font-medium transition-colors border-b-2 ${
            activeTab === 'users'
              ? 'border-blue-500 text-blue-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Users size={18} />
          Users
        </Button>
        <Button variant="outline"
          onClick={ () => setActiveTab('roles')}
          className={`flex items-center gap-2 px-6 py-3 font-medium transition-colors border-b-2 ${
            activeTab === 'roles'
              ? 'border-blue-500 text-blue-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Shield size={18} />
          Roles & Permissions
        </Button>
        <Button variant="outline"
          onClick={ () => setActiveTab('audit')}
          className={`flex items-center gap-2 px-6 py-3 font-medium transition-colors border-b-2 ${
            activeTab === 'audit'
              ? 'border-blue-500 text-blue-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Activity size={18} />
          Audit Logs
        </Button>
      </div>

      {/* Tab Panels */}
      <div className="mt-6">
        {activeTab === 'users' && <UserTable />}
        {activeTab === 'roles' && <RoleTable />}
        {activeTab === 'audit' && <AuditLogTable />}
      </div>
    </div>
  );
}
