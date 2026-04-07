import React, { useState } from 'react';
import { X } from 'lucide-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { engineeringApi } from '../services/engineeringApi';
import toast from 'react-hot-toast';

interface DomainProviderModalProps {
  onClose: () => void;
  initialData?: any;
}

export default function DomainProviderModal({ onClose, initialData }: DomainProviderModalProps) {
  const queryClient = useQueryClient();
  const [providerType, setProviderType] = useState<'namecheap' | 'godaddy' | 'route53' | 'azure_dns'>(initialData?.provider_type || 'namecheap');
  const [displayName, setDisplayName] = useState(initialData?.display_name || '');
  
  // Namecheap Fields
  const [ncApiUser, setNcApiUser] = useState('');
  const [ncApiKey, setNcApiKey] = useState('');
  const [ncUsername, setNcUsername] = useState('');
  const [ncClientIp, setNcClientIp] = useState('');

  // GoDaddy Fields
  const [gdApiKey, setGdApiKey] = useState('');
  const [gdApiSecret, setGdApiSecret] = useState('');
  const [gdEnvironment, setGdEnvironment] = useState<'production' | 'ote'>('production');

  // Route53 Fields
  const [r53AccessKey, setR53AccessKey] = useState('');
  const [r53SecretKey, setR53SecretKey] = useState('');
  const [r53Region, setR53Region] = useState('us-east-1');
  const [r53RoleArn, setR53RoleArn] = useState('');

  // Azure DNS Fields
  const [azTenantId, setAzTenantId] = useState('');
  const [azClientId, setAzClientId] = useState('');
  const [azClientSecret, setAzClientSecret] = useState('');
  const [azSubId, setAzSubId] = useState('');
  const [azResourceGroup, setAzResourceGroup] = useState('');

  const mutation = useMutation({
    mutationFn: (data: any) => initialData ? engineeringApi.updateDomainProvider(initialData.id, data) : engineeringApi.createDomainProvider(data),
    onSuccess: () => {
      toast.success(initialData ? 'Registrar updated successfully' : 'Domain registrar added successfully');
      queryClient.invalidateQueries({ queryKey: ['domain-providers'] });
      onClose();
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.error?.message || 'Failed to save domain registrar');
    }
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const payload: any = {
      display_name: displayName,
    };
    
    // Type is required on create, but optional/ignored on update, we provide anyway
    if (!initialData) {
      payload.provider_type = providerType;
    }

    if (providerType === 'namecheap') {
      // NOTE: namecheap optionally checks fields if present during updates.
      // If we are passing credentials, they must be full.
      if (!initialData || ncApiUser || ncApiKey || ncClientIp) {
        payload.namecheap_credentials = {
          api_user: ncApiUser,
          api_key: ncApiKey,
          username: ncApiUser, // The Namecheap API requires both, but 99% of the time they are identical unless using delegated accounts. UX simplified.
          client_ip: ncClientIp,
        };
      }
    } else if (providerType === 'godaddy') {
      if (!initialData || gdApiKey || gdApiSecret) {
        payload.godaddy_credentials = {
          api_key: gdApiKey,
          api_secret: gdApiSecret,
          environment: gdEnvironment,
        };
      }
    } else if (providerType === 'route53') {
      if (!initialData || r53AccessKey || r53SecretKey) {
        payload.route53_credentials = {
          aws_access_key_id: r53AccessKey,
          aws_secret_access_key: r53SecretKey,
          region: r53Region,
          iam_role_arn: r53RoleArn || undefined,
        };
      }
    } else if (providerType === 'azure_dns') {
      if (!initialData || azSubId || azTenantId) {
        payload.azure_dns_credentials = {
          tenant_id: azTenantId,
          client_id: azClientId,
          client_secret: azClientSecret,
          subscription_id: azSubId,
          resource_group: azResourceGroup,
        };
      }
    }

    mutation.mutate(payload);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-[#12182b] border border-slate-700 w-full max-w-lg rounded-xl shadow-2xl p-6 max-h-[90vh] overflow-y-auto">
        <div className="flex justify-between items-center mb-6">
          <h2 className="text-xl font-semibold text-slate-100">{initialData ? 'Edit Domain Registrar' : 'Add Domain Registrar'}</h2>
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
              onChange={(e) => setProviderType(e.target.value as any)}
              className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 disabled:opacity-60"
            >
              <option value="namecheap">Namecheap</option>
              <option value="godaddy">GoDaddy</option>
              <option value="route53">Amazon Route 53</option>
              <option value="azure_dns">Azure DNS</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Display Name</label>
            <input
              required
              type="text"
              placeholder="e.g., Primary Namecheap"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500"
            />
          </div>

          {/* Namecheap Fields */}
          {providerType === 'namecheap' && (
            <div className="grid grid-cols-1 gap-4 bg-slate-800/30 p-4 rounded-lg border border-slate-700/50">
              <div className="text-xs text-slate-400 mb-2">Note: To use Namecheap API, you must whitelist your external IP Address inside your Namecheap dashboard first.</div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">API User</label>
                <input required={!initialData} type="text" value={ncApiUser} onChange={(e) => setNcApiUser(e.target.value)} placeholder={initialData ? "Leave blank to keep unchanged" : ""} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">API Key</label>
                <input required={!initialData} type="password" value={ncApiKey} onChange={(e) => setNcApiKey(e.target.value)} placeholder={initialData ? "Leave blank to keep unchanged" : ""} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Whitelisted Client IP</label>
                <input required={!initialData} type="text" placeholder={initialData ? "Leave blank to keep unchanged" : "e.g., 203.0.113.10"} value={ncClientIp} onChange={(e) => setNcClientIp(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
            </div>
          )}

          {/* GoDaddy Fields */}
          {providerType === 'godaddy' && (
            <div className="grid grid-cols-1 gap-4 bg-slate-800/30 p-4 rounded-lg border border-slate-700/50">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Environment</label>
                <select value={gdEnvironment} onChange={(e) => setGdEnvironment(e.target.value as any)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500">
                  <option value="production">Production</option>
                  <option value="ote">OTE (Testing)</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">API Key</label>
                <input required type="text" value={gdApiKey} onChange={(e) => setGdApiKey(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">API Secret</label>
                <input required type="password" value={gdApiSecret} onChange={(e) => setGdApiSecret(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
            </div>
          )}

          {/* Route53 Fields */}
          {providerType === 'route53' && (
            <div className="grid grid-cols-1 gap-4 bg-slate-800/30 p-4 rounded-lg border border-slate-700/50">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">AWS Access Key ID</label>
                <input required type="text" value={r53AccessKey} onChange={(e) => setR53AccessKey(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">AWS Secret Access Key</label>
                <input required type="password" value={r53SecretKey} onChange={(e) => setR53SecretKey(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Region</label>
                <input required type="text" placeholder="us-east-1" value={r53Region} onChange={(e) => setR53Region(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">IAM Role ARN (Optional)</label>
                <input type="text" placeholder="arn:aws:iam::..." value={r53RoleArn} onChange={(e) => setR53RoleArn(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
            </div>
          )}

          {/* Azure DNS Fields */}
          {providerType === 'azure_dns' && (
            <div className="grid grid-cols-1 gap-4 bg-slate-800/30 p-4 rounded-lg border border-slate-700/50">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Tenant ID</label>
                <input required type="text" value={azTenantId} onChange={(e) => setAzTenantId(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Subscription ID</label>
                <input required type="text" value={azSubId} onChange={(e) => setAzSubId(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Client ID</label>
                <input required type="text" value={azClientId} onChange={(e) => setAzClientId(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Client Secret</label>
                <input required type="password" value={azClientSecret} onChange={(e) => setAzClientSecret(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Resource Group</label>
                <input required type="text" value={azResourceGroup} onChange={(e) => setAzResourceGroup(e.target.value)} className="w-full bg-[#1a2235] border border-slate-700 rounded-md px-3 py-2 text-slate-200 outline-none focus:border-blue-500 font-mono text-sm" />
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
              {mutation.isPending ? 'Saving...' : initialData ? 'Update Registrar' : 'Save Registrar'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
