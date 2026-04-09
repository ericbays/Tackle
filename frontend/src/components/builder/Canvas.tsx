import { useBuilderStore } from '../../store/builderStore';
import { RenderNode } from './RenderNode';

export const Canvas = () => {
    const activePageId = useBuilderStore(state => state.activePageId);
    const project = useBuilderStore(state => state.project);
    const zoom = useBuilderStore(state => state.zoom);
    const devicePreview = useBuilderStore(state => state.devicePreview);

    let deviceWidth = '100%';
    if (devicePreview === 'desktop') deviceWidth = '1440px';
    if (devicePreview === 'tablet') deviceWidth = '768px';
    if (devicePreview === 'mobile') deviceWidth = '375px';

    const activePage = project?.definition_json?.pages.find(p => p.page_id === activePageId);

    const handleDragOver = (e: React.DragEvent) => {
        e.preventDefault(); // Must prevent default to allow drop
        // If we are dragging over the direct root padding (not bubbling from a node)
        // clear the indicator so they know it goes to the bottom of the canvas
        if (e.target === e.currentTarget) {
            const indicator = useBuilderStore.getState().currentDropIndicator;
            if (indicator) {
                useBuilderStore.getState().setCurrentDropIndicator(null);
            }
        }
    };

    const handleDrop = (e: React.DragEvent) => {
        e.preventDefault();
        
        // Only accept the drop if we are not actively targeting a component Hitbox
        // Since hitboxes sit over components, if we hit the root canvas, it's just raw padding area.
        const indicator = useBuilderStore.getState().currentDropIndicator;
        if (indicator) {
            // A child hitbox is already handling this drop via bubbling, let it handle it.
            return;
        }

        const payloadRaw = e.dataTransfer.getData('application/json');
        if (!payloadRaw) return;

        try {
            const payload = JSON.parse(payloadRaw);
            if (activePageId && payload.type) {
                if (payload.isPaletteOriginal) {
                    useBuilderStore.getState().addNode(activePageId, 'canvas-root', payload.type, 'bottom');
                } else {
                    useBuilderStore.getState().moveNode(activePageId, payload.id, 'canvas-root', 'bottom');
                }
            }
        } catch (err) {
            console.error("Invalid canvas drop payload", err);
        }

        useBuilderStore.getState().setCurrentDropIndicator(null);
        useBuilderStore.getState().setActiveNativeDragItem(null);
    };

    return (
        <div className="flex-1 bg-slate-950 overflow-auto relative p-8">
            <div className="w-full flex justify-center">
                <div 
                    style={{ 
                        width: deviceWidth, 
                        transform: `scale(${zoom / 100})`, 
                        transition: 'all 0.2s ease-out',
                        transformOrigin: 'top center',
                        minHeight: '100vh',
                        boxShadow: '0 0 40px rgba(0,0,0,0.5)'
                    }}
                    className="bg-white text-slate-900 font-sans"
                >
                    {activePage && activePage.component_tree && (
                        <div 
                            className="canvas-root min-h-screen relative"
                            onDragOver={handleDragOver}
                            onDrop={handleDrop}
                        >
                            {activePage.component_tree.map(node => (
                                <RenderNode key={node.component_id} node={node} />
                            ))}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};
