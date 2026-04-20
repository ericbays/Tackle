import React, { useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useSMTPStore } from '../../store/smtpStore';
import { Network, Server, ShieldCheck, Activity, AlertCircle, Plus, Copy, Trash2, Edit } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Badge } from '../../components/ui/Badge';
import { Card } from '../../components/ui/Card';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '../../components/ui/Table';

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

    const getStatusVariant = (status: string) => {
        switch (status) {
            case 'healthy': return 'success';
            case 'error': return 'destructive';
            case 'untested': return 'warning';
            default: return 'outline';
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
                <Button
                    onClick={() => window.location.href = '/smtp-profiles/new'}
                    variant="primary"
                    className="flex items-center gap-2"
                >
                    <Plus className="w-4 h-4" />
                    New Profile
                </Button>
            </div>

            <Card className="overflow-hidden shadow-2xl">
                <div className="overflow-x-auto">
                    <Table className="border-0 shadow-none rounded-none w-full text-left text-sm text-slate-300">
                        <TableHeader className="bg-slate-800/50">
                            <TableRow className="hover:bg-transparent text-xs uppercase font-semibold text-slate-400 border-b border-slate-800">
                                <TableHead>Profile Name</TableHead>
                                <TableHead>Relay Destination</TableHead>
                                <TableHead>Authentication</TableHead>
                                <TableHead>Status</TableHead>
                                <TableHead className="text-right">Actions</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {isLoading && (!profiles || profiles.length === 0) ? (
                                <TableRow>
                                    <TableCell colSpan={5} className="py-12 text-center text-slate-500">Loading profiles...</TableCell>
                                </TableRow>
                            ) : profiles.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={5} className="py-12 text-center text-slate-500">No SMTP profiles configured. Create one to get started.</TableCell>
                                </TableRow>
                            ) : (
                                Array.isArray(profiles) && profiles.map((p) => (
                                    <TableRow key={p.id} className="group">
                                        <TableCell className="font-medium text-slate-200">
                                            {p.name}
                                            <div className="text-xs text-slate-500 font-normal mt-1">{p.from_address}</div>
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex items-center gap-2">
                                                <Server className="w-4 h-4 text-slate-500" />
                                                <span className="font-mono text-xs">{p.host}:{p.port}</span>
                                            </div>
                                            <div className="text-[10px] text-slate-500 mt-1 uppercase tracking-wider">{p.tls_mode}</div>
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex items-center gap-2">
                                                {p.has_username ? (
                                                    <ShieldCheck className="w-4 h-4 text-emerald-500" />
                                                ) : (
                                                    <AlertCircle className="w-4 h-4 text-amber-500" />
                                                )}
                                                <span>{p.auth_type}</span>
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <Badge variant={getStatusVariant(p.status || '') as any} className="gap-1.5 uppercase font-semibold">
                                                {p.status === 'error' && <AlertCircle className="w-3.5 h-3.5" />}
                                                {p.status === 'healthy' && <Activity className="w-3.5 h-3.5" />}
                                                {p.status ? p.status : 'UNKNOWN'}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <div className="flex items-center justify-end opacity-0 group-hover:opacity-100 transition-opacity pr-2 gap-1">
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    onClick={() => handleTest(p.id)}
                                                    disabled={isTesting}
                                                    title="Test connection"
                                                    className="text-blue-400 hover:text-blue-300 hover:bg-blue-400/10"
                                                >
                                                    <Activity className="w-4 h-4" />
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    onClick={() => handleDuplicate(p.id, p.name)}
                                                    title="Duplicate profile"
                                                >
                                                    <Copy className="w-4 h-4" />
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    onClick={() => window.location.href = `/smtp-profiles/${p.id}`}
                                                    title="Edit profile"
                                                >
                                                    <Edit className="w-4 h-4" />
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    onClick={() => handleDelete(p.id, p.name)}
                                                    title="Delete profile"
                                                    className="text-red-400 hover:text-red-300 hover:bg-red-400/10"
                                                >
                                                    <Trash2 className="w-4 h-4" />
                                                </Button>
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </div>
            </Card>
        </div>
    );
}
