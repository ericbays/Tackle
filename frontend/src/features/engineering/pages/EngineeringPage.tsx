import React, { useState } from 'react';
import { Cloud, Globe, Server } from 'lucide-react';
import ProvidersTab from '../components/ProvidersTab';
import DomainsTab from '../components/DomainsTab';
import InfrastructureTab from '../components/InfrastructureTab';
import { Button } from '../../../components/ui/Button';

export default function EngineeringPage() {
  const [activeTab, setActiveTab] = useState<'providers' | 'domains' | 'infrastructure'>('providers');

  return (
    <div className="space-y-6 max-w-7xl mx-auto w-full">
      {/* Page Header */}
      <div>
        <h1 className="text-2xl font-bold text-slate-100">Engineering</h1>
        <p className="text-slate-400 mt-1">Manage cloud credential integrations, domain registration, and active endpoint infrastructure.</p>
      </div>

      {/* Tabs Navigation */}
      <div className="flex border-b border-slate-800">
        <Button variant="outline"
          onClick={ () => setActiveTab('providers')}
          className={`flex items-center gap-2 px-6 py-3 font-medium transition-colors border-b-2 ${
            activeTab === 'providers'
              ? 'border-blue-500 text-blue-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Cloud size={18} />
          Providers
        </Button>
        <Button variant="outline"
          onClick={ () => setActiveTab('domains')}
          className={`flex items-center gap-2 px-6 py-3 font-medium transition-colors border-b-2 ${
            activeTab === 'domains'
              ? 'border-blue-500 text-blue-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Globe size={18} />
          Domains
        </Button>
        <Button variant="outline"
          onClick={ () => setActiveTab('infrastructure')}
          className={`flex items-center gap-2 px-6 py-3 font-medium transition-colors border-b-2 ${
            activeTab === 'infrastructure'
              ? 'border-blue-500 text-blue-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Server size={18} />
          Infrastructure
        </Button>
      </div>

      {/* Main Tab Content Region */}
      <div className="pt-4">
        {activeTab === 'providers' && <ProvidersTab />}
        {activeTab === 'domains' && <DomainsTab />}
        {activeTab === 'infrastructure' && <InfrastructureTab />}
      </div>
    </div>
  );
}
