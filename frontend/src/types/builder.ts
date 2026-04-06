export type ComponentType = 
  | 'Container' 
  | 'Grid' 
  | 'Columns' 
  | 'Column' 
  | 'Spacer' 
  | 'Divider'
  | 'Navbar'
  | 'Footer'
  | 'Breadcrumb'
  | 'Heading'
  | 'Paragraph'
  | 'Rich Text'
  | 'Image'
  | 'Video Embed'
  | 'Form Container'
  | 'Text Input'
  | 'Email Input'
  | 'Password Input'
  | 'Select'
  | 'Checkbox'
  | 'Radio Group'
  | 'Submit Button'
  | 'Button'
  | 'Link'
  | 'Tab Group'
  | 'Accordion'
  | 'Logo Placeholder'
  | 'Hero Banner';

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
  // Fallback for custom injected styles. Safe as a Record<string, string> mapping to React.CSSProperties
  [key: string]: any;
}

export interface ComponentNode {
  id: string; // unique UUID per instance
  type: ComponentType;
  label?: string; // custom name if renamed in Layers
  isHidden: boolean;
  isLocked: boolean;
  
  // Tab 1: Content properties (context-sensitive)
  props: Record<string, any>;
  
  // Tab 2: Style properties
  style: StyleProperties;
  
  // Tab 3: Advanced
  customCss?: string;
  customJs?: string;
  
  children: ComponentNode[];
}

export interface PageNode {
  id: string;
  name: string;
  isStartPage: boolean;
  rootComponent: ComponentNode; // A single invisible root container
}

export interface LandingPageProject {
  id: string;
  name: string;
  pages: PageNode[];
  globalCss: string;
  globalJs: string;
  themeId?: string;
}
