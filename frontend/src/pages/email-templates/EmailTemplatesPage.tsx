import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Search, MoreVertical, Trash2, Edit2, Copy } from 'lucide-react';
import { useEmailTemplates, useDeleteTemplate, useDuplicateTemplate } from '../../hooks/useEmailTemplates';
import type { EmailTemplate } from '../../hooks/useEmailTemplates';

export default function EmailTemplatesPage() {
    const navigate = useNavigate();
    const [search, setSearch] = useState('');

    const { data: templatesRes, isLoading } = useEmailTemplates();
    const templates = templatesRes?.items || [];
    
    const deleteMutation = useDeleteTemplate();
    const duplicateMutation = useDuplicateTemplate();

    const filteredTemplates = useMemo(() => {
        return templates.filter((t: EmailTemplate) => 
            t.name.toLowerCase().includes(search.toLowerCase()) || 
            t.subject.toLowerCase().includes(search.toLowerCase())
        );
    }, [templates, search]);

    const handleDelete = async (id: string, name: string) => {
        if (window.confirm(`Are you sure you want to delete ${name}?`)) {
            await deleteMutation.mutateAsync(id);
        }
    };

    const handleDuplicate = async (id: string) => {
        await duplicateMutation.mutateAsync(id);
    };

    return (
        <div className="max-w-7xl mx-auto space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-2xl font-bold text-white tracking-tight">Email Templates</h1>
                <button 
                    onClick={() => navigate('/email-templates/new')}
                    className="flex items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white px-4 py-2 rounded-md font-medium transition-colors"
                >
                    <Plus className="w-5 h-5" />
                    New Template
                </button>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden flex flex-col">
                <div className="p-4 border-b border-slate-800 flex items-center justify-between gap-4">
                    <div className="flex items-center gap-4 flex-1">
                        <div className="relative max-w-sm w-full">
                            <Search className="w-4 h-4 absolute left-3 top-2.5 text-slate-500" />
                            <input 
                                type="text"
                                placeholder="Search templates..."
                                className="w-full bg-slate-950 border border-slate-800 rounded-md pl-10 pr-4 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
                                value={search}
                                onChange={e => setSearch(e.target.value)}
                            />
                        </div>
                    </div>
                </div>

                <div className="overflow-x-auto min-h-[400px]">
                    <table className="w-full text-left text-sm whitespace-nowrap">
                        <thead className="bg-slate-950/50 text-slate-400 border-b border-slate-800">
                            <tr>
                                <th className="px-4 py-3 font-medium">Name</th>
                                <th className="px-4 py-3 font-medium">Subject</th>
                                <th className="px-4 py-3 font-medium">Category</th>
                                <th className="px-4 py-3 font-medium">Updated</th>
                                <th className="px-4 py-3 font-medium w-12 cursor-pointer"></th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800">
                            {isLoading ? (
                                <tr><td colSpan={5} className="px-4 py-8 text-center text-slate-500">Loading templates...</td></tr>
                            ) : filteredTemplates.length === 0 ? (
                                <tr>
                                    <td colSpan={5} className="px-4 py-16 text-center text-slate-400">
                                        <p className="text-lg">No templates match</p>
                                    </td>
                                </tr>
                            ) : (
                                filteredTemplates.map((template: EmailTemplate) => (
                                    <tr 
                                        key={template.id} 
                                        className="hover:bg-slate-800/50 cursor-pointer transition-colors group"
                                        onClick={() => navigate(`/email-templates/${template.id}`)}
                                    >
                                        <td className="px-4 py-3">
                                            <div className="font-medium text-slate-200">{template.name}</div>
                                            {template.description && <div className="text-xs text-slate-500 truncate max-w-xs">{template.description}</div>}
                                        </td>
                                        <td className="px-4 py-3 text-slate-400 truncate max-w-xs">{template.subject}</td>
                                        <td className="px-4 py-3">
                                            <span className="px-2 py-1 rounded bg-slate-800 text-xs text-slate-300">
                                                {template.category}
                                            </span>
                                        </td>
                                        <td className="px-4 py-3 text-slate-400">{new Date(template.updated_at).toLocaleDateString()}</td>
                                        <td className="px-4 py-3 text-slate-500 text-center relative" onClick={e => e.stopPropagation()}>
                                            <div className="group-hover:opacity-100 opacity-0 transition-opacity flex items-center justify-end gap-2 pr-4">
                                                <button onClick={() => navigate(`/email-templates/${template.id}`)} className="p-1.5 hover:bg-slate-700 rounded text-slate-400 hover:text-blue-400" title="Edit"><Edit2 className="w-4 h-4" /></button>
                                                <button onClick={() => handleDuplicate(template.id)} className="p-1.5 hover:bg-slate-700 rounded text-slate-400 hover:text-green-400" title="Duplicate"><Copy className="w-4 h-4" /></button>
                                                <button onClick={() => handleDelete(template.id, template.name)} className="p-1.5 hover:bg-slate-700 rounded text-slate-400 hover:text-red-400" title="Delete"><Trash2 className="w-4 h-4" /></button>
                                            </div>
                                            <button className="p-1 group-hover:hidden absolute top-3 right-4"><MoreVertical className="w-4 h-4" /></button>
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
