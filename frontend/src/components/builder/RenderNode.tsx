import React from 'react';
import { type ComponentNode } from '../../types/builder';
import { useBuilderStore } from '../../store/builderStore';

interface RenderNodeProps {
    node: ComponentNode;
}

export const RenderNode: React.FC<RenderNodeProps> = ({ node }) => {
    const selectedNodeIds = useBuilderStore(state => state.selectedNodeIds);
    const setSelectedNodes = useBuilderStore(state => state.setSelectedNodes);
    const activePageId = useBuilderStore(state => state.activePageId);

    const activeNativeDragItem = useBuilderStore(state => state.activeNativeDragItem);
    const setActiveNativeDragItem = useBuilderStore(state => state.setActiveNativeDragItem);
    const currentDropIndicator = useBuilderStore(state => state.currentDropIndicator);
    const setCurrentDropIndicator = useBuilderStore(state => state.setCurrentDropIndicator);

    const addNode = useBuilderStore(state => state.addNode);
    const moveNode = useBuilderStore(state => state.moveNode);

    const isSelected = selectedNodeIds.includes(node.component_id);
    const isRoot = node.component_id === 'root-1';

    // Droppable target if it's a layout element
    const isContainer = ['container', 'row', 'column', 'section', 'card', 'navbar', 'footer', 'sidebar', 'tabs', 'form'].includes(node.type);

    const handleClick = (e: React.MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setSelectedNodes([node.component_id]);
    };

    const handleDragStart = (e: React.DragEvent<HTMLElement>) => {
        if (isRoot) {
            e.preventDefault();
            return;
        }
        e.stopPropagation();

        const payload = {
            id: node.component_id,
            type: node.type,
            isPaletteOriginal: false
        };
        e.dataTransfer.setData('application/json', JSON.stringify(payload));
        e.dataTransfer.effectAllowed = 'move';
        setActiveNativeDragItem(payload);
    };

    const handleDragEnd = (e: React.DragEvent<HTMLElement>) => {
        e.stopPropagation();
        setActiveNativeDragItem(null);
        setCurrentDropIndicator(null);
    };

    const handleDragOver = (e: React.DragEvent<HTMLElement>) => {
        if (!activeNativeDragItem || activeNativeDragItem.id === node.component_id) return;

        e.preventDefault();
        e.stopPropagation();

        const rect = e.currentTarget.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;

        let pos: 'top' | 'bottom' | 'left' | 'right' | 'inside' = 'bottom';

        if (isRoot) {
            // The root page container engulfs everything, nesting is the only option here.
            pos = 'inside';
        } else {
            // Use strict, fixed pixel hitboxes for edges to prioritize nesting ease
            const thresholdX = 15;
            const thresholdY = 15;

            if (y < thresholdY) pos = 'top';
            else if (y > rect.height - thresholdY) pos = 'bottom';
            else if (x < thresholdX) pos = 'left';
            else if (x > rect.width - thresholdX) pos = 'right';
            else pos = isContainer ? 'inside' : 'bottom';
        }

        if (currentDropIndicator?.nodeId !== node.component_id || currentDropIndicator?.position !== pos) {
            setCurrentDropIndicator({ nodeId: node.component_id, position: pos });
        }
    };

    const handleDrop = (e: React.DragEvent<HTMLElement>) => {
        e.preventDefault();
        e.stopPropagation();

        if (!currentDropIndicator || currentDropIndicator.nodeId !== node.component_id) {
            setCurrentDropIndicator(null);
            // If dragging from palette, clear state so it doesn't get stuck
            if (activeNativeDragItem?.isPaletteOriginal) {
                setActiveNativeDragItem(null);
            }
            return;
        }

        const position = currentDropIndicator.position;
        const payloadRaw = e.dataTransfer.getData('application/json');

        if (payloadRaw) {
            try {
                const payload = JSON.parse(payloadRaw);
                if (payload.type && activePageId) {
                    if (payload.isPaletteOriginal) {
                        addNode(activePageId, node.component_id, payload.type, position);
                    } else if (payload.id !== node.component_id) {
                        moveNode(activePageId, payload.id, node.component_id, position);
                    }
                }
            } catch (err) {
                console.error("Invalid drop payload", err);
            }
        }

        setCurrentDropIndicator(null);
        setActiveNativeDragItem(null);
    };

    let ComponentTag: any = 'div';
    let baseClassName = '';

    switch (node.type) {
        case 'heading':
            ComponentTag = node.properties?.level || 'h2';
            baseClassName = 'font-bold text-slate-800 text-2xl';
            break;
        case 'paragraph':
        case 'text':
            ComponentTag = 'p';
            baseClassName = 'text-slate-600 leading-relaxed whitespace-pre-wrap';
            break;
        case 'submit_button':
        case 'button':
            ComponentTag = 'button';
            baseClassName = 'bg-blue-600 text-white font-medium rounded-md px-5 py-2 hover:bg-blue-700 transition shadow-sm w-fit';
            break;
        case 'container':
        case 'section':
            baseClassName = 'w-full flex flex-col';
            break;
        case 'column':
            baseClassName = 'flex-1 min-w-[120px] flex flex-col';
            break;
        case 'row':
            baseClassName = 'w-full flex flex-row items-start flex-wrap';
            break;
        case 'navbar':
            ComponentTag = 'nav';
            baseClassName = 'w-full flex flex-row items-center justify-between border-b border-slate-200';
            break;
        case 'footer':
            ComponentTag = 'footer';
            baseClassName = 'w-full flex flex-col items-center justify-center border-t border-slate-200';
            break;
        case 'tabs':
            baseClassName = 'w-full flex flex-col border border-slate-200 rounded';
            break;
        case 'accordion':
            baseClassName = 'w-full flex flex-col border border-slate-200 rounded divide-y divide-slate-200';
            break;
        case 'form':
            ComponentTag = 'form';
            baseClassName = 'w-full flex flex-col bg-white rounded-xl shadow-sm border border-slate-200';
            break;
        case 'image':
            ComponentTag = 'img';
            baseClassName = 'max-w-full h-auto rounded';
            break;
        case 'video_embed':
            ComponentTag = 'iframe';
            baseClassName = 'w-full aspect-video rounded border-0';
            break;
        case 'text_input':
        case 'email_input':
        case 'password_input':
            ComponentTag = 'input';
            baseClassName = 'border border-slate-300 rounded-lg px-4 py-2.5 w-full focus:outline-none focus:ring-2 focus:ring-blue-500 text-slate-900 placeholder:text-slate-400 bg-slate-50 pointer-events-none';
            break;
        case 'select':
            ComponentTag = 'select';
            baseClassName = 'border border-slate-300 rounded-lg px-4 py-2.5 w-full focus:outline-none focus:ring-2 focus:ring-blue-500 text-slate-900 bg-slate-50';
            break;
        case 'checkbox':
        case 'radio':
            ComponentTag = 'fieldset';
            baseClassName = 'flex flex-col gap-2';
            break;
        default:
            baseClassName = 'flex flex-col';
            break;
    }

    // Builder Visual Bounding Boxes & Outlines
    let builderClasses = 'relative group transition-all duration-150 outline-offset-[-1px] ';

    if (!isRoot) {
        if (isContainer) {
            // Structural layout constraints strictly for the builder UX
            builderClasses += 'p-4 pt-8 gap-4 min-h-[100px] min-w-[100px] outline outline-1 outline-slate-300 outline-dashed hover:outline-blue-400 hover:bg-blue-50/5 ';
        } else {
            // Leaf node constraints
            builderClasses += 'p-2 hover:outline outline-1 outline-blue-300 outline-dashed hover:bg-blue-50/5 ';
        }
    }

    if (isSelected && !isRoot) {
        builderClasses += 'outline outline-2 outline-blue-500 z-10 bg-blue-50/5 shadow-sm ';
    }

    const isActiveTarget = currentDropIndicator?.nodeId === node.component_id;
    if (isActiveTarget) {
        const pos = currentDropIndicator.position;
        if (pos === 'top') builderClasses += '!border-t-4 !border-t-blue-500 rounded-none z-20 shadow-lg ';
        if (pos === 'bottom') builderClasses += '!border-b-4 !border-b-blue-500 rounded-none z-20 shadow-lg ';
        if (pos === 'left') builderClasses += '!border-l-4 !border-l-blue-500 rounded-none z-20 shadow-lg ';
        if (pos === 'right') builderClasses += '!border-r-4 !border-r-blue-500 rounded-none z-20 shadow-lg ';
        if (pos === 'inside') builderClasses += '!outline-4 !outline-blue-500 !bg-blue-100/30 z-20 shadow-lg ';
    }

    const combinedStyle = {
        ...(node.properties?.style || {}),
        position: 'relative' as any,
        opacity: activeNativeDragItem?.id === node.component_id ? 0.4 : 1,
    };

    builderClasses += ` node-${node.component_id} `;

    const DynamicStyles = () => {
        if (!node.properties?.hover_style && !node.properties?.active_style) return null;
        let cssString = '';
        
        if (node.properties?.hover_style && Object.keys(node.properties.hover_style).length > 0) {
            const hoverEntries = Object.entries(node.properties.hover_style).map(([k, v]) => `${k.replace(/([A-Z])/g, "-$1").toLowerCase()}: ${v} !important;`).join(' ');
            cssString += `.node-${node.component_id}:hover { ${hoverEntries} } `;
        }

        if (node.properties?.active_style && Object.keys(node.properties.active_style).length > 0) {
            const activeEntries = Object.entries(node.properties.active_style).map(([k, v]) => `${k.replace(/([A-Z])/g, "-$1").toLowerCase()}: ${v} !important;`).join(' ');
            cssString += `.node-${node.component_id}:active { ${activeEntries} } `;
        }

        return <style dangerouslySetInnerHTML={{ __html: cssString }} />;
    };

    const isImage = node.type === 'image' || node.type === 'logo';
    const isVideo = node.type === 'video_embed';
    const isInput = ComponentTag === 'input';
    const isSelect = node.type === 'select';
    const isButton = node.type === 'button' || node.type === 'submit_button';
    const isVoidElement = isImage || isInput || isVideo || isSelect || isButton;

    const propsWithText = {
        ...node.properties,
        ...(isInput && {
            type: node.type === 'email_input' ? 'email' : node.type === 'password_input' ? 'password' : 'text',
            placeholder: node.properties?.placeholder,
            readOnly: true
        })
    };

    let innerContent: React.ReactNode = null;
    if (['heading', 'paragraph', 'text'].includes(node.type)) {
        innerContent = node.properties?.text || node.type;
    } else if (node.type === 'checkbox' || node.type === 'radio') {
        innerContent = (node.properties?.options || []).map((opt: any, i: number) => (
            <label key={i} className="flex items-center gap-2 text-sm text-slate-700">
                <input type={node.type} name={node.properties?.name || ''} value={opt.value} readOnly />
                {opt.label}
            </label>
        ));
    } else if (node.type === 'tabs') {
        innerContent = (
            <div className="w-full flex border-b border-slate-200">
                {(node.properties?.options || []).map((opt: any, i: number) => (
                    <div key={i} className={`py-2 px-4 text-sm font-medium border-b-2 ${i === 0 ? 'border-blue-500 text-blue-600' : 'border-transparent text-slate-500'}`}>
                        {opt.label}
                    </div>
                ))}
            </div>
        );
    } else if (node.type === 'accordion') {
        innerContent = (node.properties?.options || []).map((opt: any, i: number) => (
            <div key={i} className="w-full p-3 flex justify-between items-center text-sm font-medium text-slate-700 bg-slate-50">
                {opt.label}
                <span className="text-slate-400">+</span>
            </div>
        ));
    }

    const Badge = () => {
        if (isRoot) return null;

        let badgeColor = 'bg-blue-500/90 text-white';
        let badgeBorder = '';
        if (isContainer) {
            badgeColor = 'bg-slate-700/90 text-white';
        }
        if (isSelected) {
            badgeColor = 'bg-blue-600 text-white';
        }

        return (
            <div className={`absolute top-0 left-0 ${badgeColor} ${badgeBorder} text-[9px] uppercase tracking-[0.05em] font-semibold px-2 py-0.5 rounded-br-md z-30 pointer-events-none whitespace-nowrap backdrop-blur-sm transition-all ${isSelected ? 'flex' : 'hidden group-hover:flex'}`}>
                {node.properties?.component_name || node.type}
            </div>
        );
    };

    if (isVoidElement) {
        return (
            <div
                className={`relative inline-block ${builderClasses}`}
                onClick={handleClick}
                draggable={!isRoot}
                onDragStart={handleDragStart}
                onDragEnd={handleDragEnd}
                onDragOver={handleDragOver}
                onDrop={handleDrop}
                style={{ position: 'relative', opacity: combinedStyle.opacity, width: combinedStyle.width || '100%', flex: combinedStyle.flex }}
            >
                <DynamicStyles />
                <Badge />
                {isImage && (
                    <img
                        src={node.properties?.src || 'https://placehold.co/600x400/e2e8f0/64748b?text=Placeholder+Image'}
                        alt={node.properties?.alt || 'Placeholder'}
                        className={`w-full h-full ${baseClassName}`}
                        style={combinedStyle}
                    />
                )}
                {isVideo && (
                    <iframe 
                        src={node.properties?.src || 'https://www.youtube.com/embed/dQw4w9WgXcQ'}
                        className={`w-full h-full pointer-events-none ${baseClassName}`}
                        style={combinedStyle}
                        title="Video"
                    />
                )}
                {isInput && (
                    <input
                        type={node.type === 'email_input' ? 'email' : node.type === 'password_input' ? 'password' : 'text'}
                        placeholder={node.properties?.placeholder || 'Enter text...'}
                        className={`w-full h-full ${baseClassName}`}
                        style={combinedStyle}
                        readOnly
                    />
                )}
                {isSelect && (
                    <select
                        className={`w-full h-full ${baseClassName} pointer-events-none`}
                        style={combinedStyle}
                    >
                        {(node.properties?.options || []).map((opt: any, i: number) => (
                            <option key={i} value={opt.value}>{opt.label}</option>
                        ))}
                    </select>
                )}
                {isButton && (
                    <button
                        className={`w-full h-full ${baseClassName} pointer-events-none`}
                        style={combinedStyle}
                    >
                        {node.properties?.text || node.properties?.content || node.type}
                    </button>
                )}
            </div>
        );
    }

    return (
        <ComponentTag
            className={`${baseClassName} ${builderClasses}`}
            style={combinedStyle}
            onClick={handleClick}
            draggable={!isRoot}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
            onDragOver={handleDragOver}
            onDrop={handleDrop}
        >
            <DynamicStyles />
            <Badge />
            {innerContent}
            {isContainer && node.children?.map(child => (
                <RenderNode key={child.component_id} node={child} />
            ))}
        </ComponentTag>
    );
};
