export type ComponentType = string;

// The backend strictly parses the AST schema according to standard properties mapping.
export interface StyleProperties {
  display?: string;
  position?: string;
  overflow?: string;
  width?: string;
  height?: string;
  margin?: string;
  padding?: string;
  fontFamily?: string;
  fontWeight?: string;
  fontSize?: string;
  lineHeight?: string;
  color?: string;
  textAlign?: string;
  textTransform?: string;
  backgroundColor?: string;
  opacity?: number;
  [key: string]: any;
}

export interface ComponentNode {
  component_id: string; // unique UUID per instance, mapped from Go's requirement
  type: ComponentType; // Must be lowercase e.g., 'heading', 'container', 'form'
  
  // Properties contains BOTH attributes and inline styles for the backend to interpret natively
  properties: {
      text?: string;
      src?: string;
      alt?: string;
      placeholder?: string;
      level?: 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6';
      href?: string;
      style?: StyleProperties;
      hover_style?: StyleProperties;
      active_style?: StyleProperties;
      options?: { label: string; value: string }[];
      [key: string]: any;
  };
  
  event_bindings?: any[];
  children?: ComponentNode[];
}

export interface PageNode {
  page_id: string;
  name: string;
  route: string;
  title: string;
  favicon?: string;
  meta_tags?: any[];
  component_tree: ComponentNode[]; // Replaces single rootComponent to match Go AST
  page_styles?: string;
  page_js?: string;
}

export interface LandingPageDefinitionJSON {
  schema_version: number;
  pages: PageNode[];
  global_styles: string;
  global_js: string;
  theme: Record<string, any>;
  navigation: any[];
}

export interface LandingPageProject {
  id: string;
  name: string;
  description?: string;
  definition_json: LandingPageDefinitionJSON;
}
