import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useEmailTemplate, useSaveEmailTemplate } from '../../hooks/useEmailTemplates';
import type { EmailTemplate } from '../../hooks/useEmailTemplates';
import HtmlEmailEditor from '../../components/email-templates/HtmlEmailEditor';
import VariableInsertMenu from '../../components/email-templates/VariableInsertMenu';
import { ArrowLeft, Save } from 'lucide-react';
import { useRef } from 'react';
import { toast } from 'react-hot-toast';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Button } from '../../components/ui/Button';

export default function EmailTemplateEditor() {
    const { id } = useParams();
    const navigate = useNavigate();
    const isNew = id === 'new';

    const { data: template, isLoading: isFetching } = useEmailTemplate(isNew ? null : id!);
    const saveMutation = useSaveEmailTemplate();

    const [draft, setDraft] = useState<Partial<EmailTemplate>>({
        name: '',
        description: '',
        subject: '',
        category: 'IT',
        tags: [],
        sender_name: '',
        sender_email: '',
        html_body: '<p>Hello world</p>',
    });

    const [lastSaved, setLastSaved] = useState<Date | null>(null);

    const subjectInputRef = useRef<HTMLInputElement>(null);

    const handleInsertSubjectVar = (variable: string) => {
        if (!subjectInputRef.current) {
            setDraft(prev => ({ ...prev, subject: (prev.subject || '') + variable }));
            return;
        }
        const input = subjectInputRef.current;
        const start = input.selectionStart || 0;
        const end = input.selectionEnd || 0;
        const current = draft.subject || '';
        const newSubject = current.substring(0, start) + variable + current.substring(end);
        
        setDraft(prev => ({ ...prev, subject: newSubject }));
        
        // Attempt to restore focus/cursor after state update
        setTimeout(() => {
            input.focus();
            input.setSelectionRange(start + variable.length, start + variable.length);
        }, 0);
    };

    useEffect(() => {
        if (template && !isNew) {
            setDraft(template);
            setLastSaved(new Date(template.updated_at));
        }
    }, [template, isNew]);

    const handleSave = async (closeAfterSave = false) => {
        try {
            const data = await saveMutation.mutateAsync({
                id: isNew ? undefined : id,
                payload: draft
            });
            setLastSaved(new Date());
            toast.success('Template saved successfully');
            
            if (closeAfterSave) {
                navigate('/email-templates');
            } else if (isNew && data?.id) {
                navigate(`/email-templates/${data.id}`, { replace: true });
            }
        } catch (err) {
            console.error(err);
            toast.error('Failed to save template');
        }
    };

    if (isFetching && !isNew) {
        return <div className="p-8 text-slate-500">Loading template...</div>;
    }

    return (
        <div className="flex flex-col flex-1 min-h-0 h-full">
            {/* Header */}
            <div className="flex items-center justify-between px-6 py-4 bg-slate-900 border-b border-slate-800 shrink-0">
                <div className="flex items-center gap-4">
                    <Button variant="outline" onClick={ () => navigate('/email-templates')} className="text-slate-400 hover:text-white transition-colors">
                        <ArrowLeft className="w-5 h-5" />
                    </Button>
                    <div>
                        <h1 className="text-lg font-semibold text-white">
                            {isNew ? 'New Email Template' : (draft.name || 'Untitled Template')}
                        </h1>
                        <p className="text-xs text-slate-500">
                            {saveMutation.isPending ? 'Saving...' : lastSaved ? `Last saved ${lastSaved.toLocaleTimeString()}` : 'Unsaved draft'}
                        </p>
                    </div>
                </div>
                <div className="flex items-center gap-3">
                    <Button variant="outline" 
                        onClick={ () => handleSave(false)}
                        className="px-4 py-2 text-sm font-medium text-slate-300 hover:text-white hover:bg-slate-800 rounded-md transition-colors"
                        disabled={saveMutation.isPending}
                    >
                        Save
                    </Button>
                    <Button variant="outline" 
                        onClick={ () => handleSave(true)}
                        className="flex items-center gap-2 px-4 py-2 text-sm font-medium bg-blue-600 hover:bg-blue-500 text-white rounded-md transition-colors outline-none"
                        disabled={saveMutation.isPending}
                    >
                        <Save className="w-4 h-4" /> Save & Close
                    </Button>
                </div>
            </div>

            {/* Non-Scrolling Content Area */}
            <div className="flex-1 flex flex-col min-h-0 overflow-hidden p-6 lg:p-8 space-y-6 max-h-full">
                
                {/* Metadata Card */}
                <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 grid grid-cols-1 md:grid-cols-2 gap-6 shrink-0">
                    <div className="space-y-4">
                        <div>
                            <label className="block text-sm font-medium text-slate-400 mb-1">Template Name *</label>
                            <Input 
                                type="text"
                                
                                value={draft.name || ''}
                                onChange={e => setDraft({...draft, name: e.target.value})}
                                placeholder="E.g., Q2 IT Password Reset"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-400 mb-1">Subject Line *</label>
                            <div className="relative">
                                <Input 
                                    ref={subjectInputRef}
                                    type="text"
                                    
                                    value={draft.subject || ''}
                                    onChange={e => setDraft({...draft, subject: e.target.value})}
                                    placeholder="Action Required: {{target.first_name}}"
                                />
                                <div className="absolute right-2 top-1.5 flex items-center">
                                    <VariableInsertMenu 
                                        onInsert={handleInsertSubjectVar}
                                        buttonClassName="text-xs bg-slate-800 hover:bg-slate-700 text-slate-300 px-2 py-1 rounded transition-colors"
                                    />
                                </div>
                            </div>
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-400 mb-1">Description</label>
                            <Input 
                                type="text"
                                
                                value={draft.description || ''}
                                onChange={e => setDraft({...draft, description: e.target.value})}
                            />
                        </div>
                    </div>

                    <div className="space-y-4">
                        <div>
                            <label className="block text-sm font-medium text-slate-400 mb-1">Sender Name *</label>
                            <Input 
                                type="text"
                                
                                value={draft.sender_name || ''}
                                onChange={e => setDraft({...draft, sender_name: e.target.value})}
                                placeholder="IT Support"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-400 mb-1">Sender Email *</label>
                            <Input 
                                type="email"
                                
                                value={draft.sender_email || ''}
                                onChange={e => setDraft({...draft, sender_email: e.target.value})}
                                placeholder="support@company.com"
                            />
                        </div>
                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label className="block text-sm font-medium text-slate-400 mb-1">Category *</label>
                                <Select 
                                    
                                    value={draft.category || ''}
                                    onChange={e => setDraft({...draft, category: e.target.value})}
                                >
                                    <option value="IT">IT</option>
                                    <option value="HR">HR</option>
                                    <option value="Finance">Finance</option>
                                    <option value="Shipping">Shipping</option>
                                    <option value="Generic">Generic</option>
                                </Select>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Unified Editor */}
                <div className="flex flex-col flex-1 min-h-0 w-full mt-4">
                    <HtmlEmailEditor 
                        content={draft.html_body || ''} 
                        onChange={(html) => setDraft({...draft, html_body: html})} 
                        onSave={() => handleSave(false)}
                    />
                </div>

            </div>
        </div>
    );
}
