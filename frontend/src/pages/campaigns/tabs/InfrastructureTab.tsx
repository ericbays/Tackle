import { Cloud, Server, Globe, ShieldCheck, Send } from 'lucide-react';
import { useParams, Link } from 'react-router-dom';
import { useCampaignStore } from '../../../store/campaignStore';
import { useSMTPStore } from '../../../store/smtpStore';
import { useCloudCredentials, useDomains } from '../../../hooks/useConfigurations';
import PermissionGate from '../../../components/auth/PermissionGate';
import { useEffect } from 'react';
import { Select } from '../../../components/ui/Select';
import { Button } from '../../../components/ui/Button';

export const PROVIDER_CONFIGS: Record<string, { regions: string[]; sizes: string[] }> = {
    aws: {
        regions: ['us-east-1 (N. Virginia)', 'us-east-2 (Ohio)', 'us-west-1 (N. California)', 'us-west-2 (Oregon)', 'eu-west-1 (Ireland)'],
        sizes: ['t3.micro', 't3.small', 't3.medium', 't3.large']
    },
    azure: {
        regions: ['eastus', 'eastus2', 'westus', 'centralus', 'westeurope'],
        sizes: ['Standard_B1s', 'Standard_B1ms', 'Standard_B2s']
    },
    proxmox: {
        regions: ['local', 'cluster-1'],
        sizes: ['1vCPU, 1GB RAM', '2vCPU, 2GB RAM', '4vCPU, 4GB RAM']
    }
};

export default function InfrastructureTab() {
    const { id } = useParams();
    const { draft: { infrastructure }, updateInfrastructure, isSaving, saveCampaign } = useCampaignStore();

    const { profiles, fetchProfiles } = useSMTPStore();
    useEffect(() => {
        fetchProfiles();
    }, [fetchProfiles]);

    const { data: cloudCredentials = [], isLoading: isLoadingCreds } = useCloudCredentials();
    const { data: domains = [], isLoading: isLoadingDomains } = useDomains();

    const selectedConfig = PROVIDER_CONFIGS[infrastructure.provider?.toLowerCase()] || { regions: [], sizes: [] };

    return (
        <div className="space-y-8">
            <section className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
                <div className="p-6">
                    <div className="flex items-center gap-3 mb-6">
                        <Cloud className="w-5 h-5 text-blue-400" />
                        <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">Cloud Host Selection</h2>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        <div>
                            <label className="block text-sm font-medium text-slate-400 mb-2">Provider Profile</label>
                            {isLoadingCreds ? (
                                <div className="text-slate-500 text-sm py-2">Loading configured providers...</div>
                            ) : (
                                <Select 
                                    value={infrastructure.provider}
                                    onChange={(e) => updateInfrastructure({ provider: e.target.value, region: '', instanceSize: '' })}
                                    className="w-full bg-slate-950 border border-slate-800 rounded-md px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
                                >
                                    <option value="">Select a configured provider...</option>
                                    {cloudCredentials.map((cred) => (
                                        <option key={cred.id} value={cred.provider_type}>
                                            {cred.display_name} ({cred.provider_type})
                                        </option>
                                    ))}
                                </Select>
                            )}
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-400 mb-2">Region</label>
                            <Select 
                                value={infrastructure.region}
                                onChange={(e) => updateInfrastructure({ region: e.target.value })}
                                className="w-full bg-slate-950 border border-slate-800 rounded-md px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500 disabled:opacity-50"
                                disabled={!infrastructure.provider || selectedConfig.regions.length === 0}
                            >
                                <option value="">Select Region...</option>
                                {selectedConfig.regions.map(r => (
                                    <option key={r} value={r}>{r}</option>
                                ))}
                            </Select>
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-400 mb-2">Instance Size</label>
                            <Select 
                                value={infrastructure.instanceSize}
                                onChange={(e) => updateInfrastructure({ instanceSize: e.target.value })}
                                className="w-full bg-slate-950 border border-slate-800 rounded-md px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500 disabled:opacity-50"
                                disabled={!infrastructure.provider || selectedConfig.sizes.length === 0}
                            >
                                <option value="">Select Size...</option>
                                {selectedConfig.sizes.map(s => (
                                    <option key={s} value={s}>{s}</option>
                                ))}
                            </Select>
                            {infrastructure.instanceSize && (
                                <p className="mt-2 text-xs text-slate-500">Recommended for your configured targets</p>
                            )}
                        </div>
                    </div>
                </div>
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
                <div className="p-6">
                    <div className="flex items-center justify-between mb-6">
                        <div className="flex items-center gap-3">
                            <Send className="w-5 h-5 text-indigo-400" />
                            <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">SMTP Relay Profile</h2>
                        </div>
                        <Link to="/smtp-profiles/new" className="text-xs text-blue-400 hover:text-blue-300">
                            + Register New Profile
                        </Link>
                    </div>

                    <div className="max-w-md">
                        <label className="block text-sm font-medium text-slate-400 mb-2">Outgoing Mail Server</label>
                        <Select 
                            value={infrastructure.smtpProfileId || ''}
                            onChange={(e) => updateInfrastructure({ smtpProfileId: e.target.value })}
                            className="w-full bg-slate-950 border border-slate-800 rounded-md px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
                        >
                            <option value="">Select an Operations SMTP profile...</option>
                            {profiles.map((p) => (
                                <option key={p.id} value={p.id}>
                                    {p.name} ({p.host}:{p.port})
                                </option>
                            ))}
                        </Select>
                        <p className="mt-2 text-xs text-slate-500">This profile will be used to dispatch the phishing email payloads.</p>
                    </div>
                </div>
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
                <div className="p-6">
                    <div className="flex items-center gap-3 mb-6">
                        <Globe className="w-5 h-5 text-emerald-400" />
                        <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">Endpoint Config</h2>
                    </div>

                    <div className="max-w-md space-y-6">
                        <div>
                            <label className="block text-sm font-medium text-slate-400 mb-2">Phishing Domain</label>
                            {isLoadingDomains ? (
                                <div className="text-slate-500 text-sm py-2">Loading configured domains...</div>
                            ) : (
                                <Select 
                                    value={infrastructure.domain}
                                    onChange={(e) => updateInfrastructure({ domain: e.target.value })}
                                    className="w-full bg-slate-950 border border-slate-800 rounded-md px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
                                >
                                    <option value="">Select a registered domain...</option>
                                    {domains.map((domain) => (
                                        <option key={domain.id} value={domain.domain_name}>
                                            {domain.domain_name}
                                        </option>
                                    ))}
                                </Select>
                            )}
                            <p className="mt-2 text-xs text-slate-500">Must be pre-registered via Tackle</p>
                        </div>

                        <div className="flex items-start gap-3 bg-slate-950/50 p-4 rounded-lg border border-slate-800">
                            <ShieldCheck className="w-5 h-5 text-emerald-500 shrink-0 mt-0.5" />
                            <div>
                                <h3 className="text-sm font-medium text-slate-300">Auto TLS Provisioning</h3>
                                <p className="text-xs text-slate-500 mt-1">We will automatically provision SSL certs for this domain via Let's Encrypt upon launch.</p>
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            <section className="border border-dashed border-slate-800 rounded-xl overflow-hidden flex items-center justify-center py-12 px-6">
                <div className="text-center max-w-md">
                    <Server className="w-8 h-8 text-slate-600 mx-auto mb-4" />
                    <h3 className="text-sm font-semibold text-slate-400 mb-2">Offline</h3>
                    <p className="text-xs text-slate-500 leading-relaxed">
                        Server telemetry and status indicators will appear here once the campaign builds and dynamically deploys backend targets.
                    </p>
                </div>
            </section>

            <div className="flex justify-end pt-4">
                <PermissionGate permission="campaigns:write">
                    <Button variant="outline" 
                        onClick={ () => saveCampaign(id)}
                        disabled={isSaving}
                        className="bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white px-6 py-2.5 rounded-md font-medium transition-colors"
                    >
                        {isSaving ? 'Saving...' : 'Save Infrastructure'}
                    </Button>
                </PermissionGate>
            </div>
        </div>
    );
}
