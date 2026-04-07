import React, { useState } from 'react';
import { X } from 'lucide-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { engineeringApi } from '../services/engineeringApi';
import toast from 'react-hot-toast';

interface CloudCredentialModalProps {
  onClose: () => void;
  initialData?: any;
}

export default function CloudCredentialModal({ onClose, initialData }: CloudCredentialModalProps) {
  const queryClient = useQueryClient();
  const [providerType, setProviderType] = useState<'aws' | 'azure'>(initialData?.provider_type || 'aws');
  const [displayName, setDisplayName] = useState(initialData?.display_name || '');
  const [defaultRegion, setDefaultRegion] = useState(initialData?.default_region || 'us-east-1');
  
  // AWS Fields
  const [awsAccessKeyId, setAwsAccessKeyId] = useState('');
  const [awsSecretKey, setAwsSecretKey] = useState('');
  
  // Azure Fields
  const [azureTenantId, setAzureTenantId] = useState('');
  const [azureClientId, setAzureClientId] = useState('');
  const [azureClientSecret, setAzureClientSecret] = useState('');
  const [azureSubscriptionId, setAzureSubscriptionId] = useState('');

  const mutation = useMutation({
    mutationFn: (data: any) => initialData ? engineeringApi.updateCloudCredential(initialData.id, data) : engineeringApi.createCloudCredential(data),
    onSuccess: () => {
      toast.success(initialData ? 'Cloud credentials updated successfully' : 'Cloud credentials added successfully');
      queryClient.invalidateQueries({ queryKey: ['cloud-credentials'] });
      onClose();
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.error?.message || 'Failed to save cloud credential');
    }
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const payload: any = {
      display_name: displayName,
      default_region: defaultRegion,
    };
    
    if (!initialData) {
      payload.provider_type = providerType;
    }

    if (providerType === 'aws') {
      if (!initialData || awsAccessKeyId || awsSecretKey) {
        payload.aws = {
          access_key_id: awsAccessKeyId,
          secret_access_key: awsSecretKey,
        };
      }
    } else {
      if (!initialData || azureTenantId || azureClientId || azureClientSecret || azureSubscriptionId) {
        payload.azure = {
          tenant_id: azureTenantId,
          client_id: azureClientId,
          client_secret: azureClientSecret,
          subscription_id: azureSubscriptionId,
        };
      }
    }

    mutation.mutate(payload);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-[#12182b] border border-slate-700 w-full max-w-lg rounded-xl shadow-2xl p-6">
        <div className="flex justify-between items-center mb-6">
          <h2 className="text-xl font-semibold text-slate-100">{initialData ? 'Edit Cloud Credential' : 'Add Cloud Credential'}</h2>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200 p-1 rounded-md transition-colors hover:bg-slate-800">
            <X size={20} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Provider Type</label>
            <select
              required
              disabled={!!initialData}
              value={providerType}
              onChange={(e) => setProviderType(e.target.value as 'aws' | 'azure')}
              className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 disabled:opacity-60"
            >
              <option value="aws">Amazon Web Services (AWS)</option>
              <option value="azure">Microsoft Azure</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Display Name</label>
            <input
              required
              type="text"
              placeholder="e.g., Production AWS Account"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Default Region</label>
            <input
              required
              type="text"
              placeholder={providerType === 'aws' ? 'us-east-1' : 'eastus'}
              value={defaultRegion}
              onChange={(e) => setDefaultRegion(e.target.value)}
              className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500"
            />
          </div>

          {providerType === 'aws' && (
            <div className="grid grid-cols-1 gap-4 bg-slate-800/30 p-4 rounded-lg border border-slate-700/50">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Access Key ID</label>
                <input
                  required={!initialData}
                  type="text"
                  placeholder={initialData ? "Leave blank to keep unchanged" : ""}
                  value={awsAccessKeyId}
                  onChange={(e) => setAwsAccessKeyId(e.target.value)}
                  className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Secret Access Key</label>
                <input
                  required={!initialData}
                  type="password"
                  placeholder={initialData ? "Leave blank to keep unchanged" : ""}
                  value={awsSecretKey}
                  onChange={(e) => setAwsSecretKey(e.target.value)}
                  className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm"
                />
              </div>
            </div>
          )}

          {providerType === 'azure' && (
            <div className="grid grid-cols-1 gap-4 bg-slate-800/30 p-4 rounded-lg border border-slate-700/50">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Tenant ID</label>
                <input
                  required
                  type="text"
                  value={azureTenantId}
                  onChange={(e) => setAzureTenantId(e.target.value)}
                  className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Subscription ID</label>
                <input
                  required
                  type="text"
                  value={azureSubscriptionId}
                  onChange={(e) => setAzureSubscriptionId(e.target.value)}
                  className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Client ID</label>
                <input
                  required
                  type="text"
                  value={azureClientId}
                  onChange={(e) => setAzureClientId(e.target.value)}
                  className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Client Secret</label>
                <input
                  required
                  type="password"
                  value={azureClientSecret}
                  onChange={(e) => setAzureClientSecret(e.target.value)}
                  className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm"
                />
              </div>
            </div>
          )}

          <div className="flex justify-end gap-3 mt-8 pt-4 border-t border-slate-800">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 bg-transparent text-slate-300 hover:text-white rounded-md transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={mutation.isPending}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-md transition-colors flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {mutation.isPending && (
                <div className="w-4 h-4 border-2 border-white/20 border-t-white rounded-full animate-spin" />
              )}
              {mutation.isPending ? 'Saving...' : initialData ? 'Update Credentials' : 'Save Credentials'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
