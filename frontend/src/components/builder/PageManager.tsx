import { useState } from 'react';
import { useBuilderStore } from '../../store/builderStore';
import { FileText, Plus, Settings, Trash2, Edit2, X, Check } from 'lucide-react';
import { type PageNode } from '../../types/builder';

export const PageManager = () => {
    const project = useBuilderStore(state => state.project);
    const activePageId = useBuilderStore(state => state.activePageId);
    const setActivePage = useBuilderStore(state => state.setActivePage);
    const addPage = useBuilderStore(state => state.addPage);
    const removePage = useBuilderStore(state => state.removePage);
    const updatePage = useBuilderStore(state => state.updatePage);

    const [editingPageId, setEditingPageId] = useState<string | null>(null);
    const [editForm, setEditForm] = useState<Partial<PageNode>>({});

    // If no project, safely return empty
    if (!project || !project.definition_json) return null;

    const pages = project.definition_json.pages || [];

    const handleEditStart = (e: React.MouseEvent, page: PageNode) => {
        e.stopPropagation();
        setEditingPageId(page.page_id);
        setEditForm({ name: page.name, route: page.route, title: page.title });
    };

    const handleEditSave = (e: React.MouseEvent, pageId: string) => {
        e.stopPropagation();
        updatePage(pageId, editForm);
        setEditingPageId(null);
    };

    const handleEditCancel = (e: React.MouseEvent) => {
        e.stopPropagation();
        setEditingPageId(null);
    };

    return (
        <div className="flex-1 flex flex-col bg-slate-900 overflow-hidden">
            <div className="p-4 border-b border-slate-800">
                <button 
                    onClick={() => addPage()}
                    className="w-full flex items-center justify-center gap-2 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 py-2 px-4 rounded text-xs font-semibold tracking-wider transition-colors"
                >
                    <Plus className="w-4 h-4 text-blue-400" /> NEW PAGE
                </button>
            </div>

            <div className="flex-1 overflow-y-auto p-3 space-y-2">
                {pages.map(page => {
                    const isActive = page.page_id === activePageId;
                    const isEditing = editingPageId === page.page_id;

                    return (
                        <div 
                            key={page.page_id} 
                            className={`flex flex-col rounded border transition-colors ${isActive ? 'bg-slate-800/80 border-slate-700' : 'bg-slate-900 border-slate-800 hover:bg-slate-800/50'}`}
                        >
                            <div 
                                className="flex items-center justify-between p-3 cursor-pointer group"
                                onClick={() => !isEditing && setActivePage(page.page_id)}
                            >
                                <div className="flex items-center gap-3 overflow-hidden flex-1">
                                    <FileText className={`w-4 h-4 shrink-0 ${isActive ? 'text-blue-400' : 'text-slate-500'}`} />
                                    <div className="flex flex-col overflow-hidden">
                                        <span className={`text-sm font-medium truncate ${isActive ? 'text-slate-200' : 'text-slate-400'}`}>{page.name}</span>
                                        <span className="text-[10px] text-slate-500 font-mono truncate">{page.route}</span>
                                    </div>
                                </div>

                                {!isEditing && (
                                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity ml-2 shrink-0">
                                        <button 
                                            className="p-1.5 text-slate-400 hover:text-white hover:bg-slate-700 rounded transition-colors"
                                            onClick={(e) => handleEditStart(e, page)}
                                            title="Edit route settings"
                                        >
                                            <Settings className="w-3.5 h-3.5" />
                                        </button>
                                        {pages.length > 1 && (
                                            <button 
                                                className="p-1.5 text-slate-400 hover:text-red-400 hover:bg-slate-700 rounded transition-colors"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    if (window.confirm(`Delete page '${page.name}'?`)) {
                                                        removePage(page.page_id);
                                                    }
                                                }}
                                                title="Delete Page"
                                            >
                                                <Trash2 className="w-3.5 h-3.5" />
                                            </button>
                                        )}
                                    </div>
                                )}
                            </div>

                            {/* Edit Form Rollout */}
                            {isEditing && (
                                <div className="p-3 pt-0 border-t border-slate-800 mt-1 bg-slate-800/30">
                                    <div className="space-y-3 mt-3">
                                        <div className="space-y-1">
                                            <label className="text-[10px] text-slate-500 uppercase tracking-widest">Internal Name</label>
                                            <input 
                                                className="w-full bg-slate-950 border border-slate-700/80 rounded px-2 py-1.5 text-xs text-slate-200 focus:outline-none focus:border-blue-500"
                                                value={editForm.name || ''}
                                                onChange={e => setEditForm({ ...editForm, name: e.target.value })}
                                                onClick={e => e.stopPropagation()}
                                            />
                                        </div>
                                        <div className="space-y-1">
                                            <label className="text-[10px] text-slate-500 uppercase tracking-widest">URL Route</label>
                                            <input 
                                                className="w-full bg-slate-950 border border-slate-700/80 rounded px-2 py-1.5 text-xs font-mono text-blue-400 focus:outline-none focus:border-blue-500"
                                                value={editForm.route || ''}
                                                onChange={e => setEditForm({ ...editForm, route: e.target.value.toLowerCase().replace(/\s+/g, '-') })}
                                                onClick={e => e.stopPropagation()}
                                                placeholder="/route"
                                            />
                                        </div>
                                        <div className="space-y-1">
                                            <label className="text-[10px] text-slate-500 uppercase tracking-widest">Page Title (Browser Tab)</label>
                                            <input 
                                                className="w-full bg-slate-950 border border-slate-700/80 rounded px-2 py-1.5 text-xs text-slate-200 focus:outline-none focus:border-blue-500"
                                                value={editForm.title || ''}
                                                onChange={e => setEditForm({ ...editForm, title: e.target.value })}
                                                onClick={e => e.stopPropagation()}
                                            />
                                        </div>
                                        <div className="flex gap-2 pt-2">
                                            <button 
                                                className="flex-1 flex items-center justify-center gap-2 py-1.5 rounded bg-blue-600 hover:bg-blue-500 text-white text-xs font-medium transition-colors"
                                                onClick={(e) => handleEditSave(e, page.page_id)}
                                            >
                                                <Check className="w-3.5 h-3.5" /> Save
                                            </button>
                                            <button 
                                                className="flex-1 flex items-center justify-center gap-2 py-1.5 rounded bg-slate-700 hover:bg-slate-600 text-slate-200 text-xs font-medium transition-colors"
                                                onClick={handleEditCancel}
                                            >
                                                <X className="w-3.5 h-3.5" /> Cancel
                                            </button>
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>
                    );
                })}
            </div>
        </div>
    );
};
