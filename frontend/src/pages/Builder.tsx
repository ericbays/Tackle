import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Monitor, Smartphone, Tablet, ZoomIn, ZoomOut, Save, Play, Loader2, Undo2, Redo2, Blocks, Network, Layers, ChevronDown, Radio } from 'lucide-react';
import { useBuilderStore } from '../store/builderStore';
import { Canvas } from '../components/builder/Canvas';
import { ComponentPalette } from '../components/builder/ComponentPalette';
import { LayersPanel } from '../components/builder/LayersPanel';
import { RightPropertyEditor } from '../components/builder/RightPropertyEditor';
import { PageManager } from '../components/builder/PageManager';
import { WorkflowEditor } from '../components/builder/WorkflowEditor';
import { useQueryClient } from '@tanstack/react-query';
import toast from 'react-hot-toast';
import { api } from '../services/api';

export default function Builder() {
  const queryClient = useQueryClient();
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
  const project = useBuilderStore(state => state.project);
  const activePageId = useBuilderStore(state => state.activePageId);

  const [isLoading, setIsLoading] = useState(true);
  const [activeDragItem, setActiveDragItem] = useState<{ id: string, type: string, isPaletteOriginal?: boolean } | null>(null);
  const [leftPanel, setLeftPanel] = useState<'components' | 'pages' | 'layers' | 'workflows'>('components');
  const navigate = useNavigate();

  const [lastSavedIndex, setLastSavedIndex] = useState(0);
  const [showPreviewModal, setShowPreviewModal] = useState(false);
  const [isEditingName, setIsEditingName] = useState(false);
  const [editedName, setEditedName] = useState('');
  
  const [activeDevUrl, setActiveDevUrl] = useState<string | null>(null);
  const [devServerStatus, setDevServerStatus] = useState<'offline' | 'starting' | 'online'>('offline');
  const [showDeployMenu, setShowDeployMenu] = useState(false);

  // Load from real DB API
  useEffect(() => {
    const fetchPage = async () => {
        setIsLoading(true);
        try {
            if (!id || id === 'new') {
                // Generate blank scaffolding tracking to DefinitionJSON Schema
                loadProject({
                    id: 'new',
                    name: 'New Landing Page',
                    definition_json: {
                        schema_version: 1,
                        pages: [{
                            page_id: 'page-1',
                            name: 'Home',
                            route: '/',
                            title: 'Home',
                            component_tree: [{
                                component_id: 'root-1',
                                type: 'container',
                                properties: { style: { padding: '0', minHeight: '100vh', backgroundColor: '#ffffff' } },
                                children: []
                            }],
                            page_styles: '',
                            page_js: ''
                        }],
                        global_styles: '',
                        global_js: '',
                        theme: {},
                        navigation: []
                    }
                });
            } else {
                // Hydrate from Postgres via Go API
                const res = await api.get(`/landing-pages/${id}`);
                const fetchedProject = res.data.data || res.data;
                if (fetchedProject && fetchedProject.definition_json) {
                    loadProject(fetchedProject);
                }
            }
            
            // Hydrate Dev Server Status
            if (id && id !== 'new') {
                try {
                    const statusRes = await api.get(`/landing-pages/${id}/dev-server/status`);
                    const statusData = statusRes.data?.data || statusRes.data;
                    if (statusData && statusData.status === 'online') {
                        setDevServerStatus('online');
                        setActiveDevUrl(statusData.url_b || statusData.url);
                    } else if (statusData && (statusData.status === 'starting' || statusData.status === 'pending')) {
                        setDevServerStatus('starting');
                    }
                } catch(e) {}
            }
            
        } catch (err) {
            console.error('Failed to parse landing page:', err);
        } finally {
            setIsLoading(false);
        }
    };

    fetchPage();
  }, [id, loadProject]);

  // Dev Server Sync - HMR Broadcast Pipeline
  // Pushes structural logic natively backwards into the React App Compiler without requiring Save + Build
  useEffect(() => {
    if (!project?.definition_json || id === 'new' || devServerStatus !== 'online') return;
    
    const timeoutId = setTimeout(async () => {
        try {
            // Push to the Go Context Engine via the Sync endpoint to trigger ESBuild Recompilation + Socket Broadcast
            const payload = {
                ...project.definition_json,
                pages: project.definition_json.pages.map((page: any) => ({
                    ...page,
                    component_tree: serializeComponentTree(page.component_tree || [])
                }))
            };
            await api.post(`/landing-pages/${id}/dev-server/sync`, payload);
        } catch (err) {
            // Intentionally silent; if standard DevServer isn't running it safely drops it
        }
    }, 600); // 600ms debounce
    
    return () => clearTimeout(timeoutId);
  }, [project?.definition_json, id, devServerStatus]);

  // Dev Server Polling during Startup
  useEffect(() => {
      let interval: NodeJS.Timeout;
      if (devServerStatus === 'starting' && id && id !== 'new') {
          interval = setInterval(async () => {
              try {
                  const res = await api.get(`/landing-pages/${id}/dev-server/status`);
                  const data = res.data?.data || res.data;
                  if (data?.status === 'online') {
                      setDevServerStatus('online');
                      setActiveDevUrl(data.url_b || data.url);
                      toast.success('Development Server Registered & Online!', { id: 'dev-deploy' });
                      clearInterval(interval);
                      
                      // Auto-open live preview once online
                      if (data.url_b || data.url) {
                          window.open(data.url_b || data.url, '_blank');
                      }
                  } else if (data?.status === 'offline') {
                      setDevServerStatus('offline');
                      toast.error('Development Server failed to start', { id: 'dev-deploy' });
                      clearInterval(interval);
                  }
              } catch (err) {
                  // Ignore to keep polling
              }
          }, 2000);
      }
      return () => {
          if (interval) clearInterval(interval);
      };
  }, [devServerStatus, id]);

  // Serializer to safely map UI properties (React 'style' object, 'text') to Backend Compiler format ('inline_style', 'content')
  const serializeComponentTree = (nodes: any[]): any[] => {
      return nodes.map(node => {
          const props = { ...node.properties };
          
          if (props.text !== undefined) {
              props.content = props.text;
          }
          if (props.style) {
              const styleString = Object.entries(props.style).map(([k, v]) => {
                  const kebabKey = k.replace(/([A-Z])/g, "-$1").toLowerCase();
                  return `${kebabKey}: ${v}`;
              }).join('; ');
              props.inline_style = styleString;
          }

          return {
              ...node,
              properties: props,
              children: node.children ? serializeComponentTree(node.children) : []
          };
      });
  };

  const handleSave = async (close = false): Promise<string | null> => {
      // Must use getState() for the literal current mutable store if we don't want to rely on the hook closure safely during async
      const currentProject = useBuilderStore.getState().project;
      if (!currentProject) return null;
      try {
          // Prepare DB compliant payload by compiling UI bindings
          const serializedDefinition = {
              ...currentProject.definition_json,
              pages: currentProject.definition_json.pages.map((page: any) => ({
                  ...page,
                  component_tree: serializeComponentTree(page.component_tree || [])
              }))
          };

          if (id === 'new') {
              const payload = {
                  name: currentProject.name || 'New Landing Page',
                  description: currentProject.description || '',
                  definition_json: serializedDefinition
              };
              const res = await api.post(`/landing-pages`, payload);
              const newId = res.data?.data?.id || res.data?.id;
              
              setLastSavedIndex(useBuilderStore.getState().historyIndex);
              
              if (close) {
                  await queryClient.invalidateQueries({ queryKey: ['landing-pages'] });
                  toast.success('Application created successfully!');
                  navigate('/landing-pages');
              } else {
                  navigate(`/builder/${newId}`, { replace: true });
                  toast.success('Application created successfully!');
              }
              return newId;
          } else {
              const payload = {
                  name: currentProject.name,
                  description: currentProject.description,
                  definition_json: serializedDefinition
              };
              await api.put(`/landing-pages/${currentProject.id}`, payload);
              setLastSavedIndex(useBuilderStore.getState().historyIndex);
              if (close) {
                  await queryClient.invalidateQueries({ queryKey: ['landing-pages'] });
                  toast.success('Application saved successfully!');
                  navigate('/landing-pages');
              } else {
                  toast.success('Application saved successfully!');
              }
              return currentProject.id;
          }
      } catch (error) {
          console.error('Failed to save application', error);
          toast.error('Failed to save application. Ensure your backend is running.');
          return null;
      }
  };

  const openPreviewWithDisclaimer = () => {
      if (id === 'new' || historyIndex !== lastSavedIndex) {
          setShowPreviewModal(true);
      } else {
          executePreview(id as string);
      }
  };

  const executePreview = async (projectId: string) => {
      setDevServerStatus('starting');
      toast.loading('Starting Development Deployment...', { id: 'dev-deploy' });
      try {
          // Fire and forget; the polling useEffect handles UX transitions
          await api.post(`/landing-pages/${projectId}/dev-server/start`);
      } catch (err) {
          console.error(err);
          setDevServerStatus('offline');
          toast.error('Failed to start development deployment', { id: 'dev-deploy' });
      }
  };

  const stopDevServer = async () => {
      if (id === 'new') return;
      const toastId = toast.loading('Stopping Development Server...');
      try {
          await api.post(`/landing-pages/${id}/dev-server/stop`);
          setDevServerStatus('offline');
          setActiveDevUrl(null);
          toast.success('Development Server Stopped', { id: toastId });
      } catch (err) {
          console.error(err);
          toast.error('Failed to stop development server', { id: toastId });
      }
  };

  const handlePreviewCompile = async () => {
      const savedId = await handleSave(false);
      if (!savedId) return;
      executePreview(savedId);
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
        <div className="flex flex-col h-screen w-screen bg-slate-900 text-slate-200 overflow-hidden text-sm">
            
            {/* Top Toolbar */}
            <div className="h-14 border-b border-slate-800 bg-slate-900 flex items-center justify-between px-4 shrink-0 z-50">
                <div className="flex items-center gap-4">
                    <button onClick={() => navigate('/landing-pages')} className="text-slate-400 hover:text-white transition-colors" title="Back to Dashboard">
                        <ArrowLeft className="w-5 h-5" />
                    </button>
                    {isEditingName ? (
                        <input 
                            autoFocus
                            className="bg-slate-800 text-slate-200 px-2 py-1 rounded border border-blue-500 outline-none w-48 text-sm font-semibold"
                            value={editedName}
                            onChange={(e) => setEditedName(e.target.value)}
                            onBlur={() => {
                                setIsEditingName(false);
                                if (project && editedName.trim() !== '') {
                                    useBuilderStore.getState().loadProject({ ...project, name: editedName.trim() });
                                }
                            }}
                            onKeyDown={(e) => {
                                if (e.key === 'Enter') {
                                    setIsEditingName(false);
                                    if (project && editedName.trim() !== '') {
                                        useBuilderStore.getState().loadProject({ ...project, name: editedName.trim() });
                                    }
                                } else if (e.key === 'Escape') {
                                    setIsEditingName(false);
                                }
                            }}
                        />
                    ) : (
                        <div 
                            className="font-semibold text-slate-200 cursor-pointer hover:bg-slate-800 px-2 py-1 rounded flex items-center gap-1"
                            onClick={() => {
                                setEditedName(project?.name || 'New Landing Page');
                                setIsEditingName(true);
                            }}
                            title="Click to rename"
                        >
                            {project?.name || 'New Landing Page'} <span className="text-xs opacity-50 ml-1">▾</span>
                        </div>
                    )}
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
                        {devServerStatus === 'online' && activeDevUrl && (
                            <button 
                                onClick={() => window.open(activeDevUrl, '_blank')}
                                className="flex items-center gap-1.5 text-xs font-semibold text-emerald-400 bg-emerald-400/10 hover:bg-emerald-400/20 px-2 py-1 rounded-full border border-emerald-400/20 transition-colors mr-1 cursor-pointer"
                                title="Click to view live development server"
                            >
                                <Radio className="w-3.5 h-3.5 animate-pulse" />
                                Live
                            </button>
                        )}
                        
                        <div className="relative flex items-center">
                            <button 
                                className={`flex items-center gap-2 transition-colors px-2 py-1.5 rounded-l-md border border-transparent ${devServerStatus === 'starting' ? 'text-blue-400' : 'text-slate-300 hover:text-white hover:bg-slate-800'}`}
                                onClick={openPreviewWithDisclaimer}
                                disabled={devServerStatus === 'starting'}
                            >
                                {devServerStatus === 'starting' ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
                                {devServerStatus === 'online' ? 'Redeploy Development' : 'Deploy Development'}
                            </button>
                            
                            {devServerStatus === 'online' && (
                                <>
                                    <button 
                                        className={`flex items-center justify-center px-1 py-1.5 rounded-r-md transition-colors border-l border-slate-700 h-full ${showDeployMenu ? 'bg-slate-800 text-white' : 'text-slate-300 hover:text-white hover:bg-slate-800'}`}
                                        onClick={() => setShowDeployMenu(!showDeployMenu)}
                                    >
                                        <ChevronDown className="w-4 h-4" />
                                    </button>
                                    
                                    {showDeployMenu && (
                                        <div className="absolute top-full right-0 mt-1 w-48 bg-slate-800 border border-slate-700 rounded-md shadow-xl overflow-hidden z-50">
                                            <button 
                                                className="w-full text-left px-4 py-2 bg-slate-800 hover:bg-red-900/30 text-red-400 hover:text-red-300 transition-colors text-sm font-medium"
                                                onClick={() => {
                                                    stopDevServer();
                                                    setShowDeployMenu(false);
                                                }}
                                            >
                                                Stop Development
                                            </button>
                                        </div>
                                    )}
                                </>
                            )}
                        </div>

                        <button 
                            className="bg-slate-700 hover:bg-slate-600 text-white px-4 py-2 rounded-md font-medium transition-colors border border-slate-600 shadow-sm ml-2"
                            onClick={() => handleSave(false)}
                        >
                            Save
                        </button>
                        <button 
                            className="flex items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white px-4 py-2 rounded-md font-medium transition-colors shadow-sm"
                            onClick={() => handleSave(true)}
                        >
                            <Save className="w-4 h-4" /> Save & Close
                        </button>
                    </div>
                </div>
            </div>

            {/* Main Builder Area */}
            <div className="flex flex-1 overflow-hidden relative">
                
                {/* Left Icon Toolbar (48px) */}
                <div className="w-12 border-r border-slate-800 bg-slate-900 flex flex-col items-center py-4 shrink-0 z-40 gap-2">
                    <button 
                        onClick={() => setLeftPanel('components')}
                        className={`w-10 h-10 flex items-center justify-center rounded-md cursor-pointer transition-all ${leftPanel === 'components' ? 'bg-slate-800 text-accent-primary' : 'hover:bg-slate-800 text-slate-400 hover:text-white'}`}
                        title="Components"
                    >
                        <Blocks className="w-5 h-5" />
                    </button>
                    <button 
                        onClick={() => setLeftPanel('layers')}
                        className={`w-10 h-10 flex items-center justify-center rounded-md cursor-pointer transition-all ${leftPanel === 'layers' ? 'bg-slate-800 text-accent-primary' : 'hover:bg-slate-800 text-slate-400 hover:text-white'}`}
                        title="DOM Tree (Layers)"
                    >
                        <Network className="w-5 h-5" />
                    </button>
                    <button 
                        onClick={() => setLeftPanel('pages')}
                        className={`w-10 h-10 flex items-center justify-center rounded-md cursor-pointer transition-all ${leftPanel === 'pages' ? 'bg-slate-800 text-blue-400' : 'hover:bg-slate-800 text-slate-400 hover:text-white'}`}
                        title="Pages & Routing"
                    >
                        <Layers className="w-5 h-5" />
                    </button>
                    <button 
                        onClick={() => setLeftPanel('workflows')}
                        className={`w-10 h-10 flex items-center justify-center rounded-md cursor-pointer transition-all mt-4 border-t border-slate-800 pt-2 ${leftPanel === 'workflows' ? 'bg-slate-800 text-blue-400' : 'hover:bg-slate-800 text-slate-400 hover:text-white'}`}
                        title="Global Event Workflows"
                    >
                        <Network className="w-5 h-5" />
                    </button>
                </div>

                {/* Left Flyout */}
                <div className="w-[280px] border-r border-slate-800 bg-slate-900 flex flex-col shrink-0 overflow-hidden">
                    <div className="h-12 border-b border-slate-800 flex items-center px-4 font-semibold shrink-0 uppercase tracking-wide text-[10px] text-slate-500">
                        {leftPanel === 'components' && 'UI Component Palette'}
                        {leftPanel === 'pages' && 'Pages & Routes Manager'}
                        {leftPanel === 'layers' && 'AST Canvas Navigator'}
                        {leftPanel === 'workflows' && 'Global Event Workflows'}
                    </div>
                    {leftPanel === 'components' && <ComponentPalette />}
                    {leftPanel === 'pages' && <PageManager />}
                    {leftPanel === 'layers' && <LayersPanel />}
                    {leftPanel === 'workflows' && <WorkflowEditor />}
                </div>

                {/* Center Canvas */}
                <Canvas />
                
                <RightPropertyEditor />

            </div>
            
            {/* Preview Save Disclaimer Modal */}
            {showPreviewModal && (
                <div className="fixed inset-0 bg-slate-950/80 z-[100] flex items-center justify-center backdrop-blur-sm">
                    <div className="bg-slate-900 border border-slate-800 rounded-lg shadow-2xl p-6 w-[450px]">
                        <h3 className="text-lg font-semibold text-white mb-2">Unsaved Changes Detected</h3>
                        <p className="text-slate-400 mb-6 leading-relaxed">
                            The visual compiler engine generates previews against the database state. 
                            We must commit your local changes to securely generate the preview payload.
                        </p>
                        <div className="flex justify-end gap-3">
                            <button 
                                className="px-4 py-2 rounded text-slate-400 hover:text-slate-200 transition-colors"
                                onClick={() => setShowPreviewModal(false)}
                            >
                                Cancel
                            </button>
                            <button 
                                className="flex flex-row items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white px-5 py-2 rounded font-medium transition-colors border-none"
                                onClick={() => {
                                    setShowPreviewModal(false);
                                    handlePreviewCompile();
                                }}
                            >
                                <Save className="w-4 h-4" /> Save & Preview
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
