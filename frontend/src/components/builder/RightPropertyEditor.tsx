import { useState } from 'react';
import { useBuilderStore } from '../../store/builderStore';
import { type ComponentNode } from '../../types/builder';

export const RightPropertyEditor = () => {
    const [activeTab, setActiveTab] = useState<'content' | 'style' | 'advanced'>('content');
    
    const project = useBuilderStore(state => state.project);
    const activePageId = useBuilderStore(state => state.activePageId);
    const selectedNodeIds = useBuilderStore(state => state.selectedNodeIds);
    const updateNode = useBuilderStore(state => state.updateNode);
    const removeNode = useBuilderStore(state => state.removeNode);

    const activePage = project?.pages.find(p => p.id === activePageId);

    // Helper to find selected Node generically (MVP supports single selection edge)
    const getSelectedNode = (nodes: ComponentNode[], targetId: string): ComponentNode | null => {
        for (const n of nodes) {
            if (n.id === targetId) return n;
            if (n.children) {
                const found = getSelectedNode(n.children, targetId);
                if (found) return found;
            }
        }
        return null;
    };

    let selectedNode: ComponentNode | null = null;
    if (activePage && selectedNodeIds.length === 1) {
        selectedNode = getSelectedNode([activePage.rootComponent], selectedNodeIds[0]);
    }

    if (!selectedNode) {
        return (
            <div className="w-[320px] border-l border-slate-800 bg-slate-900 flex flex-col shrink-0">
                <div className="p-8 text-center text-slate-500 text-sm mt-10">
                    Select a component in the canvas to edit its properties.
                </div>
            </div>
        );
    }

    const handlePropChange = (key: string, value: any) => {
        if (!activePageId) return;
        updateNode(activePageId, selectedNode!.id, {
            props: { ...selectedNode!.props, [key]: value }
        });
    };

    const handleStyleChange = (key: string, value: any) => {
        if (!activePageId) return;
        updateNode(activePageId, selectedNode!.id, {
            style: { ...selectedNode!.style, [key]: value }
        });
    };

    return (
        <div className="w-[320px] border-l border-slate-800 bg-slate-900 flex flex-col shrink-0 max-h-screen overflow-y-auto">
            {/* Tabs */}
            <div className="flex border-b border-slate-800 shrink-0 sticky top-0 bg-slate-900 z-10">
                <button 
                    className={`flex-1 py-3 text-center text-sm font-medium border-b-2 ${activeTab === 'style' ? 'border-accent-primary text-white' : 'border-transparent text-slate-400 hover:text-white'}`}
                    onClick={() => setActiveTab('style')}
                >
                    Style
                </button>
                <button 
                    className={`flex-1 py-3 text-center text-sm font-medium border-b-2 ${activeTab === 'content' ? 'border-accent-primary text-white' : 'border-transparent text-slate-400 hover:text-white'}`}
                    onClick={() => setActiveTab('content')}
                >
                    Content
                </button>
                <button 
                    className={`flex-1 py-3 text-center text-sm font-medium border-b-2 ${activeTab === 'advanced' ? 'border-accent-primary text-white' : 'border-transparent text-slate-400 hover:text-white'}`}
                    onClick={() => setActiveTab('advanced')}
                >
                    Advanced
                </button>
            </div>

            {/* Content Editor */}
            {activeTab === 'content' && (
                <div className="p-4 space-y-4">
                    <div className="mb-4">
                        <div className="text-xs text-slate-500 uppercase tracking-widest font-semibold mb-2">Component</div>
                        <div className="text-white text-lg font-medium">{selectedNode.type}</div>
                    </div>

                    {['Heading', 'Paragraph', 'Button', 'Submit Button'].includes(selectedNode.type) && (
                        <div className="space-y-1">
                            <label className="text-xs font-medium text-slate-400">Text Content</label>
                            <textarea 
                                className="w-full bg-slate-950 border border-slate-700 rounded p-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
                                value={selectedNode.props.text || ''}
                                onChange={(e) => handlePropChange('text', e.target.value)}
                                rows={3}
                            />
                        </div>
                    )}
                    
                    {['Heading'].includes(selectedNode.type) && (
                        <div className="space-y-1">
                            <label className="text-xs font-medium text-slate-400">Heading Level</label>
                            <select 
                                className="w-full bg-slate-950 border border-slate-700 rounded p-2 text-sm text-slate-200 focus:outline-none hover:border-slate-500"
                                value={selectedNode.props.level || 'h2'}
                                onChange={(e) => handlePropChange('level', e.target.value)}
                            >
                                <option value="h1">H1</option>
                                <option value="h2">H2</option>
                                <option value="h3">H3</option>
                                <option value="h4">H4</option>
                            </select>
                        </div>
                    )}

                    {['Text Input', 'Email Input', 'Password Input'].includes(selectedNode.type) && (
                         <div className="space-y-4">
                            <div className="space-y-1">
                                <label className="text-xs font-medium text-slate-400">Placeholder</label>
                                <input 
                                    type="text"
                                    className="w-full bg-slate-950 border border-slate-700 rounded p-2 text-sm text-slate-200 focus:outline-none"
                                    value={selectedNode.props.placeholder || ''}
                                    onChange={(e) => handlePropChange('placeholder', e.target.value)}
                                />
                            </div>
                         </div>
                    )}
                </div>
            )}

            {/* Style Editor */}
            {activeTab === 'style' && (
                <div className="p-4 space-y-6">
                    <div className="space-y-3">
                        <div className="text-xs font-medium text-slate-400 flex justify-between">Layout</div>
                        
                        <div className="grid grid-cols-2 gap-2">
                            <div className="space-y-1">
                                <label className="text-[10px] text-slate-500 uppercase">Width</label>
                                <input 
                                    className="w-full bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-sm text-slate-200"
                                    value={selectedNode.style.width || ''}
                                    onChange={e => handleStyleChange('width', e.target.value)}
                                    placeholder="auto"
                                />
                            </div>
                            <div className="space-y-1">
                                <label className="text-[10px] text-slate-500 uppercase">Height</label>
                                <input 
                                    className="w-full bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-sm text-slate-200"
                                    value={selectedNode.style.height || ''}
                                    onChange={e => handleStyleChange('height', e.target.value)}
                                    placeholder="auto"
                                />
                            </div>
                        </div>

                        <div className="grid grid-cols-2 gap-2">
                            <div className="space-y-1">
                                <label className="text-[10px] text-slate-500 uppercase">Padding (px / rem)</label>
                                <input 
                                    className="w-full bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-sm text-slate-200"
                                    value={selectedNode.style.padding || ''}
                                    onChange={e => handleStyleChange('padding', e.target.value)}
                                    placeholder="0px"
                                />
                            </div>
                            <div className="space-y-1">
                                <label className="text-[10px] text-slate-500 uppercase">Margin (px / rem)</label>
                                <input 
                                    className="w-full bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-sm text-slate-200"
                                    value={selectedNode.style.margin || ''}
                                    onChange={e => handleStyleChange('margin', e.target.value)}
                                    placeholder="0px"
                                />
                            </div>
                        </div>

                    </div>

                    <div className="space-y-3">
                        <div className="text-xs font-medium text-slate-400">Typography</div>
                        <div className="space-y-1">
                            <label className="text-[10px] text-slate-500 uppercase">Text Color</label>
                            <input 
                                type="color"
                                className="w-full h-8 bg-slate-950 border border-slate-700 rounded p-1 cursor-pointer"
                                value={selectedNode.style.color || '#000000'}
                                onChange={e => handleStyleChange('color', e.target.value)}
                            />
                        </div>
                        <div className="space-y-1 mt-2">
                            <label className="text-[10px] text-slate-500 uppercase">Text Align</label>
                            <select 
                                className="w-full bg-slate-950 border border-slate-700 rounded p-2 text-sm text-slate-200 focus:outline-none"
                                value={selectedNode.style.textAlign || 'left'}
                                onChange={e => handleStyleChange('textAlign', e.target.value)}
                            >
                                <option value="left">Left</option>
                                <option value="center">Center</option>
                                <option value="right">Right</option>
                            </select>
                        </div>
                        <div className="space-y-1 mt-2">
                            <label className="text-[10px] text-slate-500 uppercase">Font Size (px/rem)</label>
                            <input 
                                className="w-full bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-sm text-slate-200"
                                value={selectedNode.style.fontSize || ''}
                                onChange={e => handleStyleChange('fontSize', e.target.value)}
                                placeholder="e.g. 16px or 1.5rem"
                            />
                        </div>
                    </div>

                    <div className="space-y-3">
                        <div className="text-xs font-medium text-slate-400">Background & Borders</div>
                        <div className="space-y-1">
                            <label className="text-[10px] text-slate-500 uppercase">Background Color</label>
                            <input 
                                type="color"
                                className="w-full h-8 bg-slate-950 border border-slate-700 rounded p-1 cursor-pointer"
                                value={selectedNode.style.backgroundColor || '#ffffff'}
                                onChange={e => handleStyleChange('backgroundColor', e.target.value)}
                            />
                        </div>
                        <div className="space-y-1 mt-2">
                            <label className="text-[10px] text-slate-500 uppercase">Border Radius (px/rem)</label>
                            <input 
                                className="w-full bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-sm text-slate-200"
                                value={selectedNode.style.borderRadius || ''}
                                onChange={e => handleStyleChange('borderRadius', e.target.value)}
                                placeholder="e.g. 4px or 50%"
                            />
                        </div>
                    </div>
                </div>
            )}

            {/* Delete Component Button */}
            {selectedNode.id !== activePage?.rootComponent.id && (
                <div className="mt-8 p-4 pt-0">
                    <button 
                        className="w-full py-2 bg-red-900/40 text-red-400 hover:bg-red-900/60 hover:text-red-300 rounded border border-red-900/50 transition-colors text-sm font-medium flex items-center justify-center gap-2"
                        onClick={() => removeNode(activePageId!, selectedNode!.id)}
                    >
                        Delete Component
                    </button>
                </div>
            )}
        </div>
    );
};
