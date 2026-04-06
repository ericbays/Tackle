import { useDraggable } from '@dnd-kit/core';
import { type ComponentType } from '../../types/builder';
import { Type, Square, Layout, List, FormInput, MousePointerClick, Image as ImageIcon } from 'lucide-react';

interface PaletteItemProps {
    type: ComponentType;
    icon: React.ReactNode;
    label: string;
}

const PALETTE_GROUPS: { name: string, items: PaletteItemProps[] }[] = [
    {
        name: 'Layout',
        items: [
            { type: 'Container', label: 'Container', icon: <Square className="w-5 h-5" /> },
            { type: 'Grid', label: 'Grid', icon: <Layout className="w-5 h-5" /> },
            { type: 'Columns', label: 'Columns', icon: <List className="w-5 h-5" /> },
        ]
    },
    {
        name: 'Content',
        items: [
            { type: 'Heading', label: 'Heading', icon: <Type className="w-5 h-5" /> },
            { type: 'Paragraph', label: 'Paragraph', icon: <Type className="w-5 h-5" /> },
            { type: 'Image', label: 'Image', icon: <ImageIcon className="w-5 h-5" /> },
        ]
    },
    {
        name: 'Forms',
        items: [
            { type: 'Form Container', label: 'Form', icon: <FormInput className="w-5 h-5" /> },
            { type: 'Text Input', label: 'Input', icon: <FormInput className="w-5 h-5" /> },
            { type: 'Submit Button', label: 'Button', icon: <MousePointerClick className="w-5 h-5" /> },
        ]
    }
];

const DraggableItem = ({ item }: { item: PaletteItemProps }) => {
    // Generate a unique ID for the palette source drag
    const { attributes, listeners, setNodeRef } = useDraggable({
        id: `palette-${item.type}`,
        data: {
            isPaletteOriginal: true,
            type: item.type
        }
    });

    return (
        <div 
            ref={setNodeRef} 
            {...listeners} 
            {...attributes}
            className="flex flex-col items-center justify-center p-3 cursor-grab bg-slate-900 border border-slate-700 rounded-md hover:bg-slate-800 hover:border-slate-600 transition-colors"
        >
            <div className="text-slate-400 mb-2">{item.icon}</div>
            <div className="text-xs text-slate-300 pointer-events-none">{item.label}</div>
        </div>
    );
};

export const ComponentPalette = () => {
    return (
        <div className="flex-1 overflow-y-auto p-4 space-y-6">
            <div className="relative">
                <input 
                    type="text" 
                    placeholder="Search components..." 
                    className="w-full bg-slate-950 border border-slate-800 rounded-md py-1.5 px-3 text-sm text-slate-200 focus:outline-none focus:border-accent-primary"
                />
            </div>
            
            {PALETTE_GROUPS.map(group => (
                <div key={group.name} className="space-y-3">
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">{group.name}</h3>
                    <div className="grid grid-cols-2 gap-2">
                        {group.items.map(item => (
                            <DraggableItem key={item.type} item={item} />
                        ))}
                    </div>
                </div>
            ))}
        </div>
    );
};
