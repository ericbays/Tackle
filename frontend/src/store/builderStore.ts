import { create } from 'zustand';
import { v4 as uuidv4 } from 'uuid';
import { type LandingPageProject, type ComponentNode, type ComponentType } from '../types/builder';

interface BuilderState {
  project: LandingPageProject | null;
  history: LandingPageProject[];
  historyIndex: number;
  activePageId: string | null;
  selectedNodeIds: string[];
  hoveredNodeId: string | null;
  devicePreview: 'desktop' | 'tablet' | 'mobile';
  zoom: number;

  // Actions
  loadProject: (project: LandingPageProject) => void;
  setActivePage: (pageId: string) => void;
  setSelectedNodes: (nodeIds: string[]) => void;
  setHoveredNode: (nodeId: string | null) => void;
  setDevicePreview: (device: 'desktop' | 'tablet' | 'mobile') => void;
  setZoom: (zoom: number) => void;

  // Tree mutations
  addNode: (pageId: string, parentId: string, type: ComponentType, index?: number) => void;
  updateNode: (pageId: string, nodeId: string, updates: Partial<ComponentNode>) => void;
  removeNode: (pageId: string, nodeId: string) => void;
  moveNode: (pageId: string, nodeId: string, newParentId: string, newIndex: number) => void;
  
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
    if (node.id === nodeId) {
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

const getDefaultProps = (type: ComponentType) => {
  switch(type) {
    case 'Heading': return { text: 'Heading', level: 'h2' };
    case 'Paragraph': return { text: 'Lorem ipsum dolor sit amet...' };
    case 'Button': return { text: 'Button', type: 'button' };
    case 'Submit Button': return { text: 'Submit', type: 'submit' };
    case 'Text Input': return { placeholder: 'Enter text...' };
    case 'Email Input': return { placeholder: 'Enter email...' };
    case 'Password Input': return { placeholder: 'Enter password...' };
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

  loadProject: (project) => set({ 
    project,
    history: [project],
    historyIndex: 0,
    activePageId: project.pages.length > 0 ? project.pages[0].id : null 
  }),

  setActivePage: (pageId) => set({ activePageId: pageId, selectedNodeIds: [] }),
  setSelectedNodes: (nodeIds) => set({ selectedNodeIds: nodeIds }),
  setHoveredNode: (nodeId) => set({ hoveredNodeId: nodeId }),
  setDevicePreview: (device) => set({ devicePreview: device }),
  setZoom: (zoom) => set({ zoom }),

  addNode: (pageId, parentId, type, index) => set((state) => {
    if (!state.project) return state;

    const newNode: ComponentNode = {
      id: uuidv4(),
      type,
      isHidden: false,
      isLocked: false,
      props: getDefaultProps(type),
      style: {},
      children: []
    };

    const newPages = state.project.pages.map(page => {
      if (page.id !== pageId) return page;

      const addChildToParent = (node: ComponentNode): ComponentNode => {
        if (node.id === parentId) {
          const newChildren = [...node.children];
          if (index !== undefined) {
             newChildren.splice(index, 0, newNode);
          } else {
             newChildren.push(newNode);
          }
          return { ...node, children: newChildren };
        }
        return {
          ...node,
          children: node.children.map(addChildToParent)
        };
      };

      return {
        ...page,
        rootComponent: addChildToParent(page.rootComponent)
      };
    });

    const newProject = { ...state.project, pages: newPages };
    return pushHistory(state, newProject);
  }),

  updateNode: (pageId, nodeId, updates) => set((state) => {
    if (!state.project) return state;

    const newPages = state.project.pages.map(page => {
      if (page.id !== pageId) return page;

      return {
        ...page,
        rootComponent: findAndModifyNode([page.rootComponent], nodeId, (n) => ({ ...n, ...updates }))[0]
      };
    });

    const newProject = { ...state.project, pages: newPages };
    return pushHistory(state, newProject);
  }),

  removeNode: (pageId, nodeId) => set((state) => {
    if (!state.project) return state;
    // Logic to recursively filter out the node containing the ID
    const filterNode = (nodes: ComponentNode[]): ComponentNode[] => {
      return nodes.filter(n => n.id !== nodeId).map(n => ({
        ...n,
        children: filterNode(n.children)
      }));
    };

    const newPages = state.project.pages.map(page => {
      if (page.id !== pageId) return page;
      // We can't delete the root component, so we just filter its children downward
      if (page.rootComponent.id === nodeId) return page; 
      
      return {
        ...page,
        rootComponent: {
          ...page.rootComponent,
          children: filterNode(page.rootComponent.children)
        }
      };
    });

    const newProject = { ...state.project, pages: newPages };
    return {
      ...pushHistory(state, newProject),
      selectedNodeIds: state.selectedNodeIds.filter(id => id !== nodeId)
    };
  }),

  moveNode: (pageId, nodeId, newParentId, newIndex) => set((state) => {
    if (!state.project) return state;
    
    let targetNode: ComponentNode | null = null;
    let originalParentId: string | null = null;

    // 1. Find the node and its parent, cloning it so we can re-insert
    const findNodeAndParent = (nodes: ComponentNode[], parentId: string | null) => {
        for (const node of nodes) {
            if (node.id === nodeId) {
                targetNode = JSON.parse(JSON.stringify(node)); // Deep clone
                originalParentId = parentId;
                return;
            }
            if (node.children) {
                findNodeAndParent(node.children, node.id);
            }
        }
    };

    const rootNodes = state.project.pages.find(p => p.id === pageId)?.rootComponent;
    if (!rootNodes) return state;
    
    findNodeAndParent([rootNodes], null);

    if (!targetNode || !originalParentId) return state; // Can't move root or invalid node.

    // 2. Build the new tree by filtering out the old node, and slicing it into the new parent
    const rebuildTree = (node: ComponentNode): ComponentNode => {
        // Remove it from current children
        let newChildren = node.children.filter(c => c.id !== nodeId);

        // If this is the active destination, splice it in.
        if (node.id === newParentId) {
            newChildren.splice(newIndex, 0, targetNode!);
        }

        return {
            ...node,
            children: newChildren.map(rebuildTree)
        };
    };

    const newPages = state.project.pages.map(page => {
        if (page.id !== pageId) return page;
        return {
            ...page,
            rootComponent: rebuildTree(page.rootComponent)
        };
    });

    const newProject = { ...state.project, pages: newPages };
    return pushHistory(state, newProject);
  }),

  undo: () => set((state) => {
      if (state.historyIndex > 0) {
          const prevIndex = state.historyIndex - 1;
          return {
              project: state.history[prevIndex],
              historyIndex: prevIndex
          };
      }
      return state;
  }),

  redo: () => set((state) => {
      if (state.historyIndex < state.history.length - 1) {
          const nextIndex = state.historyIndex + 1;
          return {
              project: state.history[nextIndex],
              historyIndex: nextIndex
          };
      }
      return state;
  })
  };
});
