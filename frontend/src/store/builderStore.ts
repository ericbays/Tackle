import { create } from 'zustand';
import { v4 as uuidv4 } from 'uuid';
import { type LandingPageProject, type ComponentNode, type ComponentType, type PageNode } from '../types/builder';

interface BuilderState {
  project: LandingPageProject | null;
  history: LandingPageProject[];
  historyIndex: number;
  activePageId: string | null;
  selectedNodeIds: string[];
  hoveredNodeId: string | null;
  devicePreview: 'desktop' | 'tablet' | 'mobile';
  zoom: number;

  activeNativeDragItem: any | null;
  currentDropIndicator: { nodeId: string, position: 'top' | 'bottom' | 'left' | 'right' | 'inside' } | null;

  // Actions
  loadProject: (project: LandingPageProject) => void;
  setActivePage: (pageId: string) => void;
  addPage: () => void;
  removePage: (pageId: string) => void;
  setSelectedNodes: (nodeIds: string[]) => void;
  setHoveredNode: (nodeId: string | null) => void;
  setDevicePreview: (device: 'desktop' | 'tablet' | 'mobile') => void;
  setZoom: (zoom: number) => void;
  
  setActiveNativeDragItem: (item: any | null) => void;
  setCurrentDropIndicator: (indicator: { nodeId: string, position: 'top' | 'bottom' | 'left' | 'right' | 'inside' } | null) => void;

  // Tree mutations
  addNode: (pageId: string, targetNodeId: string, type: ComponentType, placement: 'top' | 'bottom' | 'left' | 'right' | 'inside') => void;
  updateNode: (pageId: string, nodeId: string, updates: Partial<ComponentNode>) => void;
  removeNode: (pageId: string, nodeId: string) => void;
  moveNode: (pageId: string, nodeId: string, targetNodeId: string, placement: 'top' | 'bottom' | 'left' | 'right' | 'inside') => void;
  
  // Page & Definition Mutations
  updatePage: (pageId: string, updates: Partial<PageNode>) => void;
  updateNavigation: (navigation: any[]) => void;
  
  // History Control
  undo: () => void;
  redo: () => void;
}

const findAndModifyNode = (
  nodes: ComponentNode[],
  nodeId: string,
  modifyFn: (node: ComponentNode) => ComponentNode
): ComponentNode[] => {
  return nodes.map(node => {
    if (node.component_id === nodeId) {
      return modifyFn(node);
    }
    if (node.children && node.children.length > 0) {
      return {
        ...node,
        children: findAndModifyNode(node.children, nodeId, modifyFn)
      };
    }
    return node;
  });
};

// Generates fallback properties for the exact backend types
const getDefaultProps = (type: ComponentType) => {
  switch(type) {
    case 'heading': return { text: 'Heading', level: 'h2' };
    case 'paragraph': return { text: 'Lorem ipsum dolor sit amet...' };
    case 'button': return { text: 'Button', type: 'button' };
    case 'submit_button': return { text: 'Submit', type: 'submit' };
    case 'text_input': return { placeholder: 'Enter text...' };
    case 'email_input': return { placeholder: 'Enter email...' };
    case 'password_input': return { placeholder: 'Enter password...' };
    case 'container': return { style: { padding: '20px', minHeight: '50px' } };
    default: return {};
  }
};

export const useBuilderStore = create<BuilderState>((set) => {
  const pushHistory = (state: BuilderState, newProject: LandingPageProject) => {
      const newHistory = state.history.slice(0, state.historyIndex + 1);
      newHistory.push(newProject);
      if (newHistory.length > 50) newHistory.shift();
      return {
          project: newProject,
          history: newHistory,
          historyIndex: newHistory.length - 1
      };
  };

  return {
      project: null,
      history: [],
      historyIndex: -1,
      activePageId: null,
      selectedNodeIds: [],
      hoveredNodeId: null,
      devicePreview: 'desktop',
      zoom: 100,
      activeNativeDragItem: null,
      currentDropIndicator: null,

      setActiveNativeDragItem: (item) => set({ activeNativeDragItem: item }),
      setCurrentDropIndicator: (indicator) => set({ currentDropIndicator: indicator }),

  loadProject: (project) => {
    // Safety check: ensure every page has at least a root-1 container so drag-and-drop collision detection works
    if (project && project.definition_json && project.definition_json.pages) {
        project.definition_json.pages.forEach(page => {
            if (!page.component_tree || page.component_tree.length === 0) {
                page.component_tree = [{
                    component_id: 'root-1',
                    type: 'container',
                    properties: { style: { padding: '2rem', minHeight: '100vh', backgroundColor: '#ffffff' } },
                    children: []
                }];
            }
        });
    }

    set({ 
        project, 
        activePageId: project?.definition_json?.pages?.[0]?.page_id || null,
        history: [project],
        historyIndex: 0
    });
  },

  setActivePage: (pageId) => set({ activePageId: pageId, selectedNodeIds: [] }),
  addPage: () => set((state) => {
    if (!state.project) return state;
    const newPageId = `page-${crypto.randomUUID().slice(0, 8)}`;
    const newPageName = `New Page ${state.project.definition_json.pages.length + 1}`;
    
    const newPage = {
        page_id: newPageId,
        name: newPageName,
        route: `/${newPageName.toLowerCase().replace(/\s+/g, '-')}`,
        title: newPageName,
        component_tree: [{
            component_id: `root-${crypto.randomUUID().slice(0, 8)}`,
            type: 'container' as const,
            properties: { style: { padding: '2rem', minHeight: '100vh', backgroundColor: '#ffffff' } },
            children: []
        }],
        page_styles: '',
        page_js: ''
    };

    const newProject = { 
        ...state.project, 
        definition_json: { 
            ...state.project.definition_json, 
            pages: [...state.project.definition_json.pages, newPage] 
        } 
    };
    return pushHistory(state, newProject);
  }),
  removePage: (pageId) => set((state) => {
    if (!state.project) return state;
    const currentPages = state.project.definition_json.pages;
    if (currentPages.length <= 1) return state; // Prevent deleting last page

    const newPages = currentPages.filter(p => p.page_id !== pageId);
    
    const newProject = { 
        ...state.project, 
        definition_json: { 
            ...state.project.definition_json, 
            pages: newPages 
        } 
    };

    const activeId = state.activePageId === pageId ? newPages[0].page_id : state.activePageId;

    const newState = pushHistory(state, newProject);
    return { ...newState, activePageId: activeId, selectedNodeIds: [] };
  }),
  updatePage: (pageId, updates) => set((state) => {
    if (!state.project) return state;
    const newPages = state.project.definition_json.pages.map(page => {
        if (page.page_id === pageId) {
            return { ...page, ...updates };
        }
        return page;
    });
    const newProject = { 
        ...state.project, 
        definition_json: { ...state.project.definition_json, pages: newPages }
    };
    return pushHistory(state, newProject);
  }),
  updateNavigation: (navigation) => set((state) => {
    if (!state.project) return state;
    const newProject = { 
        ...state.project, 
        definition_json: { ...state.project.definition_json, navigation }
    };
    return pushHistory(state, newProject);
  }),
  setSelectedNodes: (nodeIds) => set({ selectedNodeIds: nodeIds }),
  setHoveredNode: (nodeId) => set({ hoveredNodeId: nodeId }),
  setDevicePreview: (device) => set({ devicePreview: device }),
  setZoom: (zoom) => set({ zoom }),

  addNode: (pageId, targetNodeId, type, placement) => set((state) => {
    if (!state.project) return state;

    const newNode: ComponentNode = {
      component_id: uuidv4(),
      type,
      properties: getDefaultProps(type),
      children: []
    };

    const newPages = state.project.definition_json.pages.map((page: PageNode) => {
        if (page.page_id !== pageId) return page;

        const clonedTree = JSON.parse(JSON.stringify(page.component_tree || []));
        
        // Handle structural drop on canvas root
        if (targetNodeId === 'canvas-root' || targetNodeId === pageId) {
            if (placement === 'top' || placement === 'left') {
                clonedTree.unshift(newNode);
            } else {
                clonedTree.push(newNode);
            }
        } else {
            const insertNodeAtTarget = (nodes: ComponentNode[], parentNode?: ComponentNode): boolean => {
                for (let i = 0; i < nodes.length; i++) {
                    if (nodes[i].component_id === targetNodeId) {
                        if (placement === 'inside') {
                            nodes[i].children = nodes[i].children || [];
                            nodes[i].children.push(newNode);
                            return true;
                        }

                        const isParentRow = parentNode?.type === 'row' || parentNode?.properties?.style?.flexDirection === 'row';

                        if (placement === 'left' || placement === 'right') {
                            if (isParentRow) {
                                nodes.splice(placement === 'left' ? i : i + 1, 0, newNode);
                            } else {
                                // Wrap in a transparent flex row
                                const targetCopy = { ...nodes[i] };
                                const newRow: ComponentNode = {
                                    component_id: uuidv4(),
                                    type: 'row',
                                    properties: { style: { display: 'flex', flexDirection: 'row', gap: '16px', width: '100%' } },
                                    children: placement === 'left' ? [newNode, targetCopy] : [targetCopy, newNode]
                                };
                                nodes[i] = newRow;
                            }
                        } else if (placement === 'top' || placement === 'bottom') {
                            if (!isParentRow) {
                                nodes.splice(placement === 'top' ? i : i + 1, 0, newNode);
                            } else {
                                // Wrap in a flex column
                                const targetCopy = { ...nodes[i] };
                                const newCol: ComponentNode = {
                                    component_id: uuidv4(),
                                    type: 'column',
                                    properties: { style: { display: 'flex', flexDirection: 'column', gap: '16px', flex: '1' } },
                                    children: placement === 'top' ? [newNode, targetCopy] : [targetCopy, newNode]
                                };
                                nodes[i] = newCol;
                            }
                        }
                        return true;
                    }
                    if (nodes[i].children && nodes[i].children!.length > 0) {
                        if (insertNodeAtTarget(nodes[i].children!, nodes[i])) {
                            return true;
                        }
                    }
                }
                return false;
            };
            insertNodeAtTarget(clonedTree);
        }

        return { ...page, component_tree: clonedTree };
    });

    const newProject = { 
        ...state.project, 
        definition_json: { ...state.project.definition_json, pages: newPages }
    };
    return pushHistory(state, newProject);
  }),

  updateNode: (pageId, nodeId, updates) => set((state) => {
    if (!state.project) return state;

    const newPages = state.project.definition_json.pages.map(page => {
      if (page.page_id !== pageId) return page;

      return {
        ...page,
        component_tree: findAndModifyNode(page.component_tree, nodeId, (n) => ({ ...n, ...updates }))
      };
    });

    const newProject = { 
        ...state.project, 
        definition_json: { ...state.project.definition_json, pages: newPages }
    };
    return pushHistory(state, newProject);
  }),

  removeNode: (pageId, nodeId) => set((state) => {
    if (!state.project) return state;
    
    const filterNode = (nodes: ComponentNode[]): ComponentNode[] => {
      return nodes.filter(n => n.component_id !== nodeId).map(n => ({
        ...n,
        children: n.children ? filterNode(n.children) : []
      }));
    };

    const newPages = state.project.definition_json.pages.map(page => {
      if (page.page_id !== pageId) return page;
      
      return {
        ...page,
        component_tree: filterNode(page.component_tree)
      };
    });

    const newProject = { 
        ...state.project, 
        definition_json: { ...state.project.definition_json, pages: newPages }
    };
    return {
      ...pushHistory(state, newProject),
      selectedNodeIds: state.selectedNodeIds.filter(id => id !== nodeId)
    };
  }),

  moveNode: (pageId, nodeId, targetNodeId, placement) => set((state) => {
    if (!state.project) return state;
    
    let targetNode: ComponentNode | null = null;
    const findNode = (nodes: ComponentNode[]) => {
        for (const node of nodes) {
            if (node.component_id === nodeId) {
                targetNode = JSON.parse(JSON.stringify(node));
                return;
            }
            if (node.children) findNode(node.children);
        }
    };

    const activePage = state.project.definition_json.pages.find(p => p.page_id === pageId);
    if (!activePage) return state;
    
    findNode(activePage.component_tree);
    if (!targetNode) return state; 

    const newPages = state.project.definition_json.pages.map(page => {
        if (page.page_id !== pageId) return page;
        
        // 1. Remove the node deeply
        const filterNode = (nodes: ComponentNode[]): ComponentNode[] => {
            return nodes.filter(n => n.component_id !== nodeId).map(n => ({
                ...n,
                children: n.children ? filterNode(n.children) : []
            }));
        };
        const treeWithoutNode = filterNode(page.component_tree);

        // 2. Insert the targetNode at the new target
        const clonedTree = JSON.parse(JSON.stringify(treeWithoutNode));
        if (targetNodeId === 'canvas-root' || targetNodeId === pageId) {
            if (placement === 'top' || placement === 'left') {
                clonedTree.unshift(targetNode!);
            } else {
                clonedTree.push(targetNode!);
            }
        } else {
            const insertNodeAtTarget = (nodes: ComponentNode[], parentNode?: ComponentNode): boolean => {
                for (let i = 0; i < nodes.length; i++) {
                    if (nodes[i].component_id === targetNodeId) {
                        if (placement === 'inside') {
                            nodes[i].children = nodes[i].children || [];
                            nodes[i].children.push(targetNode!);
                            return true;
                        }

                        const isParentRow = parentNode?.type === 'row' || parentNode?.properties?.style?.flexDirection === 'row';

                        if (placement === 'left' || placement === 'right') {
                            if (isParentRow) {
                                nodes.splice(placement === 'left' ? i : i + 1, 0, targetNode!);
                            } else {
                                const targetCopy = { ...nodes[i] };
                                const newRow: ComponentNode = {
                                    component_id: uuidv4(),
                                    type: 'row',
                                    properties: { style: { display: 'flex', flexDirection: 'row', gap: '16px', width: '100%' } },
                                    children: placement === 'left' ? [targetNode!, targetCopy] : [targetCopy, targetNode!]
                                };
                                nodes[i] = newRow;
                            }
                        } else if (placement === 'top' || placement === 'bottom') {
                            if (!isParentRow) {
                                nodes.splice(placement === 'top' ? i : i + 1, 0, targetNode!);
                            } else {
                                const targetCopy = { ...nodes[i] };
                                const newCol: ComponentNode = {
                                    component_id: uuidv4(),
                                    type: 'column',
                                    properties: { style: { display: 'flex', flexDirection: 'column', gap: '16px', flex: '1' } },
                                    children: placement === 'top' ? [targetNode!, targetCopy] : [targetCopy, targetNode!]
                                };
                                nodes[i] = newCol;
                            }
                        }
                        return true;
                    }
                    if (nodes[i].children && nodes[i].children!.length > 0) {
                        if (insertNodeAtTarget(nodes[i].children!, nodes[i])) {
                            return true;
                        }
                    }
                }
                return false;
            };
            insertNodeAtTarget(clonedTree);
        }

        return { ...page, component_tree: clonedTree };
    });

    const newProject = { 
        ...state.project, 
        definition_json: { ...state.project.definition_json, pages: newPages }
    };
    return pushHistory(state, newProject);
  }),

  undo: () => set((state) => {
      if (state.historyIndex > 0) {
          const prevIndex = state.historyIndex - 1;
          return { project: state.history[prevIndex], historyIndex: prevIndex };
      }
      return state;
  }),

  redo: () => set((state) => {
      if (state.historyIndex < state.history.length - 1) {
          const nextIndex = state.historyIndex + 1;
          return { project: state.history[nextIndex], historyIndex: nextIndex };
      }
      return state;
  })
  };
});
