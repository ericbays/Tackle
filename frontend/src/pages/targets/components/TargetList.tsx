import { useEffect, useState, useRef } from 'react';
import { useTargetStore, type TargetDTO } from '../../../store/targetStore';
import { Download, Upload, Trash2, Search, Loader2, FileDown, Edit, X, Save } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Card } from '../../../components/ui/Card';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '../../../components/ui/Table';

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
            <Card className="flex p-4 gap-4 items-end shadow-xl">
                <div className="flex-1 space-y-1.5">
                    <label className="text-xs font-semibold text-slate-400">Search Email</label>
                    <div className="relative">
                        <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 z-10" />
                        <Input 
                            type="text" 
                            
                            placeholder="user@company.com..."
                            value={filters.email || ''}
                            onChange={(e) => setFilters({ email: e.target.value })}
                        />
                    </div>
                </div>
                <div className="flex-1 space-y-1.5">
                    <label className="text-xs font-semibold text-slate-400">Department</label>
                    <Input 
                        type="text" 
                        placeholder="IT, HR..."
                        value={filters.department || ''}
                        onChange={(e) => setFilters({ department: e.target.value })}
                    />
                </div>
                <div className="flex gap-2">
                    {selectedIds.size > 0 && (
                        <Button 
                            onClick={handleBulkDelete}
                            variant="destructive"
                            
                         variant="outline">
                            <Trash2 className="w-4 h-4" /> Delete ({selectedIds.size})
                        </Button>
                    )}
                    <Button 
                        onClick={handleExport}
                        variant="secondary"
                        
                     variant="outline">
                        <Download className="w-4 h-4" /> 
                        {selectedIds.size > 0 ? `Export (${selectedIds.size})` : 'Export All'}
                    </Button>
                    
                    <Button 
                        onClick={handleDownloadTemplate}
                        variant="secondary"
                        title="Download CSV Template"
                        
                     variant="outline">
                        <FileDown className="w-4 h-4" /> 
                    </Button>

                    <div className="relative">
                        <Input
                            type="file"
                            accept=".csv"
                            
                            ref={fileInputRef}
                            onChange={handleFileUpload}
                        />
                        <Button variant="outline" 
                            onClick={ () => fileInputRef.current?.click()}
                            variant="primary"
                            className="flex items-center gap-2"
                        >
                            <Upload className="w-4 h-4" /> Import CSV
                        </Button>
                    </div>
                </div>
            </Card>

            <Card className="overflow-hidden shadow-2xl">
                <div className="overflow-x-auto">
                    <Table className="border-0 shadow-none rounded-none text-left text-sm text-slate-300 w-full">
                        <TableHeader className="bg-slate-800/50">
                            <TableRow className="hover:bg-transparent text-xs uppercase font-semibold text-slate-400 border-b border-slate-800">
                                <TableHead className="w-12">
                                    <input 
                                        type="checkbox" 
                                        className="rounded border-slate-600 bg-slate-800/50 text-blue-500 focus:ring-blue-500 focus:ring-offset-slate-900" 
                                        checked={targets.length > 0 && selectedIds.size === targets.length}
                                        onChange={handleSelectAll}
                                    />
                                </TableHead>
                                <TableHead>Contact</TableHead>
                                <TableHead>Department &amp; Title</TableHead>
                                <TableHead>Added</TableHead>
                                <TableHead className="text-right">Actions</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {isLoading ? (
                                <TableRow>
                                    <TableCell colSpan={5} className="py-12 text-center text-slate-500">
                                        <Loader2 className="w-8 h-8 animate-spin mx-auto text-blue-500 mb-2" />
                                        Fetching target profiles...
                                    </TableCell>
                                </TableRow>
                            ) : targets.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={5} className="py-12 text-center text-slate-500">No targets found. Add a target or import a CSV.</TableCell>
                                </TableRow>
                            ) : (
                                targets.map((t) => (
                                    <TableRow key={t.id}>
                                        <TableCell>
                                            <input 
                                                type="checkbox" 
                                                className="rounded border-slate-600 bg-slate-800/50 text-blue-500 focus:ring-blue-500 focus:ring-offset-slate-900" 
                                                checked={selectedIds.has(t.id)}
                                                onChange={() => handleSelectOne(t.id)}
                                            />
                                        </TableCell>
                                        <TableCell>
                                            <div className="font-medium text-slate-200">{t.first_name} {t.last_name}</div>
                                            <div className="text-slate-500 text-xs mt-0.5">{t.email}</div>
                                        </TableCell>
                                        <TableCell>
                                            <div className="text-slate-200">{t.department || <span className="text-slate-600 italic">Unspecified</span>}</div>
                                            <div className="text-slate-500 text-xs mt-0.5">{t.title}</div>
                                        </TableCell>
                                        <TableCell>
                                            <div className="text-slate-400 text-xs">{new Date(t.created_at).toLocaleDateString()}</div>
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <Button variant="outline" 
                                                variant="ghost"
                                                size="icon"
                                                onClick={ () => setEditingTarget(t)}
                                                title="Edit Target"
                                            >
                                                <Edit className="w-4 h-4" />
                                            </Button>
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </div>
                
                {/* Pagination Footer */}
                {targets.length > 0 && (
                    <div className="px-6 py-4 border-t border-slate-800 flex items-center justify-between">
                        <div className="text-xs text-slate-400">
                            Page {filters.page} of {totalPages} ({total} targets)
                        </div>
                        <div className="flex gap-2">
                            <Button variant="outline"
                                variant="secondary"
                                disabled={filters.page === 1}
                                onClick={ () => setFilters({ page: (filters.page || 1) - 1 })}
                            >
                                Previous
                            </Button>
                            <Button variant="outline"
                                variant="secondary"
                                disabled={filters.page === totalPages}
                                onClick={ () => setFilters({ page: (filters.page || 1) + 1 })}
                            >
                                Next
                            </Button>
                        </div>
                    </div>
                )}
            </Card>

            {/* Edit Target Modal */}
            {editingTarget && (
                <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 z-[100]">
                    <Card className="w-full max-w-md shadow-2xl p-0">
                        <div className="flex justify-between items-center p-6 border-b border-slate-800">
                            <h2 className="text-xl font-semibold text-white">Edit Target Record</h2>
                            <Button variant="outline" onClick={ () => setEditingTarget(null)} className="text-slate-500 hover:text-slate-300">
                                <X className="w-5 h-5" />
                            </Button>
                        </div>
                        <form onSubmit={handleSaveEdit} className="p-6 space-y-4">
                            <div>
                                <label className="block text-xs font-semibold text-slate-400 mb-1.5">Email *</label>
                                <Input 
                                    type="email" 
                                    required 
                                    value={editingTarget.email}
                                    onChange={e => setEditingTarget({...editingTarget, email: e.target.value})}
                                />
                            </div>
                            <div className="flex gap-4">
                                <div className="flex-1">
                                    <label className="block text-xs font-semibold text-slate-400 mb-1.5">First Name</label>
                                    <Input 
                                        type="text" 
                                        value={editingTarget.first_name || ''}
                                        onChange={e => setEditingTarget({...editingTarget, first_name: e.target.value})}
                                    />
                                </div>
                                <div className="flex-1">
                                    <label className="block text-xs font-semibold text-slate-400 mb-1.5">Last Name</label>
                                    <Input 
                                        type="text" 
                                        value={editingTarget.last_name || ''}
                                        onChange={e => setEditingTarget({...editingTarget, last_name: e.target.value})}
                                    />
                                </div>
                            </div>
                            <div className="flex gap-4">
                                <div className="flex-1">
                                    <label className="block text-xs font-semibold text-slate-400 mb-1.5">Department</label>
                                    <Input 
                                        type="text" 
                                        value={editingTarget.department || ''}
                                        onChange={e => setEditingTarget({...editingTarget, department: e.target.value})}
                                    />
                                </div>
                                <div className="flex-1">
                                    <label className="block text-xs font-semibold text-slate-400 mb-1.5">Title</label>
                                    <Input 
                                        type="text" 
                                        value={editingTarget.title || ''}
                                        onChange={e => setEditingTarget({...editingTarget, title: e.target.value})}
                                    />
                                </div>
                            </div>

                            <div className="pt-4 flex justify-end gap-3 border-t border-slate-800 mt-6">
                                <Button variant="outline" variant="ghost" type="button" onClick={ () => setEditingTarget(null)}>
                                    Cancel
                                </Button>
                                <Button variant="primary" type="submit"  variant="outline">
                                    <Save className="w-4 h-4" /> Save Record
                                </Button>
                            </div>
                        </form>
                    </Card>
                </div>
            )}
        </div>
    );
}
