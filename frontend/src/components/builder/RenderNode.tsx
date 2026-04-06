import React from 'react';
import { SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { type ComponentNode } from '../../types/builder';
import { useBuilderStore } from '../../store/builderStore';

interface RenderNodeProps {
    node: ComponentNode;
}

export const RenderNode: React.FC<RenderNodeProps> = ({ node }) => {
    const selectedNodeIds = useBuilderStore(state => state.selectedNodeIds);
    const setSelectedNodes = useBuilderStore(state => state.setSelectedNodes);

    const isSelected = selectedNodeIds.includes(node.id);

    // Droppable target if it's a layout element
    const isContainer = ['Container', 'Grid', 'Columns', 'Column', 'Form Container'].includes(node.type);
    
    const {
        attributes,
        listeners,
        setNodeRef,
        transform,
        transition,
        isDragging
    } = useSortable({
        id: node.id,
        disabled: node.isLocked,
        data: {
            acceptsChildren: isContainer,
            type: node.type
        }
    });

    const handleClick = (e: React.MouseEvent) => {
        e.stopPropagation(); // prevent parent selection
        setSelectedNodes([node.id]);
    };

    // Base CSS for rendering inside the iframe.
    let ComponentTag: any = 'div';
    let baseClassName = '';

    switch (node.type) {
        case 'Heading':
            ComponentTag = node.props.level || 'h2';
            baseClassName = 'font-bold text-slate-900';
            break;
        case 'Paragraph':
            ComponentTag = 'p';
            baseClassName = 'text-slate-600 leading-relaxed';
            break;
        case 'Submit Button':
        case 'Button':
            ComponentTag = 'button';
            baseClassName = 'bg-blue-600 text-white font-medium rounded-md px-4 py-2.5 hover:bg-blue-700 transition cursor-pointer shadow-sm';
            break;
        case 'Container':
            baseClassName = 'w-full min-h-[50px] flex flex-col gap-2';
            break;
        case 'Form Container':
            ComponentTag = 'form';
            baseClassName = 'w-full min-h-[50px] flex flex-col gap-4 bg-white p-6 shadow-sm rounded-lg border border-slate-200';
            break;
        case 'Grid':
            baseClassName = 'w-full min-h-[50px] grid grid-cols-1 md:grid-cols-2 gap-4';
            break;
        case 'Columns':
            baseClassName = 'w-full min-h-[50px] flex flex-col md:flex-row gap-4';
            break;
        case 'Image':
            ComponentTag = 'img';
            baseClassName = 'max-w-full h-auto rounded';
            break;
        case 'Text Input':
        case 'Email Input':
        case 'Password Input':
            ComponentTag = 'input';
            baseClassName = 'border border-slate-300 rounded-md px-3 py-2 w-full focus:outline-none focus:ring-2 focus:ring-blue-500 text-slate-900 placeholder:text-slate-400';
            break;
    }

    // Merge standard style and tailwind classes
    // We add explicitly transition-all so margin/padding scrubbing is smooth.
    const combinedStyle = {
        ...node.style,
        position: 'relative' as any,
        transform: CSS.Transform.toString(transform),
        transition,
        opacity: isDragging ? 0.3 : 1,
        ...(isSelected ? { outline: '2px solid #38bdf8', outlineOffset: '-2px' } : {})
    };

    const isImage = node.type === 'Image';

    const propsWithText = {
        ...node.props,
        ...(ComponentTag === 'input' && { 
            type: node.type === 'Email Input' ? 'email' : node.type === 'Password Input' ? 'password' : 'text',
            placeholder: node.props.placeholder,
            readOnly: true // prevent typing inside builder
        })
    };

    const innerContent = ['Heading', 'Paragraph', 'Button', 'Submit Button'].includes(node.type) 
        ? node.props.text 
        : null;

    if (ComponentTag === 'input') {
        return (
            <div className="relative group cursor-pointer w-full" onClick={handleClick}>
                <ComponentTag 
                    className={baseClassName}
                    style={combinedStyle}
                    {...propsWithText}
                />
            </div>
        );
    }

    if (isImage) {
        return (
             <div className="relative group cursor-pointer" onClick={handleClick}>
                 <ComponentTag 
                     className={baseClassName}
                     style={combinedStyle}
                     src={node.props.src || 'https://placehold.co/600x400/e2e8f0/64748b?text=Placeholder+Image'}
                     alt={node.props.alt || 'Placeholder'}
                 />
             </div>
        );
    }

    return (
        <ComponentTag 
            ref={setNodeRef}
            {...attributes}
            {...(node.id === 'root-1' ? {} : listeners)}
            className={`${baseClassName} relative group cursor-pointer transition-all ${isContainer ? 'empty:min-h-[80px] empty:bg-slate-50 border border-transparent hover:border-slate-300 empty:border-dashed empty:border-slate-300 empty:bg-slate-50/50 flex flex-col' : ''}`}
            style={combinedStyle}
            onClick={handleClick}
        >
            {innerContent}
            {isContainer && (
                <SortableContext items={node.children?.map(c => c.id) || []} strategy={verticalListSortingStrategy}>
                    {node.children?.map(child => (
                        <RenderNode key={child.id} node={child} />
                    ))}
                </SortableContext>
            )}
        </ComponentTag>
    );
};
