import React, { useState, useEffect } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useSMTPStore } from '../../store/smtpStore';
import { Network, Server, ShieldCheck, Mail, Sliders, Save, ArrowLeft, Loader2 } from 'lucide-react';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Button } from '../../components/ui/Button';

export default function SMTPProfileEditor() {
    const { id } = useParams();
    const navigate = useNavigate();
    const { getProfile, createProfile, updateProfile } = useSMTPStore();
    const isNew = id === 'new';

    const [isLoading, setIsLoading] = useState(!isNew);
    const [isSaving, setIsSaving] = useState(false);

    const [formData, setFormData] = useState({
        name: '',
        description: '',
        host: '',
        port: 587,
        auth_type: 'login',
        username: '',
        password: '',
        tls_mode: 'starttls',
        tls_skip_verify: false,
        from_address: '',
        from_name: '',
        reply_to: '',
        custom_helo: '',
        max_send_rate: 0,
        max_connections: 5,
        timeout_connect: 30,
        timeout_send: 60
    });

    useEffect(() => {
        const load = async () => {
            if (!isNew && id) {
                const p = await getProfile(id);
                if (p) {
                    setFormData({
                        name: p.name,
                        description: p.description || '',
                        host: p.host,
                        port: p.port,
                        auth_type: p.auth_type,
                        username: '', // keep empty to preserve existing
                        password: '', // keep empty to preserve existing
                        tls_mode: p.tls_mode,
                        tls_skip_verify: p.tls_skip_verify,
                        from_address: p.from_address,
                        from_name: p.from_name || '',
                        reply_to: p.reply_to || '',
                        custom_helo: p.custom_helo || '',
                        max_send_rate: p.max_send_rate || 0,
                        max_connections: p.max_connections,
                        timeout_connect: p.timeout_connect,
                        timeout_send: p.timeout_send
                    });
                } else {
                    navigate('/smtp-profiles');
                }
            }
            setIsLoading(false);
        };
        load();
    }, [id, isNew, getProfile, navigate]);

    const handleSave = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsSaving(true);
        
        const payload: any = { ...formData };
        
        // Clean up empty optional fields
        if (payload.max_send_rate === 0) payload.max_send_rate = null;
        if (!payload.description) delete payload.description;
        if (!payload.from_name) delete payload.from_name;
        if (!payload.reply_to) delete payload.reply_to;
        if (!payload.custom_helo) delete payload.custom_helo;

        // If editing and password/username are empty, do not send them to preserve
        if (!isNew) {
            if (!payload.username) delete payload.username;
            if (!payload.password) delete payload.password;
        }

        let successProp = null;
        if (isNew) {
            successProp = await createProfile(payload);
        } else if (id) {
            successProp = await updateProfile(id, payload);
        }

        if (successProp) {
            navigate('/smtp-profiles');
        } else {
            setIsSaving(false);
        }
    };

    if (isLoading) {
        return <div className="flex justify-center py-20"><Loader2 className="w-8 h-8 animate-spin text-blue-500" /></div>;
    }

    return (
        <div className="max-w-4xl mx-auto space-y-6 pb-20">
            <div className="flex items-center justify-between">
                <div>
                    <Link to="/smtp-profiles" className="text-slate-400 hover:text-white flex items-center gap-2 mb-2 transition-colors text-sm font-medium">
                        <ArrowLeft className="w-4 h-4" /> Back to Profiles
                    </Link>
                    <h1 className="text-3xl font-bold tracking-tight text-white flex items-center gap-3">
                        <Network className="w-8 h-8 text-blue-500" />
                        {isNew ? 'New SMTP Profile' : 'Edit SMTP Profile'}
                    </h1>
                </div>
                <Button
                    onClick={handleSave}
                    disabled={isSaving}
                    
                 variant="primary">
                    {isSaving ? <Loader2 className="w-5 h-5 animate-spin" /> : <Save className="w-5 h-5" />}
                    Save Profile
                </Button>
            </div>

            <form onSubmit={handleSave} className="space-y-6">
                {/* General Settings */}
                <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
                    <div className="px-6 py-4 border-b border-slate-800 bg-slate-800/20 flex items-center gap-3">
                        <Server className="w-5 h-5 text-indigo-400" />
                        <h2 className="text-lg font-semibold text-slate-200">Connection Settings</h2>
                    </div>
                    <div className="p-6 grid grid-cols-2 gap-6">
                        <div className="col-span-2">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Profile Name</label>
                            <Input
                                type="text"
                                required
                                value={formData.name}
                                onChange={e => setFormData({ ...formData, name: e.target.value })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                                placeholder="e.g. Primary GSuite Relay"
                            />
                        </div>
                        <div className="col-span-2 sm:col-span-1">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Hostname</label>
                            <Input
                                type="text"
                                required
                                value={formData.host}
                                onChange={e => setFormData({ ...formData, host: e.target.value })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white font-mono focus:outline-none focus:border-blue-500"
                                placeholder="smtp.gmail.com"
                            />
                        </div>
                        <div className="col-span-2 sm:col-span-1">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Port</label>
                            <Input
                                type="number"
                                required
                                value={formData.port}
                                onChange={e => setFormData({ ...formData, port: parseInt(e.target.value) || 0 })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white font-mono focus:outline-none focus:border-blue-500"
                            />
                        </div>
                        <div className="col-span-2 sm:col-span-1">
                            <label className="block text-sm font-medium text-slate-400 mb-1">TLS Mode</label>
                            <Select
                                value={formData.tls_mode}
                                onChange={e => setFormData({ ...formData, tls_mode: e.target.value })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                            >
                                <option value="none">None</option>
                                <option value="starttls">STARTTLS (Usually Port 587)</option>
                                <option value="tls">Implicit TLS (Usually Port 465)</option>
                            </Select>
                        </div>
                        <div className="col-span-2 sm:col-span-1 flex items-center pt-6">
                            <label className="flex items-center gap-3 cursor-pointer">
                                <input
                                    type="checkbox"
                                    checked={formData.tls_skip_verify}
                                    onChange={e => setFormData({ ...formData, tls_skip_verify: e.target.checked })}
                                    className="w-5 h-5 rounded border-slate-700 bg-slate-800/50 text-blue-500 focus:ring-blue-500 focus:ring-offset-slate-900"
                                />
                                <span className="text-sm font-medium text-slate-300">Allow Self-Signed / Invalid Certificates</span>
                            </label>
                        </div>
                    </div>
                </div>

                {/* Authentication */}
                <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
                    <div className="px-6 py-4 border-b border-slate-800 bg-slate-800/20 flex items-center gap-3">
                        <ShieldCheck className="w-5 h-5 text-emerald-400" />
                        <h2 className="text-lg font-semibold text-slate-200">Authentication</h2>
                    </div>
                    <div className="p-6 grid grid-cols-2 gap-6">
                        <div className="col-span-2">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Auth Type</label>
                            <Select
                                value={formData.auth_type}
                                onChange={e => setFormData({ ...formData, auth_type: e.target.value })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                            >
                                <option value="none">None (IP Allowlisted)</option>
                                <option value="login">LOGIN</option>
                                <option value="plain">PLAIN</option>
                                <option value="cram_md5">CRAM-MD5</option>
                            </Select>
                        </div>
                        {formData.auth_type !== 'none' && (
                            <>
                                <div className="col-span-2 sm:col-span-1">
                                    <label className="block text-sm font-medium text-slate-400 mb-1">Username</label>
                                    <Input
                                        type="text"
                                        required={isNew}
                                        value={formData.username}
                                        onChange={e => setFormData({ ...formData, username: e.target.value })}
                                        className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                                        placeholder={!isNew ? "Leave blank to keep existing" : ""}
                                    />
                                </div>
                                <div className="col-span-2 sm:col-span-1">
                                    <label className="block text-sm font-medium text-slate-400 mb-1">Password / App Password</label>
                                    <Input
                                        type="password"
                                        required={isNew}
                                        value={formData.password}
                                        onChange={e => setFormData({ ...formData, password: e.target.value })}
                                        className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                                        placeholder={!isNew ? "Leave blank to keep existing" : ""}
                                    />
                                </div>
                            </>
                        )}
                    </div>
                </div>

                {/* Identity */}
                <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
                    <div className="px-6 py-4 border-b border-slate-800 bg-slate-800/20 flex items-center gap-3">
                        <Mail className="w-5 h-5 text-amber-400" />
                        <h2 className="text-lg font-semibold text-slate-200">Identity Details</h2>
                    </div>
                    <div className="p-6 grid grid-cols-2 gap-6">
                        <div className="col-span-2">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Global From Address</label>
                            <Input
                                type="email"
                                required
                                value={formData.from_address}
                                onChange={e => setFormData({ ...formData, from_address: e.target.value })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                                placeholder="security@example.com"
                            />
                            <p className="text-xs text-slate-500 mt-1">This can be dynamically overridden per campaign if the relay permits it.</p>
                        </div>
                        <div className="col-span-2 sm:col-span-1">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Default From Name</label>
                            <Input
                                type="text"
                                value={formData.from_name}
                                onChange={e => setFormData({ ...formData, from_name: e.target.value })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                                placeholder="IT Service Desk"
                            />
                        </div>
                        <div className="col-span-2 sm:col-span-1">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Default Reply-To</label>
                            <Input
                                type="email"
                                value={formData.reply_to}
                                onChange={e => setFormData({ ...formData, reply_to: e.target.value })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                                placeholder="noreply@example.com"
                            />
                        </div>
                        <div className="col-span-2">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Custom HELO Domain (Optional)</label>
                            <Input
                                type="text"
                                value={formData.custom_helo}
                                onChange={e => setFormData({ ...formData, custom_helo: e.target.value })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white font-mono focus:outline-none focus:border-blue-500"
                                placeholder="mail.example.com"
                            />
                        </div>
                    </div>
                </div>

                {/* Operations Limits */}
                <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
                    <div className="px-6 py-4 border-b border-slate-800 bg-slate-800/20 flex items-center gap-3">
                        <Sliders className="w-5 h-5 text-rose-400" />
                        <h2 className="text-lg font-semibold text-slate-200">Operations Limits</h2>
                    </div>
                    <div className="p-6 grid grid-cols-2 gap-6">
                        <div className="col-span-2 sm:col-span-1">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Max Concurrent Connections</label>
                            <Input
                                type="number"
                                required
                                value={formData.max_connections}
                                onChange={e => setFormData({ ...formData, max_connections: parseInt(e.target.value) || 0 })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                            />
                        </div>
                        <div className="col-span-2 sm:col-span-1">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Global Rate Limit (Emails / Hour)</label>
                            <Input
                                type="number"
                                value={formData.max_send_rate || ''}
                                onChange={e => setFormData({ ...formData, max_send_rate: parseInt(e.target.value) || 0 })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                                placeholder="0 = Unlimited"
                            />
                        </div>
                        <div className="col-span-2 sm:col-span-1">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Connection Timeout (Seconds)</label>
                            <Input
                                type="number"
                                required
                                value={formData.timeout_connect}
                                onChange={e => setFormData({ ...formData, timeout_connect: parseInt(e.target.value) || 0 })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                            />
                        </div>
                        <div className="col-span-2 sm:col-span-1">
                            <label className="block text-sm font-medium text-slate-400 mb-1">Dial/Send Timeout (Seconds)</label>
                            <Input
                                type="number"
                                required
                                value={formData.timeout_send}
                                onChange={e => setFormData({ ...formData, timeout_send: parseInt(e.target.value) || 0 })}
                                className="w-full bg-slate-800/50 border border-slate-700 rounded-md px-4 py-2 text-white focus:outline-none focus:border-blue-500"
                            />
                        </div>
                    </div>
                </div>

            </form>
        </div>
    );
}
