import React from 'react';
import { useBuilderStore } from '../../store/builderStore';
import { type ComponentNode } from '../../types/builder';
import { Layers, ChevronRight, ChevronDown, MousePointer2 } from 'lucide-react';

export const LayersPanel = () => {
    const project = useBuilderStore(state => state.project);
    const activePageId = useBuilderStore(state => state.activePageId);
    
    if (!project || !activePageId) return null;

    const activePage = project.definition_json?.pages?.find(p => p.page_id === activePageId);
    
    if (!activePage || !activePage.component_tree || activePage.component_tree.length === 0) {
        return <div className="p-4 text-xs text-slate-500">No layout nodes found.</div>;
    }

    return (
        <div className="flex flex-col h-full bg-slate-900 border-l border-slate-800 text-sm overflow-hidden">
            <div className="p-4 border-b border-slate-800 sticky top-0 bg-slate-900/95 backdrop-blur z-10">
                <h3 className="font-semibold text-slate-200 flex items-center gap-2 text-xs uppercase tracking-wider">
                    <Layers size={14} className="text-blue-400" />
                    DOM Tree (Layers)
                </h3>
                <p className="text-xs text-slate-500 mt-1">Navigate the underlying AST structure</p>
            </div>
            
            <div className="flex-1 overflow-y-auto p-2">
                {activePage.component_tree.map(node => (
                    <LayerNode key={node.component_id} node={node} depth={0} />
                ))}
            </div>
        </div>
    );
};

interface LayerNodeProps {
    node: ComponentNode;
    depth: number;
}

const LayerNode: React.FC<LayerNodeProps> = ({ node, depth }) => {
    const selectedNodeIds = useBuilderStore(state => state.selectedNodeIds);
    const setSelectedNodes = useBuilderStore(state => state.setSelectedNodes);
    
    // Auto-expand tree visually in standard state, simple implementation
    const [isExpanded, setIsExpanded] = React.useState(true);

    const isSelected = selectedNodeIds.includes(node.component_id);
    const hasChildren = node.children && node.children.length > 0;
    
    const isRoot = node.component_id === 'root-1';
    
    const handleSelect = (e: React.MouseEvent) => {
        e.stopPropagation();
        if (isRoot) return;
        setSelectedNodes([node.component_id]);
    };

    const toggleExpand = (e: React.MouseEvent) => {
        e.stopPropagation();
        setIsExpanded(!isExpanded);
    };

    return (
        <div className="select-none font-mono">
            <div 
                className={`flex items-center gap-1.5 py-1.5 px-2 rounded-md cursor-pointer transition-colors
                    ${isSelected ? 'bg-blue-600 pl-1.5 border-l-[3px] border-blue-400' : 'hover:bg-slate-800'}
                    ${isRoot ? 'opacity-50 cursor-default' : ''}
                `}
                style={{ paddingLeft: `${depth * 12 + (isSelected ? 6 : 8)}px` }}
                onClick={handleSelect}
            >
                {/* Expander Icon */}
                <div 
                    className={`w-4 h-4 flex items-center justify-center text-slate-500 hover:text-slate-300 ${hasChildren ? 'cursor-pointer' : 'opacity-0'}`}
                    onClick={hasChildren ? toggleExpand : undefined}
                >
                    {hasChildren && (isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />)}
                </div>

                <div className={`text-xs truncate flex-1 tracking-tight ${isSelected ? 'text-white' : 'text-slate-300'}`}>
                    <span className="opacity-60 pr-1">&lt;</span>
                    {node.properties?.component_name ? `${node.properties.component_name} (${node.type})` : node.type}
                    <span className="opacity-60 pl-1">&gt;</span>
                    
                    {/* Inline Content Snippet */}
                    {['heading', 'paragraph', 'text', 'button'].includes(node.type) && node.properties?.text && (
                        <span className="ml-2 text-slate-500 truncate inline-block max-w-[80px] align-bottom">
                            "{node.properties.text}"
                        </span>
                    )}
                </div>
                
                {isSelected && <MousePointer2 size={12} className="text-white/70" />}
            </div>

            {hasChildren && isExpanded && (
                <div className="flex flex-col relative before:absolute before:left-[22px] before:top-0 before:bottom-0 before:w-[1px] before:bg-slate-800">
                    {node.children!.map(child => (
                        <LayerNode key={child.component_id} node={child} depth={depth + 1} />
                    ))}
                </div>
            )}
        </div>
    );
};
