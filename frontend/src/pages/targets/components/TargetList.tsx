import { useEffect, useState, useRef } from 'react';
import { useTargetStore, type TargetDTO } from '../../../store/targetStore';
import { Download, Upload, Trash2, Search, Loader2, FileDown, Edit, X, Save } from 'lucide-react';

export default function TargetList() {
    const { targets, isLoading, fetchTargets, filters, setFilters, total, totalPages, bulkDelete, bulkExport, uploadCSV, updateTarget } = useTargetStore();
    const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
    const [editingTarget, setEditingTarget] = useState<TargetDTO | null>(null);
    const fileInputRef = useRef<HTMLInputElement>(null);

    useEffect(() => {
        fetchTargets();
    }, [fetchTargets]);

    const handleSelectAll = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (e.target.checked) {
            setSelectedIds(new Set(targets.map(t => t.id)));
        } else {
            setSelectedIds(new Set());
        }
    };

    const handleSelectOne = (id: string) => {
        const next = new Set(selectedIds);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        setSelectedIds(next);
    };

    const handleBulkDelete = async () => {
        if (!confirm('Are you sure you want to delete the selected targets?')) return;
        const success = await bulkDelete(Array.from(selectedIds));
        if (success) {
            setSelectedIds(new Set());
        }
    };

    const handleExport = () => {
        bulkExport(selectedIds.size > 0 ? Array.from(selectedIds) : undefined);
    };

    const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (!file) return;

        const success = await uploadCSV(file);
        if (success) {
            // reset
            if (fileInputRef.current) fileInputRef.current.value = '';
        }
    };

    const handleDownloadTemplate = () => {
        const csvContent = "email,first_name,last_name,department,title\ntestuser@example.com,John,Doe,IT,Senior Engineer\n";
        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement("a");
        link.setAttribute("href", url);
        link.setAttribute("download", "tackle_target_import_template.csv");
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(url);
    };

    const handleSaveEdit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!editingTarget) return;

        const success = await updateTarget(editingTarget.id, {
            email: editingTarget.email,
            first_name: editingTarget.first_name,
            last_name: editingTarget.last_name,
            department: editingTarget.department,
            title: editingTarget.title
        });

        if (success) {
            setEditingTarget(null);
        }
    };

    return (
        <div className="space-y-6">
            <div className="flex bg-slate-900 border border-slate-800 rounded-xl p-4 gap-4 items-end">
                <div className="flex-1 space-y-1.5">
                    <label className="text-xs font-semibold text-slate-400">Search Email</label>
                    <div className="relative">
                        <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
                        <input 
                            type="text" 
                            className="w-full bg-slate-800/50 border border-slate-700/50 rounded-lg pl-9 pr-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
                            placeholder="user@company.com..."
                            value={filters.email}
                            onChange={(e) => setFilters({ email: e.target.value })}
                        />
                    </div>
                </div>
                <div className="flex-1 space-y-1.5">
                    <label className="text-xs font-semibold text-slate-400">Department</label>
                    <input 
                        type="text" 
                        className="w-full bg-slate-800/50 border border-slate-700/50 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
                        placeholder="IT, HR..."
                        value={filters.department || ''}
                        onChange={(e) => setFilters({ department: e.target.value })}
                    />
                </div>
                <div className="flex gap-2">
                    {selectedIds.size > 0 && (
                        <button 
                            onClick={handleBulkDelete}
                            className="bg-red-500/10 text-red-400 hover:bg-red-500/20 px-4 py-2 rounded-lg text-sm font-semibold transition-colors flex items-center gap-2"
                        >
                            <Trash2 className="w-4 h-4" /> Delete ({selectedIds.size})
                        </button>
                    )}
                    <button 
                        onClick={handleExport}
                        className="bg-slate-800 text-slate-300 hover:bg-slate-700 px-4 py-2 rounded-lg text-sm font-semibold transition-colors flex items-center gap-2"
                    >
                        <Download className="w-4 h-4" /> 
                        {selectedIds.size > 0 ? `Export (${selectedIds.size})` : 'Export All'}
                    </button>
                    
                    <button 
                        onClick={handleDownloadTemplate}
                        className="bg-slate-800 text-slate-300 hover:bg-slate-700 px-4 py-2 rounded-lg text-sm font-semibold transition-colors flex items-center gap-2"
                        title="Download CSV Template"
                    >
                        <FileDown className="w-4 h-4" /> 
                    </button>

                    <div className="relative">
                        <input
                            type="file"
                            accept=".csv"
                            className="hidden"
                            ref={fileInputRef}
                            onChange={handleFileUpload}
                        />
                        <button 
                            onClick={() => fileInputRef.current?.click()}
                            className="bg-blue-600 hover:bg-blue-500 text-white px-4 py-2 rounded-lg text-sm font-semibold transition-colors flex items-center gap-2"
                        >
                            <Upload className="w-4 h-4" /> Import CSV
                        </button>
                    </div>
                </div>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-2xl">
                <div className="overflow-x-auto">
                    <table className="w-full text-left text-sm text-slate-300">
                        <thead className="bg-slate-800/50 text-xs uppercase font-semibold text-slate-400 border-b border-slate-800">
                            <tr>
                                <th className="px-6 py-4 w-12">
                                    <input 
                                        type="checkbox" 
                                        className="rounded border-slate-600 bg-slate-800/50 text-blue-500 focus:ring-blue-500 focus:ring-offset-slate-900" 
                                        checked={targets.length > 0 && selectedIds.size === targets.length}
                                        onChange={handleSelectAll}
                                    />
                                </th>
                                <th className="px-6 py-4">Contact</th>
                                <th className="px-6 py-4">Department & Title</th>
                                <th className="px-6 py-4">Added</th>
                                <th className="px-6 py-4 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800/50">
                            {isLoading ? (
                                <tr>
                                    <td colSpan={4} className="px-6 py-12 text-center text-slate-500">
                                        <Loader2 className="w-8 h-8 animate-spin mx-auto text-blue-500 mb-2" />
                                        Fetching target profiles...
                                    </td>
                                </tr>
                            ) : targets.length === 0 ? (
                                <tr>
                                    <td colSpan={4} className="px-6 py-12 text-center text-slate-500">No targets found. Add a target or import a CSV.</td>
                                </tr>
                            ) : (
                                targets.map((t) => (
                                    <tr key={t.id} className="hover:bg-slate-800/50 transition-colors">
                                        <td className="px-6 py-4">
                                            <input 
                                                type="checkbox" 
                                                className="rounded border-slate-600 bg-slate-800/50 text-blue-500 focus:ring-blue-500 focus:ring-offset-slate-900" 
                                                checked={selectedIds.has(t.id)}
                                                onChange={() => handleSelectOne(t.id)}
                                            />
                                        </td>
                                        <td className="px-6 py-4">
                                            <div className="font-medium text-slate-200">{t.first_name} {t.last_name}</div>
                                            <div className="text-slate-500 text-xs mt-0.5">{t.email}</div>
                                        </td>
                                        <td className="px-6 py-4">
                                            <div className="text-slate-200">{t.department || <span className="text-slate-600 italic">Unspecified</span>}</div>
                                            <div className="text-slate-500 text-xs mt-0.5">{t.title}</div>
                                        </td>
                                        <td className="px-6 py-4">
                                            <div className="text-slate-400 text-xs">{new Date(t.created_at).toLocaleDateString()}</div>
                                        </td>
                                        <td className="px-6 py-4 text-right">
                                            <button 
                                                onClick={() => setEditingTarget(t)}
                                                className="text-slate-400 hover:text-blue-400 p-1 rounded hover:bg-slate-800 transition-colors"
                                                title="Edit Target"
                                            >
                                                <Edit className="w-4 h-4" />
                                            </button>
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
                
                {/* Pagination Footer */}
                {targets.length > 0 && (
                    <div className="px-6 py-4 border-t border-slate-800 flex items-center justify-between">
                        <div className="text-xs text-slate-400">
                            Page {filters.page} of {totalPages} ({total} targets)
                        </div>
                        <div className="flex gap-2">
                            <button
                                disabled={filters.page === 1}
                                onClick={() => setFilters({ page: (filters.page || 1) - 1 })}
                                className="px-3 py-1.5 text-xs font-semibold bg-slate-800 text-slate-300 rounded hover:bg-slate-700 disabled:opacity-50"
                            >
                                Previous
                            </button>
                            <button
                                disabled={filters.page === totalPages}
                                onClick={() => setFilters({ page: (filters.page || 1) + 1 })}
                                className="px-3 py-1.5 text-xs font-semibold bg-slate-800 text-slate-300 rounded hover:bg-slate-700 disabled:opacity-50"
                            >
                                Next
                            </button>
                        </div>
                    </div>
                )}
            </div>

            {/* Edit Target Modal */}
            {editingTarget && (
                <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 z-[100]">
                    <div className="bg-slate-900 border border-slate-800 w-full max-w-md rounded-xl shadow-2xl overflow-hidden">
                        <div className="flex justify-between items-center p-6 border-b border-slate-800">
                            <h2 className="text-xl font-semibold text-white">Edit Target Record</h2>
                            <button onClick={() => setEditingTarget(null)} className="text-slate-500 hover:text-slate-300">
                                <X className="w-5 h-5" />
                            </button>
                        </div>
                        <form onSubmit={handleSaveEdit} className="p-6 space-y-4">
                            <div>
                                <label className="block text-xs font-semibold text-slate-400 mb-1.5">Email *</label>
                                <input 
                                    type="email" 
                                    required 
                                    className="w-full bg-slate-800/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
                                    value={editingTarget.email}
                                    onChange={e => setEditingTarget({...editingTarget, email: e.target.value})}
                                />
                            </div>
                            <div className="flex gap-4">
                                <div className="flex-1">
                                    <label className="block text-xs font-semibold text-slate-400 mb-1.5">First Name</label>
                                    <input 
                                        type="text" 
                                        className="w-full bg-slate-800/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
                                        value={editingTarget.first_name || ''}
                                        onChange={e => setEditingTarget({...editingTarget, first_name: e.target.value})}
                                    />
                                </div>
                                <div className="flex-1">
                                    <label className="block text-xs font-semibold text-slate-400 mb-1.5">Last Name</label>
                                    <input 
                                        type="text" 
                                        className="w-full bg-slate-800/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
                                        value={editingTarget.last_name || ''}
                                        onChange={e => setEditingTarget({...editingTarget, last_name: e.target.value})}
                                    />
                                </div>
                            </div>
                            <div className="flex gap-4">
                                <div className="flex-1">
                                    <label className="block text-xs font-semibold text-slate-400 mb-1.5">Department</label>
                                    <input 
                                        type="text" 
                                        className="w-full bg-slate-800/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
                                        value={editingTarget.department || ''}
                                        onChange={e => setEditingTarget({...editingTarget, department: e.target.value})}
                                    />
                                </div>
                                <div className="flex-1">
                                    <label className="block text-xs font-semibold text-slate-400 mb-1.5">Title</label>
                                    <input 
                                        type="text" 
                                        className="w-full bg-slate-800/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
                                        value={editingTarget.title || ''}
                                        onChange={e => setEditingTarget({...editingTarget, title: e.target.value})}
                                    />
                                </div>
                            </div>

                            <div className="pt-4 flex justify-end gap-3 border-t border-slate-800 mt-6">
                                <button type="button" onClick={() => setEditingTarget(null)} className="px-4 py-2 text-sm font-semibold text-slate-400 hover:text-white">
                                    Cancel
                                </button>
                                <button type="submit" className="bg-blue-600 hover:bg-blue-500 px-6 py-2 rounded-lg text-sm font-semibold text-white flex items-center gap-2 transition-colors">
                                    <Save className="w-4 h-4" /> Save Record
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}
        </div>
    );
}
