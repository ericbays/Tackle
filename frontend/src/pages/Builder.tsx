import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Monitor, Smartphone, Tablet, ZoomIn, ZoomOut, Save, Play, Loader2, Undo2, Redo2 } from 'lucide-react';
import { useBuilderStore } from '../store/builderStore';
import { Canvas } from '../components/builder/Canvas';
import { ComponentPalette } from '../components/builder/ComponentPalette';
import { RightPropertyEditor } from '../components/builder/RightPropertyEditor';
import { DndContext, type DragEndEvent } from '@dnd-kit/core';
import { api } from '../services/api';

export default function Builder() {
  const { id } = useParams();
  // We would normally fetch the landing page data here utilizing TanStack
  
  const loadProject = useBuilderStore(state => state.loadProject);
  const zoom = useBuilderStore(state => state.zoom);
  const setZoom = useBuilderStore(state => state.setZoom);
  const devicePreview = useBuilderStore(state => state.devicePreview);
  const setDevicePreview = useBuilderStore(state => state.setDevicePreview);
  const undo = useBuilderStore(state => state.undo);
  const redo = useBuilderStore(state => state.redo);
  const historyIndex = useBuilderStore(state => state.historyIndex);
  const historyLength = useBuilderStore(state => state.history.length);

  const [isLoading, setIsLoading] = useState(true);
  const navigate = useNavigate();

  // Load from real DB API
  useEffect(() => {
    const fetchPage = async () => {
        setIsLoading(true);
        try {
            if (!id || id === 'new') {
                // Generate blank scaffolding
                loadProject({
                    id: 'new',
                    name: 'New Landing Page',
                    globalCss: '',
                    globalJs: '',
                    pages: [{
                        id: 'page-1',
                        name: 'Home',
                        isStartPage: true,
                        rootComponent: {
                            id: 'root-1',
                            type: 'Container',
                            isHidden: false,
                            isLocked: true,
                            props: {},
                            style: { padding: '2rem', minHeight: '100vh', backgroundColor: '#ffffff' },
                            children: []
                        }
                    }]
                });
            } else {
                // Hydrate from Postgres via Go API
                const res = await api.get(`/landing-pages/${id}`);
                // Safely validate payload
                if (res.data && res.data.pages) {
                    loadProject(res.data);
                }
            }
        } catch (err) {
            console.error('Failed to parse landing page:', err);
            // Default blank
        } finally {
            setIsLoading(false);
        }
    };

    fetchPage();
  }, [id, loadProject]);

  const handleDragEnd = (event: DragEndEvent) => {
      const { active, over } = event;
      if (!over) return;
      
      const activeId = active.id.toString();
      const overId = over.id.toString();

      // Check if this was dragged from the component palette flyout
      if (active.data.current?.isPaletteOriginal) {
          const type = active.data.current.type;
          const pageId = useBuilderStore.getState().activePageId;
          
          // Ascertain target dropzone. Is it a container? If not, rely on the sortable container context.
          const parentId = over.data.current?.acceptsChildren ? overId : over.data.current?.sortable?.containerId;
          
          if (pageId && parentId) {
              useBuilderStore.getState().addNode(pageId, parentId, type);
          }
      } else {
          // Visual tree reordering
          if (activeId !== overId) {
              const pageId = useBuilderStore.getState().activePageId;
              const newParentId = over.data.current?.acceptsChildren ? overId : over.data.current?.sortable?.containerId;
              const newIndex = over.data.current?.sortable?.index ?? 0;

              if (pageId && newParentId) {
                  useBuilderStore.getState().moveNode(pageId, activeId, newParentId, newIndex);
              }
          }
      }
  };

  const handleSave = async () => {
      const project = useBuilderStore.getState().project;
      if (!project) return;
      try {
          await api.put(`/landing-pages/${project.id}`, project);
          alert('Project saved successfully!');
      } catch (error) {
          console.error('Failed to save project', error);
          alert('Failed to save project. Ensure your backend is running.');
      }
  };

  if (isLoading) {
      return (
          <div className="h-screen w-screen flex items-center justify-center bg-slate-900 text-slate-400">
              <Loader2 className="w-8 h-8 animate-spin" />
              <span className="ml-3 font-medium">Hydrating Engine...</span>
          </div>
      );
  }

  return (
    <DndContext onDragEnd={handleDragEnd}>
        <div className="flex flex-col h-screen w-screen bg-slate-900 text-slate-200 overflow-hidden text-sm">
            
            {/* Top Toolbar */}
            <div className="h-14 border-b border-slate-800 bg-slate-900 flex items-center justify-between px-4 shrink-0 z-50">
                <div className="flex items-center gap-4">
                    <button onClick={() => navigate('/dashboard')} className="text-slate-400 hover:text-white transition-colors animate-pulse" title="Back to Dashboard">
                        <ArrowLeft className="w-5 h-5" />
                    </button>
                    <div className="font-semibold text-slate-200 cursor-pointer hover:bg-slate-800 px-2 py-1 rounded">
                        New Landing Page ▾
                    </div>
                </div>

                <div className="flex items-center gap-6">
                    <div className="flex items-center bg-slate-800 rounded-md p-0.5 border border-slate-700">
                        <button 
                            className={`p-1.5 rounded ${devicePreview === 'desktop' ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-white'}`}
                            onClick={() => setDevicePreview('desktop')}
                        >
                            <Monitor className="w-4 h-4" />
                        </button>
                        <button 
                            className={`p-1.5 rounded ${devicePreview === 'tablet' ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-white'}`}
                            onClick={() => setDevicePreview('tablet')}
                        >
                            <Tablet className="w-4 h-4" />
                        </button>
                        <button 
                            className={`p-1.5 rounded ${devicePreview === 'mobile' ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-white'}`}
                            onClick={() => setDevicePreview('mobile')}
                        >
                            <Smartphone className="w-4 h-4" />
                        </button>
                    </div>

                    <div className="flex items-center gap-2 text-slate-400 border-x border-slate-700 px-4">
                        <button 
                            className={`hover:text-white ${historyIndex <= 0 ? 'opacity-30 cursor-not-allowed' : ''}`} 
                            onClick={undo}
                            disabled={historyIndex <= 0}
                        ><Undo2 className="w-4 h-4" /></button>
                        <button 
                            className={`hover:text-white ${historyIndex >= historyLength - 1 ? 'opacity-30 cursor-not-allowed' : ''}`} 
                            onClick={redo}
                            disabled={historyIndex >= historyLength - 1}
                        ><Redo2 className="w-4 h-4" /></button>
                    </div>

                    <div className="flex items-center gap-2 text-slate-400">
                        <button className="hover:text-white" onClick={() => setZoom(Math.max(25, zoom - 25))}><ZoomOut className="w-4 h-4" /></button>
                        <span className="w-12 text-center text-xs">{zoom}%</span>
                        <button className="hover:text-white" onClick={() => setZoom(Math.min(200, zoom + 25))}><ZoomIn className="w-4 h-4" /></button>
                    </div>

                    <div className="flex items-center gap-3">
                        <button className="flex items-center gap-2 text-slate-300 hover:text-white transition-colors">
                            <Play className="w-4 h-4" /> Preview
                        </button>
                        <button 
                            className="flex items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white px-3 py-1.5 rounded-md transition-colors font-medium"
                            onClick={handleSave}
                        >
                            <Save className="w-4 h-4" /> Save
                        </button>
                    </div>
                </div>
            </div>

            {/* Main Builder Area */}
            <div className="flex flex-1 overflow-hidden relative">
                
                {/* Left Icon Toolbar (48px) */}
                <div className="w-12 border-r border-slate-800 bg-slate-900 flex flex-col items-center py-4 shrink-0 z-40">
                    <div className="w-10 h-10 flex items-center justify-center rounded-md hover:bg-slate-800 cursor-pointer text-slate-400 hover:text-accent-primary transition-all">
                        <div className="w-5 h-5 border-2 border-current rounded-sm flex items-center justify-center">+</div>
                    </div>
                    {/* Additional icons would follow here */}
                </div>

                {/* Left Flyout (Component Palette) */}
                <div className="w-[280px] border-r border-slate-800 bg-slate-900 flex flex-col shrink-0">
                    <div className="h-12 border-b border-slate-800 flex items-center px-4 font-semibold shrink-0">
                        Components
                    </div>
                    <ComponentPalette />
                </div>

                {/* Center Canvas */}
                <Canvas />
                
                {/* Right Property Editor */}
                <RightPropertyEditor />

            </div>
        </div>
    </DndContext>
  );
}
