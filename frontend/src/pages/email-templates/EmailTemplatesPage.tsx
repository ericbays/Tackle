import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Search, MoreVertical, Trash2, Edit2, Copy } from 'lucide-react';
import { useEmailTemplates, useDeleteTemplate, useDuplicateTemplate } from '../../hooks/useEmailTemplates';
import type { EmailTemplate } from '../../hooks/useEmailTemplates';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Badge } from '../../components/ui/Badge';
import { Card } from '../../components/ui/Card';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '../../components/ui/Table';

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
                <Button onClick={() => navigate('/email-templates/new')} variant="primary" className="flex items-center gap-2">
                    <Plus className="w-5 h-5" />
                    New Template
                </Button>
            </div>

            <Card className="flex flex-col shadow-xl">
                <div className="p-4 border-b border-slate-800 flex items-center justify-between gap-4">
                    <div className="flex items-center gap-4 flex-1">
                        <div className="relative max-w-sm w-full">
                            <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 z-10" />
                            <Input 
                                type="text"
                                placeholder="Search templates..."
                                className="pl-10"
                                value={search}
                                onChange={e => setSearch(e.target.value)}
                            />
                        </div>
                    </div>
                </div>

                <div className="min-h-[400px]">
                    <Table className="border-0 shadow-none rounded-none rounded-b-xl border-t-0 p-0">
                        <TableHeader className="bg-slate-950/50">
                            <TableRow className="hover:bg-transparent">
                                <TableHead>Name</TableHead>
                                <TableHead>Subject</TableHead>
                                <TableHead>Category</TableHead>
                                <TableHead>Updated</TableHead>
                                <TableHead className="w-12 cursor-pointer"></TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {isLoading ? (
                                <TableRow><TableCell colSpan={5} className="py-8 text-center text-slate-500">Loading templates...</TableCell></TableRow>
                            ) : filteredTemplates.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={5} className="py-16 text-center text-slate-400">
                                        <p className="text-lg">No templates match</p>
                                    </TableCell>
                                </TableRow>
                            ) : (
                                filteredTemplates.map((template: EmailTemplate) => (
                                    <TableRow 
                                        key={template.id} 
                                        className="cursor-pointer group"
                                        onClick={() => navigate(`/email-templates/${template.id}`)}
                                    >
                                        <TableCell>
                                            <div className="font-medium text-slate-200">{template.name}</div>
                                            {template.description && <div className="text-xs text-slate-500 truncate max-w-xs">{template.description}</div>}
                                        </TableCell>
                                        <TableCell className="text-slate-400 truncate max-w-xs">{template.subject}</TableCell>
                                        <TableCell>
                                            <Badge variant="secondary">{template.category}</Badge>
                                        </TableCell>
                                        <TableCell className="text-slate-400">{new Date(template.updated_at).toLocaleDateString()}</TableCell>
                                        <TableCell className="text-center relative" onClick={e => e.stopPropagation()}>
                                            <div className="group-hover:opacity-100 opacity-0 transition-opacity flex items-center justify-end pr-4">
                                                <Button onClick={() => navigate(`/email-templates/${template.id}`)} variant="ghost" size="icon" title="Edit" className="hover:text-blue-400"><Edit2 className="w-4 h-4" /></Button>
                                                <Button onClick={() => handleDuplicate(template.id)} variant="ghost" size="icon" title="Duplicate" className="hover:text-green-400"><Copy className="w-4 h-4" /></Button>
                                                <Button onClick={() => handleDelete(template.id, template.name)} variant="ghost" size="icon" title="Delete" className="hover:text-red-400"><Trash2 className="w-4 h-4" /></Button>
                                            </div>
                                            <div className="group-hover:hidden absolute top-1/2 -translate-y-1/2 right-4 text-slate-500"><MoreVertical className="w-4 h-4" /></div>
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
