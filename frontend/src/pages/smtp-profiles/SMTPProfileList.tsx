import React, { useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useSMTPStore } from '../../store/smtpStore';
import { Network, Server, ShieldCheck, Activity, AlertCircle, Plus, Copy, Trash2, Edit } from 'lucide-react';

export default function SMTPProfileList() {
    const { profiles, fetchProfiles, isLoading, deleteProfile, duplicateProfile, testProfile, isTesting } = useSMTPStore();

    useEffect(() => {
        fetchProfiles();
    }, [fetchProfiles]);

    const handleDuplicate = async (id: string, name: string) => {
        const newName = prompt('Enter a name for the duplicated profile:', `${name} (Copy)`);
        if (newName) {
            await duplicateProfile(id, newName);
        }
    };

    const handleDelete = async (id: string, name: string) => {
        if (window.confirm(`Are you sure you want to delete SMTP Profile "${name}"? This will fail if it is assigned to any campaigns.`)) {
            await deleteProfile(id);
        }
    };

    const handleTest = async (id: string) => {
        const result = await testProfile(id);
        if (result && !result.success) {
            // Toast automatically handled in store, but we can provide extra UI if needed
        }
    };

    const getStatusColor = (status: string) => {
        switch (status) {
            case 'healthy': return 'text-emerald-400 bg-emerald-400/10 border-emerald-500/20';
            case 'error': return 'text-red-400 bg-red-400/10 border-red-500/20';
            case 'untested': return 'text-amber-400 bg-amber-400/10 border-amber-500/20';
            default: return 'text-slate-400 bg-slate-800 border-slate-700';
        }
    };

    return (
        <div className="max-w-7xl mx-auto space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight text-white flex items-center gap-3">
                        <Network className="w-8 h-8 text-blue-500" />
                        SMTP Profiles
                    </h1>
                    <p className="text-slate-400 mt-2">Manage outbound email relay configurations globally available for campaign assignment.</p>
                </div>
                <Link
                    to="/smtp-profiles/new"
                    className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-medium rounded-lg transition-all shadow-lg shadow-blue-500/20"
                >
                    <Plus className="w-4 h-4" />
                    New Profile
                </Link>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-2xl">
                <div className="overflow-x-auto">
                    <table className="w-full text-left text-sm text-slate-300">
                        <thead className="bg-slate-800/50 text-xs uppercase font-semibold text-slate-400 border-b border-slate-800">
                            <tr>
                                <th className="px-6 py-4">Profile Name</th>
                                <th className="px-6 py-4">Relay Destination</th>
                                <th className="px-6 py-4">Authentication</th>
                                <th className="px-6 py-4">Status</th>
                                <th className="px-6 py-4 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800/50">
                            {isLoading && (!profiles || profiles.length === 0) ? (
                                <tr>
                                    <td colSpan={5} className="px-6 py-12 text-center text-slate-500">Loading profiles...</td>
                                </tr>
                            ) : profiles.length === 0 ? (
                                <tr>
                                    <td colSpan={5} className="px-6 py-12 text-center text-slate-500">No SMTP profiles configured. Create one to get started.</td>
                                </tr>
                            ) : (
                                Array.isArray(profiles) && profiles.map((p) => (
                                    <tr key={p.id} className="hover:bg-slate-800/50 transition-colors group">
                                        <td className="px-6 py-4 font-medium text-slate-200">
                                            {p.name}
                                            <div className="text-xs text-slate-500 font-normal mt-1">{p.from_address}</div>
                                        </td>
                                        <td className="px-6 py-4">
                                            <div className="flex items-center gap-2">
                                                <Server className="w-4 h-4 text-slate-500" />
                                                <span className="font-mono text-xs">{p.host}:{p.port}</span>
                                            </div>
                                            <div className="text-[10px] text-slate-500 mt-1 uppercase tracking-wider">{p.tls_mode}</div>
                                        </td>
                                        <td className="px-6 py-4">
                                            <div className="flex items-center gap-2">
                                                {p.has_username ? (
                                                    <ShieldCheck className="w-4 h-4 text-emerald-500" />
                                                ) : (
                                                    <AlertCircle className="w-4 h-4 text-amber-500" />
                                                )}
                                                <span>{p.auth_type}</span>
                                            </div>
                                        </td>
                                        <td className="px-6 py-4">
                                            <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-semibold border ${getStatusColor(p.status || '')}`}>
                                                {p.status === 'error' && <AlertCircle className="w-3.5 h-3.5" />}
                                                {p.status === 'healthy' && <Activity className="w-3.5 h-3.5" />}
                                                {p.status ? p.status.toUpperCase() : 'UNKNOWN'}
                                            </span>
                                        </td>
                                        <td className="px-6 py-4 text-right">
                                            <div className="flex items-center justify-end gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                                                <button
                                                    onClick={() => handleTest(p.id)}
                                                    disabled={isTesting}
                                                    title="Test connection"
                                                    className="p-2 text-blue-400 hover:text-blue-300 hover:bg-blue-400/10 rounded-md transition-colors disabled:opacity-50"
                                                >
                                                    <Activity className="w-4 h-4" />
                                                </button>
                                                <button
                                                    onClick={() => handleDuplicate(p.id, p.name)}
                                                    title="Duplicate profile"
                                                    className="p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-md transition-colors"
                                                >
                                                    <Copy className="w-4 h-4" />
                                                </button>
                                                <Link
                                                    to={`/smtp-profiles/${p.id}`}
                                                    title="Edit profile"
                                                    className="p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-md transition-colors"
                                                >
                                                    <Edit className="w-4 h-4" />
                                                </Link>
                                                <button
                                                    onClick={() => handleDelete(p.id, p.name)}
                                                    title="Delete profile"
                                                    className="p-2 text-red-400 hover:text-red-300 hover:bg-red-400/10 rounded-md transition-colors"
                                                >
                                                    <Trash2 className="w-4 h-4" />
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
