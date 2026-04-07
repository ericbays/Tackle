import { useState, useEffect, useRef } from 'react';
import { Trash2, Link as LinkIcon, Image as ImageIcon, Box, Type as TypeIcon, Table as TableIcon, AlignLeft, AlignCenter, AlignRight, AlignJustify } from 'lucide-react';

interface ElementContextMenuProps {
    x: number;
    y: number;
    targetNode: HTMLElement | null;
    onClose: () => void;
    onUpdate: () => void;
}

export default function ElementContextMenu({ x, y, targetNode, onClose, onUpdate }: ElementContextMenuProps) {
    const menuRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
                onClose();
            }
        };
        document.addEventListener('mousedown', handleClickOutside, true);
        return () => document.removeEventListener('mousedown', handleClickOutside, true);
    }, [onClose]);

    if (!targetNode) return null;

    const safeX = Math.min(x, window.innerWidth - 300);
    const safeY = Math.min(y, window.innerHeight - 400);

    const tagName = targetNode.tagName.toLowerCase();
    
    let typeConfig = 'container';
    if (tagName === 'img') typeConfig = 'image';
    else if (tagName === 'a') typeConfig = 'link';
    else if (['td', 'th', 'tr', 'tbody', 'thead', 'table'].includes(tagName)) typeConfig = 'table';
    else if (['p', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'span', 'strong', 'em', 'b', 'i', 'u', 'li'].includes(tagName)) typeConfig = 'typography';

    const renderHeader = () => {
        let icon = <Box className="w-4 h-4 text-blue-400" />;
        let title = "Container Options";
        if (typeConfig === 'image') { icon = <ImageIcon className="w-4 h-4 text-blue-400" />; title = "Image Config"; }
        if (typeConfig === 'link') { icon = <LinkIcon className="w-4 h-4 text-blue-400" />; title = "Link Config"; }
        if (typeConfig === 'table') { icon = <TableIcon className="w-4 h-4 text-blue-400" />; title = "Table Config"; }
        if (typeConfig === 'typography') { icon = <TypeIcon className="w-4 h-4 text-blue-400" />; title = "Text Config"; }

        return (
            <div className="flex items-center gap-2 mb-3 pb-2 border-b border-slate-700">
                {icon}
                <span className="text-xs font-semibold uppercase tracking-wider text-slate-300">{title}</span>
            </div>
        );
    };

    const applyStyle = (node: HTMLElement, prop: string, val: string) => { node.style.setProperty(prop, val); onUpdate(); };
    const applyAttr = (node: HTMLElement, attr: string, val: string) => { node.setAttribute(attr, val); onUpdate(); };

    // Common action
    const handleDelete = () => {
        if (typeConfig === 'table') {
            const tableNode = targetNode.closest('table') || targetNode;
            tableNode.remove();
        } else {
            targetNode.remove();
        }
        onUpdate();
        onClose();
    };

    // Specific Rendering blocks
    const ImageConfig = () => {
        const node = targetNode as HTMLImageElement;
        return (
            <div className="space-y-3 pb-2">
                <div>
                    <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Image Source (URL)</label>
                    <input type="text" defaultValue={node.src} onChange={e => { node.src = e.target.value; onUpdate(); }} className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white" />
                </div>
                <div>
                    <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Alt Text</label>
                    <input type="text" defaultValue={node.alt} onChange={e => applyAttr(node, 'alt', e.target.value)} className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white" />
                </div>
                <div className="flex gap-2">
                    <div className="flex-1">
                        <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Width</label>
                        <input type="text" defaultValue={node.style.width || node.getAttribute('width') || ''} onChange={e => { applyStyle(node, 'width', e.target.value); applyAttr(node, 'width', e.target.value); }} className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white" />
                    </div>
                    <div className="flex-1">
                        <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Height</label>
                        <input type="text" defaultValue={node.style.height || node.getAttribute('height') || ''} onChange={e => { applyStyle(node, 'height', e.target.value); applyAttr(node, 'height', e.target.value); }} className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white" />
                    </div>
                </div>
            </div>
        );
    };

    const LinkConfig = () => {
        const node = targetNode as HTMLAnchorElement;
        return (
            <div className="space-y-3 pb-2">
                <div>
                    <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Destination URL</label>
                    <input type="text" defaultValue={node.href} onChange={e => { node.href = e.target.value; onUpdate(); }} className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white" />
                </div>
                <div>
                    <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Open In</label>
                    <select defaultValue={node.target || '_self'} onChange={e => applyAttr(node, 'target', e.target.value)} className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white">
                        <option value="_self">Same Window (Default)</option>
                        <option value="_blank">New Tab (_blank)</option>
                    </select>
                </div>
                <div>
                    <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Link Color</label>
                    <div className="flex items-center gap-2">
                        <input type="color" defaultValue={node.style.color || '#3b82f6'} onChange={e => applyStyle(node, 'color', e.target.value)} className="w-6 h-6 rounded cursor-pointer border-0 p-0 bg-transparent" />
                        <span className="text-xs text-slate-400 font-mono">Override Blue</span>
                    </div>
                </div>
            </div>
        );
    };

    const TableConfig = () => {
        const tdNode = targetNode.closest('td') || targetNode.closest('th') as HTMLElement | null;
        const tableNode = targetNode.closest('table') as HTMLElement | null;
        
        return (
            <div className="space-y-4 pb-2">
                {tdNode && (
                    <div className="bg-slate-900/50 p-2 rounded border border-slate-700/50">
                        <h4 className="text-[10px] uppercase font-bold text-blue-400 mb-2">Cell Settings</h4>
                        <div className="grid grid-cols-2 gap-2 mb-2">
                            <div>
                                <label className="block text-[10px] text-slate-500 mb-1">Inner Padding</label>
                                <input type="text" defaultValue={tdNode.style.padding} onChange={e => applyStyle(tdNode, 'padding', e.target.value)} placeholder="e.g. 15px" className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1 focus:border-blue-500 text-white" />
                            </div>
                            <div>
                                <label className="block text-[10px] text-slate-500 mb-1">V-Align</label>
                                <select defaultValue={tdNode.style.verticalAlign || 'middle'} onChange={e => applyStyle(tdNode, 'vertical-align', e.target.value)} className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1 focus:border-blue-500 text-white">
                                    <option value="top">Top</option>
                                    <option value="middle">Middle</option>
                                    <option value="bottom">Bottom</option>
                                </select>
                            </div>
                        </div>
                        <div className="flex items-center gap-2">
                            <label className="text-[10px] text-slate-500">Fill Color</label>
                            <input type="color" defaultValue={tdNode.style.backgroundColor || '#ffffff'} onChange={e => applyStyle(tdNode, 'background-color', e.target.value)} className="w-5 h-5 rounded cursor-pointer bg-transparent border-0 p-0" />
                        </div>
                    </div>
                )}
                {tableNode && (
                    <div className="bg-slate-900/50 p-2 rounded border border-slate-700/50">
                        <h4 className="text-[10px] uppercase font-bold text-indigo-400 mb-2">Table Global Config</h4>
                        <div className="grid grid-cols-2 gap-2 mb-2">
                            <div>
                                <label className="block text-[10px] text-slate-500 mb-1">Table Width</label>
                                <input type="text" defaultValue={tableNode.style.width || tableNode.getAttribute('width') || ''} onChange={e => { applyStyle(tableNode, 'width', e.target.value); applyAttr(tableNode, 'width', e.target.value); }} placeholder="100% or 600px" className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1 focus:border-blue-500 text-white" />
                            </div>
                            <div>
                                <label className="block text-[10px] text-slate-500 mb-1">Outer Margin</label>
                                <input type="text" defaultValue={tableNode.style.margin} onChange={e => applyStyle(tableNode, 'margin', e.target.value)} placeholder="0 auto" className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1 focus:border-blue-500 text-white" />
                            </div>
                        </div>
                        <div>
                            <label className="block text-[10px] text-slate-500 mb-1">Border Constraint</label>
                            <input type="text" defaultValue={tableNode.style.border || (tableNode.getAttribute('border') === '1' ? '1px solid #ddd' : '')} onChange={e => applyStyle(tableNode, 'border', e.target.value)} placeholder="1px solid #e2e8f0" className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1 focus:border-blue-500 text-white" />
                        </div>
                    </div>
                )}
            </div>
        );
    };

    const TypographyConfig = () => {
        return (
            <div className="space-y-3 pb-2">
                <div className="flex bg-slate-900 border border-slate-700 rounded overflow-hidden">
                    <button onClick={() => applyStyle(targetNode, 'text-align', 'left')} className="flex-1 py-1 hover:bg-slate-700 hover:text-white text-slate-400 flex justify-center"><AlignLeft className="w-3.5 h-3.5" /></button>
                    <button onClick={() => applyStyle(targetNode, 'text-align', 'center')} className="flex-1 py-1 hover:bg-slate-700 hover:text-white text-slate-400 flex justify-center border-l border-r border-slate-700"><AlignCenter className="w-3.5 h-3.5" /></button>
                    <button onClick={() => applyStyle(targetNode, 'text-align', 'right')} className="flex-1 py-1 hover:bg-slate-700 hover:text-white text-slate-400 flex justify-center border-r border-slate-700"><AlignRight className="w-3.5 h-3.5" /></button>
                    <button onClick={() => applyStyle(targetNode, 'text-align', 'justify')} className="flex-1 py-1 hover:bg-slate-700 hover:text-white text-slate-400 flex justify-center"><AlignJustify className="w-3.5 h-3.5" /></button>
                </div>
                <div className="grid grid-cols-2 gap-3">
                    <div>
                        <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Font Size</label>
                        <input type="text" defaultValue={targetNode.style.fontSize} onChange={e => applyStyle(targetNode, 'font-size', e.target.value)} placeholder="e.g. 16px" className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white" />
                    </div>
                    <div>
                        <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Line Height</label>
                        <input type="text" defaultValue={targetNode.style.lineHeight} onChange={e => applyStyle(targetNode, 'line-height', e.target.value)} placeholder="e.g. 1.5" className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white" />
                    </div>
                </div>
                <div className="flex gap-4">
                    <div>
                        <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Text Color</label>
                        <input type="color" defaultValue={targetNode.style.color || '#000000'} onChange={e => applyStyle(targetNode, 'color', e.target.value)} className="w-6 h-6 rounded cursor-pointer bg-transparent border-0 p-0" />
                    </div>
                    <div>
                        <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Highlight</label>
                        <input type="color" defaultValue={targetNode.style.backgroundColor || '#ffffff'} onChange={e => applyStyle(targetNode, 'background-color', e.target.value)} className="w-6 h-6 rounded cursor-pointer bg-transparent border-0 p-0" />
                    </div>
                </div>
            </div>
        );
    };

    const ContainerConfig = () => {
        return (
            <div className="space-y-3 pb-2">
                <div className="flex items-center gap-2">
                    <label className="w-20 text-[10px] uppercase font-bold text-slate-500">Background</label>
                    <input type="color" defaultValue={targetNode.style.backgroundColor || '#ffffff'} onChange={e => applyStyle(targetNode, 'background-color', e.target.value)} className="w-6 h-6 rounded cursor-pointer bg-transparent border-0 p-0" />
                </div>
                <div>
                    <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Outer Margin</label>
                    <input type="text" defaultValue={targetNode.style.margin} onChange={e => applyStyle(targetNode, 'margin', e.target.value)} placeholder="0 auto (Center alignment)" className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white" />
                </div>
                <div>
                    <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Inner Space (Padding)</label>
                    <input type="text" defaultValue={targetNode.style.padding} onChange={e => applyStyle(targetNode, 'padding', e.target.value)} placeholder="e.g. 20px 40px" className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white" />
                </div>
                <div>
                    <label className="block text-[10px] uppercase font-bold text-slate-500 mb-1">Border CSS</label>
                    <input type="text" defaultValue={targetNode.style.border} onChange={e => applyStyle(targetNode, 'border', e.target.value)} placeholder="1px solid #e2e8f0" className="w-full bg-slate-900 border border-slate-700 rounded text-xs p-1.5 focus:border-blue-500 text-white" />
                </div>
            </div>
        );
    }

    return (
        <div 
            ref={menuRef}
            className="fixed z-[9999] w-64 bg-slate-800 border border-slate-700 rounded-md shadow-2xl p-3 text-slate-200 shadow-black/50"
            style={{ top: safeY, left: safeX }}
        >
            {renderHeader()}
            
            <div className="max-h-[300px] overflow-y-auto custom-scrollbar pr-1">
                {typeConfig === 'image' && <ImageConfig />}
                {typeConfig === 'link' && <LinkConfig />}
                {typeConfig === 'table' && <TableConfig />}
                {typeConfig === 'typography' && <TypographyConfig />}
                {typeConfig === 'container' && <ContainerConfig />}
            </div>

            <div className="h-px bg-slate-700 w-full mt-1 mb-3"></div>

            <button 
                onClick={handleDelete}
                className="w-full flex items-center justify-center gap-2 px-2 py-1.5 text-[11px] uppercase tracking-wider font-bold text-slate-300 bg-red-500/10 border border-red-500/20 hover:bg-red-500/20 rounded transition-colors"
                title={`Deletes the selected ${tagName.toUpperCase()} node completely`}
            >
                <Trash2 className="w-3.5 h-3.5 text-red-400" />
                Remove {typeConfig === 'table' ? 'Table Element' : 'Element'}
            </button>
        </div>
    );
}
