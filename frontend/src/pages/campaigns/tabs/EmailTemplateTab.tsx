import { Plus, X } from 'lucide-react';
import { Link, useParams } from 'react-router-dom';
import { useCampaignStore } from '../../../store/campaignStore';
import { useEmailTemplates, useSmtpProfiles } from '../../../hooks/useConfigurations';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Button } from '../../../components/ui/Button';

export default function EmailTemplateTab() {
    const { id } = useParams();
    const { draft: { emailVariants: variants }, updateEmailVariants, isSaving, saveCampaign } = useCampaignStore();

    const { data: emailTemplates = [] } = useEmailTemplates();
    const { data: smtpProfiles = [], isLoading: isLoadingSmtp } = useSmtpProfiles();

    return (
        <div className="space-y-8">
            <section className="space-y-4">
                <div className="flex items-center justify-between">
                    <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">Email Template Variants</h2>
                    <Button  variant="outline">
                        <Plus className="w-4 h-4" /> Add Variant
                    </Button>
                </div>

                <div className="space-y-4">
                    {variants.map(v => (
                        <div key={v.id} className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
                            <div className="bg-slate-950/50 border-b border-slate-800 p-4 flex items-center justify-between">
                                <div className="flex items-center gap-4">
                                    <Input 
                                        type="text" 
                                         
                                        value={v.label}
                                        onChange={(e) => updateEmailVariants(variants.map(variant => variant.id === v.id ? { ...variant, label: e.target.value } : variant))}
                                    />
                                    <span className="text-slate-500 font-medium">{v.percentage}% traffic</span>
                                </div>
                                <Button variant="outline" 
                                    onClick={ () => updateEmailVariants(variants.filter(variant => variant.id !== v.id))}
                                    className="text-slate-600 hover:text-red-400 transition-colors"
                                >
                                    <X className="w-4 h-4" />
                                </Button>
                            </div>
                            <div className="p-6">
                                <div className="mb-4">
                                    <label className="block text-sm font-medium text-slate-400 mb-2">Selected Template</label>
                                    <Select 
                                        value={v.template || ''}
                                        onChange={(e) => updateEmailVariants(variants.map(variant => variant.id === v.id ? { ...variant, template: e.target.value } : variant))}
                                        className="w-full bg-slate-950 border border-slate-800 rounded-md px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
                                    >
                                        <option value="">Select email template...</option>
                                        {emailTemplates.map(t => (
                                            <option key={t.id} value={t.id}>{t.name}</option>
                                        ))}
                                    </Select>
                                    <div className="mt-2 text-xs">
                                        <Link to="/email-templates/new" className="text-blue-500 hover:text-blue-400" target="_blank">Create New Template →</Link>
                                    </div>
                                </div>
                                
                                {v.template ? (
                                    <div className="border border-slate-800 rounded-lg bg-slate-950/30 p-4">
                                        <div className="text-sm border-b border-slate-800 pb-3 mb-3">
                                            <div className="grid grid-cols-[60px_1fr] gap-2">
                                                <span className="text-slate-500">From:</span>
                                                <span className="text-slate-300">it-support@admin-portal-updates.com</span>
                                                <span className="text-slate-500">Subject:</span>
                                                <span className="text-slate-200 font-medium">Action Required: Verify Account Access</span>
                                            </div>
                                        </div>
                                        <p className="text-sm text-slate-400 italic">Dear {'{'}first_name{'}'}, we detected a login from an unrecognized device...</p>
                                    </div>
                                ) : (
                                    <div className="border border-dashed border-slate-800 rounded-lg p-6 flex items-center justify-center text-slate-500 text-sm">
                                        Select a template to view preview
                                    </div>
                                )}
                            </div>
                        </div>
                    ))}
                </div>

                <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 mt-6">
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-widest mb-3">Traffic Split</h3>
                    <div className="flex h-8 rounded-md overflow-hidden relative border border-slate-700">
                        {variants.map((v, idx) => (
                            <div 
                                key={v.id} 
                                className={`h-full flex items-center justify-center text-xs font-medium ${idx === 0 ? 'bg-blue-600/80 text-white border-r border-slate-800' : 'bg-slate-700/50 text-slate-300'}`}
                                style={{ width: `${v.percentage}%` }}
                            >
                                {v.label} ({v.percentage}%)
                            </div>
                        ))}
                    </div>
                </div>
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden p-6">
                 <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500 mb-4">SMTP Profile</h2>
                 <p className="text-sm text-slate-400 mb-4">Select the outgoing mail server configuration. The sending address will be taken from the email template.</p>
                 {isLoadingSmtp ? (
                     <div className="text-slate-500 text-sm py-2">Loading configured SMTP profiles...</div>
                 ) : (
                     <Select >
                         <option value="">Select an SMTP Configuration...</option>
                         {smtpProfiles.map(profile => (
                             <option key={profile.id} value={profile.id}>{profile.name}</option>
                         ))}
                     </Select>
                 )}
            </section>

            <div className="flex justify-end pt-4">
                <Button variant="outline" 
                    onClick={ () => saveCampaign(id)}
                    disabled={isSaving}
                    className="bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white px-6 py-2.5 rounded-md font-medium transition-colors"
                >
                    {isSaving ? 'Saving...' : 'Save Email Configurations'}
                </Button>
            </div>
        </div>
    );
}
