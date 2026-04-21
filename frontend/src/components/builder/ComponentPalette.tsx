import { type ComponentType } from '../../types/builder';
import { useBuilderStore } from '../../store/builderStore';
import { Type, Square, Layout, List, FormInput, MousePointerClick, Image as ImageIcon, Video, Navigation, Layers, CheckSquare, ListOrdered, PanelTop, PanelBottom, Baseline } from 'lucide-react';

interface PaletteItemProps {
    type: ComponentType;
    icon: React.ReactNode;
    label: string;
}

const PALETTE_GROUPS: { name: string, items: PaletteItemProps[] }[] = [
    {
        name: 'Layout & Navigation',
        items: [
            { type: 'container', label: 'Container', icon: <Square className="w-5 h-5" /> },
            { type: 'row', label: 'Row', icon: <Layout className="w-5 h-5" /> },
            { type: 'column', label: 'Column', icon: <List className="w-5 h-5" /> },
            { type: 'divider', label: 'Divider', icon: <Baseline className="w-5 h-5" /> },
            { type: 'navbar', label: 'Navbar', icon: <PanelTop className="w-5 h-5" /> },
            { type: 'footer', label: 'Footer', icon: <PanelBottom className="w-5 h-5" /> },
        ]
    },
    {
        name: 'Content',
        items: [
            { type: 'heading', label: 'Heading', icon: <Type className="w-5 h-5" /> },
            { type: 'paragraph', label: 'Paragraph', icon: <Type className="w-5 h-5" /> },
            { type: 'image', label: 'Image', icon: <ImageIcon className="w-5 h-5" /> },
            { type: 'video_embed', label: 'Video', icon: <Video className="w-5 h-5" /> },
        ]
    },
    {
        name: 'Interactions',
        items: [
            { type: 'button', label: 'Button', icon: <MousePointerClick className="w-5 h-5" /> },
            { type: 'accordion', label: 'Accordion', icon: <Layers className="w-5 h-5" /> },
            { type: 'tabs', label: 'Tabs', icon: <Navigation className="w-5 h-5" /> },
        ]
    },
    {
        name: 'Forms',
        items: [
            { type: 'form', label: 'Form Base', icon: <FormInput className="w-5 h-5" /> },
            { type: 'text_input', label: 'Text Input', icon: <FormInput className="w-5 h-5" /> },
            { type: 'email_input', label: 'Email', icon: <FormInput className="w-5 h-5" /> },
            { type: 'password_input', label: 'Password', icon: <FormInput className="w-5 h-5" /> },
            { type: 'select', label: 'Dropdown', icon: <ListOrdered className="w-5 h-5" /> },
            { type: 'checkbox', label: 'Checkbox', icon: <CheckSquare className="w-5 h-5" /> },
            { type: 'radio', label: 'Radio', icon: <CheckSquare className="w-5 h-5" /> },
            { type: 'submit_button', label: 'Submit', icon: <MousePointerClick className="w-5 h-5" /> },
        ]
    }
];

const DraggableItem = ({ item }: { item: PaletteItemProps }) => {
    const setActiveNativeDragItem = useBuilderStore(state => state.setActiveNativeDragItem);
    const setCurrentDropIndicator = useBuilderStore(state => state.setCurrentDropIndicator);

    const handleDragStart = (e: React.DragEvent<HTMLDivElement>) => {
        const payload = {
            id: `palette-${item.type}`,
            type: item.type,
            isPaletteOriginal: true
        };
        e.dataTransfer.setData('application/json', JSON.stringify(payload));
        e.dataTransfer.effectAllowed = 'copy';
        
        setActiveNativeDragItem(payload);
    };

    const handleDragEnd = (e: React.DragEvent<HTMLDivElement>) => {
        setActiveNativeDragItem(null);
        setCurrentDropIndicator(null);
    };

    return (
        <div 
            draggable={true}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
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
