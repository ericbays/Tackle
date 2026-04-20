import React, { useState } from 'react';
import { useBuilderStore } from '../../store/builderStore';
import { type ComponentNode } from '../../types/builder';
import { ChevronDown, ChevronRight, Trash2, Info } from 'lucide-react';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { Button } from '../ui/Button';

const Accordion = ({ title, children, defaultOpen = false }: { title: string, children: React.ReactNode, defaultOpen?: boolean }) => {
    const [isOpen, setIsOpen] = useState(defaultOpen);
    return (
        <div className="border border-slate-800 rounded bg-slate-900/50 mb-3 overflow-hidden">
            <Button variant="outline" 
                onClick={ () => setIsOpen(!isOpen)} 
                className="w-full flex items-center justify-between p-3 bg-slate-800/50 hover:bg-slate-800 transition-colors text-slate-300 text-xs font-semibold uppercase tracking-wider"
            >
                {title}
                {isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
            </Button>
            {isOpen && <div className="p-3 space-y-4">{children}</div>}
        </div>
    );
};

export const RightPropertyEditor = () => {
    const [activeTab, setActiveTab] = useState<'content' | 'style' | 'advanced'>('content');
    
    const project = useBuilderStore(state => state.project);
    const activePageId = useBuilderStore(state => state.activePageId);
    const selectedNodeIds = useBuilderStore(state => state.selectedNodeIds);
    const updateNode = useBuilderStore(state => state.updateNode);
    const removeNode = useBuilderStore(state => state.removeNode);

    const activePage = project?.definition_json?.pages.find(p => p.page_id === activePageId);

    const getSelectedNode = (nodes: ComponentNode[], targetId: string): ComponentNode | null => {
        for (const n of nodes) {
            if (n.component_id === targetId) return n;
            if (n.children) {
                const found = getSelectedNode(n.children, targetId);
                if (found) return found;
            }
        }
        return null;
    };

    let selectedNode: ComponentNode | null = null;
    if (activePage && selectedNodeIds.length === 1 && activePage.component_tree) {
        selectedNode = getSelectedNode(activePage.component_tree, selectedNodeIds[0]);
    }

    const setSelectedNodes = useBuilderStore(state => state.setSelectedNodes);

    if (!selectedNode) {
        return (
            <div className="w-[340px] border-l border-slate-800 bg-slate-900 flex flex-col shrink-0 items-center justify-center p-8 text-center">
                <div className="text-slate-500 text-sm max-w-[200px] mb-6">
                    Select a component in the canvas to edit its properties.
                </div>
                <div className="border-t border-slate-800/50 pt-6 w-full flex flex-col items-center">
                    <p className="text-slate-500 text-[10px] uppercase mb-3 font-semibold tracking-wider">Page Background Settings</p>
                    <Button variant="outline" 
                        onClick={ () => setSelectedNodes(['root-1'])}
                        className="bg-slate-800 border border-slate-700 hover:bg-slate-700 text-slate-300 font-medium px-4 py-2 rounded transition text-xs w-full shadow-sm"
                    >
                        Edit Root Container
                    </Button>
                    <p className="text-slate-600 text-[10px] mt-3">Use this to remove the default padding or change the page background color.</p>
                </div>
            </div>
        );
    }

    const handlePropChange = (key: string, value: any) => {
        if (!activePageId) return;
        updateNode(activePageId, selectedNode!.component_id, {
            properties: { ...selectedNode!.properties, [key]: value }
        });
    };

    const handleStyleChange = (key: string, value: any) => {
        if (!activePageId) return;

        let sanitizedValue = value;
        const dimensionKeys = ['paddingTop', 'paddingBottom', 'paddingLeft', 'paddingRight', 'marginTop', 'marginBottom', 'marginLeft', 'marginRight', 'width', 'height', 'minWidth', 'minHeight', 'gap', 'borderWidth', 'borderRadius', 'fontSize'];
        
        if (typeof value === 'string' && dimensionKeys.includes(key)) {
            if (/^\d+$/.test(value.trim())) {
                sanitizedValue = `${value.trim()}px`;
            }
        }

        updateNode(activePageId, selectedNode!.component_id, {
            properties: { 
                ...selectedNode!.properties, 
                style: { ...(selectedNode!.properties?.style || {}), [key]: sanitizedValue } 
            }
        });
    };

    const styles = selectedNode.properties?.style || {};
    const props = selectedNode.properties || {};

    const InputRow = ({ label, value, onChange, placeholder = "", type = "text", helpText }: any) => {
        const [localValue, setLocalValue] = React.useState(value || '');

        React.useEffect(() => {
            setLocalValue(value || '');
        }, [value]);

        const handleBlur = () => {
            if (localValue !== (value || '')) {
                onChange(localValue);
            }
        };

        const handleKeyDown = (e: React.KeyboardEvent) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                handleBlur();
            }
        };

        return (
            <div className="space-y-1">
                <div className="flex justify-between items-center">
                    <label className="text-[10px] text-slate-500 uppercase tracking-widest">{label}</label>
                    {helpText && (
                        <div className="relative group/tooltip flex items-center">
                            <Info size={12} className="text-slate-500 cursor-help hover:text-blue-400 transition-colors" />
                            <div className="fixed hidden group-hover/tooltip:block bottom-[50px] right-[50px] w-72 p-4 bg-slate-800 border border-slate-700 text-slate-200 text-xs leading-relaxed rounded-lg shadow-2xl z-[100] text-left normal-case tracking-normal">
                                {helpText}
                            </div>
                        </div>
                    )}
                </div>
                <Input 
                    type={type}
                    
                    value={localValue}
                    onChange={e => setLocalValue(e.target.value)}
                    onBlur={handleBlur}
                    onKeyDown={handleKeyDown}
                    placeholder={placeholder}
                />
            </div>
        );
    };

    const SelectRow = ({ label, value, onChange, options, helpText }: any) => (
        <div className="space-y-1">
            <div className="flex justify-between items-center">
                <label className="text-[10px] text-slate-500 uppercase tracking-widest">{label}</label>
                {helpText && (
                    <div className="relative group/tooltip flex items-center">
                        <Info size={12} className="text-slate-500 cursor-help hover:text-blue-400 transition-colors" />
                        <div className="fixed hidden group-hover/tooltip:block bottom-[50px] right-[50px] w-72 p-4 bg-slate-800 border border-slate-700 text-slate-200 text-xs leading-relaxed rounded-lg shadow-2xl z-[100] text-left normal-case tracking-normal">
                            {helpText}
                        </div>
                    </div>
                )}
            </div>
            <Select 
                
                value={value || ''}
                onChange={e => onChange(e.target.value)}
            >
                {options.map((opt: any) => (
                    <option key={opt.value} value={opt.value}>{opt.label}</option>
                ))}
            </Select>
        </div>
    );

    const isInput = ['text_input', 'email_input', 'password_input', 'textarea', 'checkbox', 'radio', 'select'].includes(selectedNode.type);
    const isButton = ['button', 'submit_button', 'link'].includes(selectedNode.type);

    return (
        <div className="w-[340px] border-l border-slate-800 bg-slate-900 flex flex-col shrink-0 max-h-[calc(100vh-60px)] overflow-y-auto">
            {/* Tabs */}
            <div className="flex border-b border-slate-800 shrink-0 sticky top-0 bg-slate-900/95 backdrop-blur-sm z-10">
                <Button variant="ghost" size="sm" 
                    className={`flex-1 py-3.5 text-center text-xs font-semibold uppercase tracking-wider border-b-2 transition-colors ${activeTab === 'content' ? 'border-blue-500 text-blue-400' : 'border-transparent text-slate-400 hover:text-slate-200'}`}
                    onClick={ () => setActiveTab('content')}
                >
                    Content
                </Button>
                <Button variant="ghost" size="sm" 
                    className={`flex-1 py-3.5 text-center text-xs font-semibold uppercase tracking-wider border-b-2 transition-colors ${activeTab === 'style' ? 'border-blue-500 text-blue-400' : 'border-transparent text-slate-400 hover:text-slate-200'}`}
                    onClick={ () => setActiveTab('style')}
                >
                    Style
                </Button>
                <Button variant="ghost" size="sm" 
                    className={`flex-1 py-3.5 text-center text-xs font-semibold uppercase tracking-wider border-b-2 transition-colors ${activeTab === 'advanced' ? 'border-blue-500 text-blue-400' : 'border-transparent text-slate-400 hover:text-slate-200'}`}
                    onClick={ () => setActiveTab('advanced')}
                >
                    Advanced
                </Button>
            </div>

            <div className="p-4 bg-slate-900 border-b border-slate-800 shadow-sm flex items-center justify-between">
                <div>
                    <div className="text-[10px] text-slate-500 uppercase tracking-widest font-semibold">Component</div>
                    <div className="text-slate-200 font-medium capitalize mt-0.5">{selectedNode.type.replace('_', ' ')}</div>
                </div>
                {selectedNode.component_id !== 'root-1' && (
                    <Button variant="outline" 
                        onClick={ () => removeNode(activePageId!, selectedNode!.component_id)}
                        className="p-1.5 text-slate-500 hover:text-red-400 hover:bg-red-400/10 rounded transition-colors"
                        title="Delete component"
                    >
                        <Trash2 size={16} />
                    </Button>
                )}
            </div>

            <div className="p-4">
                {/* Content Editor */}
                {activeTab === 'content' && (
                    <div className="space-y-4">
                        <div className="pb-4 border-b border-slate-800">
                            <InputRow 
                                label="Component Canvas Name" 
                                value={props.component_name} 
                                onChange={(v: string) => handlePropChange('component_name', v)} 
                                placeholder={selectedNode.type} 
                                helpText="An internal identifier used purely to help you identify this component in the canvas layers list. Does not affect the final website code."
                            />
                        </div>
                        {['heading', 'paragraph', 'text', 'button', 'submit_button'].includes(selectedNode.type) && (
                            <div className="space-y-1">
                                <label className="text-[10px] font-medium text-slate-500 uppercase tracking-widest">Text Content</label>
                                <textarea 
                                    className="w-full bg-slate-950 border border-slate-700/80 rounded p-2.5 text-sm text-slate-200 focus:outline-none focus:border-blue-500 transition-colors"
                                    value={props.text || ''}
                                    onChange={(e) => handlePropChange('text', e.target.value)}
                                    rows={4}
                                />
                            </div>
                        )}
                        
                        {selectedNode.type === 'heading' && (
                            <SelectRow 
                                label="Heading Scope" 
                                value={props.level || 'h2'} 
                                onChange={(val: string) => handlePropChange('level', val)}
                                helpText="HTML heading level (H1-H4) used for SEO and document structure. H1 should only be used once per page."
                                options={[
                                    {label: 'H1 - Page Title', value: 'h1'},
                                    {label: 'H2 - Subtitle', value: 'h2'},
                                    {label: 'H3 - Section Header', value: 'h3'},
                                    {label: 'H4 - Small Header', value: 'h4'},
                                ]}
                            />
                        )}

                        {isInput && (
                            <div className="space-y-4">
                                <InputRow label="Input Name (Required)" value={props.name} onChange={(v: string) => handlePropChange('name', v)} placeholder="username" helpText="The technical field name used when sending data to the server (e.g. 'email', 'password'). Must be unique in the form." />
                                {['text_input', 'email_input', 'password_input', 'textarea'].includes(selectedNode.type) && (
                                    <InputRow label="Placeholder Text" value={props.placeholder} onChange={(v: string) => handlePropChange('placeholder', v)} placeholder="Enter details..." helpText="Faint hint text displayed inside an empty input field before a user types." />
                                )}
                                <InputRow label="Label Text" value={props.label_text} onChange={(v: string) => handlePropChange('label_text', v)} placeholder="" helpText="Optional text rendered directly above the input field indicating what the user should enter." />
                            </div>
                        )}

                        {selectedNode.type === 'image' && (
                            <div className="space-y-4">
                                <InputRow label="Image Source (URL)" value={props.src} onChange={(v: string) => handlePropChange('src', v)} placeholder="https://..." helpText="Absolute or relative URL linking to the image asset." />
                                <InputRow label="Alt Text" value={props.alt} onChange={(v: string) => handlePropChange('alt', v)} placeholder="Description for accessibility" helpText="Description of the image for screen readers and search engines. Crucial for accessibility." />
                            </div>
                        )}
                    </div>
                )}

                {/* Style Editor */}
                {activeTab === 'style' && (
                    <div className="space-y-1">
                        <Accordion title="Layout & Flexbox" defaultOpen={true}>
                            <SelectRow 
                                label="Display" 
                                value={styles.display || 'block'} 
                                onChange={(val: string) => handleStyleChange('display', val)}
                                helpText="Determines whether the element stacks natively, behaves inline, or unlocks advanced responsive Flexbox grids."
                                options={[
                                    {label: 'Block', value: 'block'},
                                    {label: 'Flex (Responsive)', value: 'flex'},
                                    {label: 'Inline-Block', value: 'inline-block'},
                                    {label: 'None', value: 'none'}
                                ]}
                            />
                            {styles.display === 'flex' && (
                                <>
                                    <SelectRow 
                                        label="Flex Direction" 
                                        value={styles.flexDirection || 'row'} 
                                        onChange={(val: string) => handleStyleChange('flexDirection', val)}
                                        helpText="Defines the primary axis for children: flow left-to-right (Row) or top-to-bottom (Column)."
                                        options={[
                                            {label: 'Row (Horizontal)', value: 'row'},
                                            {label: 'Column (Vertical)', value: 'column'}
                                        ]}
                                    />
                                    <SelectRow 
                                        label="Align Items" 
                                        value={styles.alignItems || 'stretch'} 
                                        onChange={(val: string) => handleStyleChange('alignItems', val)}
                                        helpText="Aligns children along the secondary (cross) axis. E.g. Vertically centering items in a row."
                                        options={[
                                            {label: 'Stretch', value: 'stretch'},
                                            {label: 'Flex Start (Start)', value: 'flex-start'},
                                            {label: 'Center', value: 'center'},
                                            {label: 'Flex End (End)', value: 'flex-end'}
                                        ]}
                                    />
                                    <SelectRow 
                                        label="Justify Content" 
                                        value={styles.justifyContent || 'flex-start'} 
                                        onChange={(val: string) => handleStyleChange('justifyContent', val)}
                                        helpText="Aligns children along the primary axis. E.g. Distributing specific horizontal spacing in a Row."
                                        options={[
                                            {label: 'Flex Start', value: 'flex-start'},
                                            {label: 'Center', value: 'center'},
                                            {label: 'Flex End', value: 'flex-end'},
                                            {label: 'Space Between', value: 'space-between'},
                                            {label: 'Space Evenly', value: 'space-evenly'}
                                        ]}
                                    />
                                    <InputRow label="Flex Gap (px/rem)" value={styles.gap} onChange={(v: string) => handleStyleChange('gap', v)} placeholder="e.g. 16px" helpText="The strict spacing placed evenly between all children of this container (e.g. '16px' or '1rem')." />
                                </>
                            )}
                            <div className="pt-2 border-t border-slate-800">
                                <SelectRow 
                                    label="Flex Sizing (Flex Grow)" 
                                    value={styles.flex || '0 1 auto'} 
                                    onChange={(val: string) => handleStyleChange('flex', val)}
                                    helpText="Whether this element itself should dynamically stretch to fill remaining empty space inside its parent."
                                    options={[
                                        {label: 'Default (0 1 auto)', value: '0 1 auto'},
                                        {label: 'Fill Space (Flex: 1)', value: '1'},
                                        {label: 'None', value: 'none'}
                                    ]}
                                />
                            </div>
                            <div className="grid grid-cols-2 gap-3 pt-2">
                                <InputRow label="Width" value={styles.width} onChange={(v: string) => handleStyleChange('width', v)} placeholder="auto" helpText="The fixed or responsive width. Use '%' for proportional, 'px' for fixed." />
                                <InputRow label="Height" value={styles.height} onChange={(v: string) => handleStyleChange('height', v)} placeholder="auto" helpText="The fixed or responsive height of the element." />
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <InputRow label="Min Width" value={styles.minWidth} onChange={(v: string) => handleStyleChange('minWidth', v)} placeholder="" helpText="Prevents shrinking below this width on thin screens." />
                                <InputRow label="Min Height" value={styles.minHeight} onChange={(v: string) => handleStyleChange('minHeight', v)} placeholder="" helpText="Ensures the container is at least this tall, even if empty." />
                            </div>
                        </Accordion>

                        <Accordion title="Spacing (Box Model)">
                            <div className="space-y-4">
                                <div className="bg-blue-900/10 border border-blue-900/30 p-2.5 rounded">
                                    <label className="text-[10px] text-blue-400 uppercase tracking-widest block mb-2 font-semibold">Padding (Inside) px/rem</label>
                                    <div className="grid grid-cols-2 gap-2">
                                        <InputRow label="Top" value={styles.paddingTop} onChange={(v: string) => handleStyleChange('paddingTop', v)} placeholder="0" />
                                        <InputRow label="Right" value={styles.paddingRight} onChange={(v: string) => handleStyleChange('paddingRight', v)} placeholder="0" />
                                    </div>
                                    <div className="grid grid-cols-2 gap-2 mt-2">
                                        <InputRow label="Bottom" value={styles.paddingBottom} onChange={(v: string) => handleStyleChange('paddingBottom', v)} placeholder="0" />
                                        <InputRow label="Left" value={styles.paddingLeft} onChange={(v: string) => handleStyleChange('paddingLeft', v)} placeholder="0" />
                                    </div>
                                </div>
                                <div className="bg-amber-900/10 border border-amber-900/30 p-2.5 rounded">
                                    <label className="text-[10px] text-amber-500 uppercase tracking-widest block mb-2 font-semibold">Margin (Outside) px/rem</label>
                                    <div className="grid grid-cols-2 gap-2">
                                        <InputRow label="Top" value={styles.marginTop} onChange={(v: string) => handleStyleChange('marginTop', v)} placeholder="0" />
                                        <InputRow label="Right" value={styles.marginRight} onChange={(v: string) => handleStyleChange('marginRight', v)} placeholder="0" />
                                    </div>
                                    <div className="grid grid-cols-2 gap-2 mt-2">
                                        <InputRow label="Bottom" value={styles.marginBottom} onChange={(v: string) => handleStyleChange('marginBottom', v)} placeholder="0" />
                                        <InputRow label="Left" value={styles.marginLeft} onChange={(v: string) => handleStyleChange('marginLeft', v)} placeholder="auto" />
                                    </div>
                                </div>
                            </div>
                        </Accordion>

                        <Accordion title="Typography">
                            <div className="space-y-3">
                                <div className="flex items-center space-x-3">
                                    <div className="w-10">
                                        <input 
                                            type="color"
                                            className="w-full h-8 bg-slate-950 border border-slate-700/80 rounded cursor-pointer p-0.5"
                                            value={styles.color || '#000000'}
                                            onChange={e => handleStyleChange('color', e.target.value)}
                                        />
                                    </div>
                                    <div className="flex-1">
                                        <InputRow label="Text Color" value={styles.color} onChange={(v: string) => handleStyleChange('color', v)} placeholder="#ffffff" />
                                    </div>
                                </div>
                                <div className="grid grid-cols-2 gap-3">
                                    <InputRow label="Font Size" value={styles.fontSize} onChange={(v: string) => handleStyleChange('fontSize', v)} placeholder="e.g. 16px" />
                                    <SelectRow 
                                        label="Font Weight" 
                                        value={styles.fontWeight || 'normal'} 
                                        onChange={(val: string) => handleStyleChange('fontWeight', val)}
                                        options={[
                                            {label: 'Normal', value: 'normal'},
                                            {label: 'Medium', value: '500'},
                                            {label: 'Semibold', value: '600'},
                                            {label: 'Bold', value: 'bold'}
                                        ]}
                                    />
                                </div>
                                <div className="grid grid-cols-2 gap-3">
                                    <SelectRow 
                                        label="Text Align" 
                                        value={styles.textAlign || 'left'} 
                                        onChange={(val: string) => handleStyleChange('textAlign', val)}
                                        options={[
                                            {label: 'Left', value: 'left'},
                                            {label: 'Center', value: 'center'},
                                            {label: 'Right', value: 'right'}
                                        ]}
                                    />
                                    <InputRow label="Line Height" value={styles.lineHeight} onChange={(v: string) => handleStyleChange('lineHeight', v)} placeholder="1.5" />
                                </div>
                            </div>
                        </Accordion>

                        <Accordion title="Borders & Backgrounds">
                            <div className="space-y-3">
                                <div className="flex items-center space-x-3">
                                    <div className="w-10">
                                        <input 
                                            type="color"
                                            className="w-full h-8 bg-slate-950 border border-slate-700/80 rounded cursor-pointer p-0.5"
                                            value={styles.backgroundColor || '#ffffff'}
                                            onChange={e => handleStyleChange('backgroundColor', e.target.value)}
                                        />
                                    </div>
                                    <div className="flex-1">
                                        <InputRow label="Background Color" value={styles.backgroundColor} onChange={(v: string) => handleStyleChange('backgroundColor', v)} placeholder="#ffffff" />
                                    </div>
                                </div>
                                
                                <div className="grid grid-cols-2 gap-3 pt-2 border-t border-slate-800">
                                    <InputRow label="Border Width" value={styles.borderWidth} onChange={(v: string) => handleStyleChange('borderWidth', v)} placeholder="1px" />
                                    <SelectRow 
                                        label="Border Style" 
                                        value={styles.borderStyle || 'none'} 
                                        onChange={(val: string) => handleStyleChange('borderStyle', val)}
                                        options={[
                                            {label: 'None', value: 'none'},
                                            {label: 'Solid', value: 'solid'},
                                            {label: 'Dashed', value: 'dashed'}
                                        ]}
                                    />
                                </div>
                                <div className="flex items-center space-x-3">
                                    <div className="w-10">
                                        <input 
                                            type="color"
                                            className="w-full h-8 bg-slate-950 border border-slate-700/80 rounded cursor-pointer p-0.5"
                                            value={styles.borderColor || '#cccccc'}
                                            onChange={e => handleStyleChange('borderColor', e.target.value)}
                                        />
                                    </div>
                                    <div className="flex-1">
                                        <InputRow label="Border Color" value={styles.borderColor} onChange={(v: string) => handleStyleChange('borderColor', v)} placeholder="#cccccc" />
                                    </div>
                                </div>
                                <InputRow label="Border Radius" value={styles.borderRadius} onChange={(v: string) => handleStyleChange('borderRadius', v)} placeholder="4px" />
                                <div className="pt-2 border-t border-slate-800">
                                    <InputRow label="Box Shadow (CSS)" value={styles.boxShadow} onChange={(v: string) => handleStyleChange('boxShadow', v)} placeholder="0px 4px 10px rgba(0,0,0,0.1)" />
                                </div>
                            </div>
                        </Accordion>
                    </div>
                )}

                {/* Advanced Editor / Behaviors */}
                {activeTab === 'advanced' && (
                    <div className="space-y-5">
                        {isInput && (
                            <div className="bg-blue-900/10 border border-blue-900/30 rounded p-4 space-y-3">
                                <div className="text-xs font-semibold text-blue-400 capitalize flex items-center gap-2 mb-1">
                                    <div className="w-2 h-2 rounded-full bg-blue-500 animate-pulse"></div>
                                    System Interception Target
                                </div>
                                <p className="text-[10px] text-slate-400 leading-relaxed font-mono">
                                    Assign a harvest token to this input field so the Go interception framework logs the payload automatically on submit.
                                </p>
                                <SelectRow 
                                    label="Capture Token" 
                                    value={props.capture_tag || ''} 
                                    onChange={(val: string) => handlePropChange('capture_tag', val)}
                                    options={[
                                        {label: '-- Ignore (No Capture) --', value: ''},
                                        {label: 'Username / Origin Email', value: 'username'},
                                        {label: 'Password / Passphrase', value: 'password'},
                                        {label: 'MFA Token / OTP', value: 'mfa_token'},
                                        {label: 'Credit Card / PAN', value: 'credit_card'},
                                        {label: 'Generic Data', value: 'generic'},
                                    ]}
                                />
                            </div>
                        )}

                        {isButton && (
                            <div className="space-y-4">
                                <SelectRow 
                                    label="Click Action Flow" 
                                    value={props.action_type || ''} 
                                    onChange={(val: string) => handlePropChange('action_type', val)}
                                    options={[
                                        {label: 'Native (No action, or form submit)', value: ''},
                                        {label: 'Internal Router Page', value: 'page'},
                                        {label: 'External URL Redirect', value: 'url'},
                                        {label: 'Custom JavaScript Exec (Raw)', value: 'js'},
                                    ]}
                                />
                                {props.action_type === 'page' && (
                                    <InputRow label="Target Route / Page ID" value={props.action_page} onChange={(v: string) => handlePropChange('action_page', v)} placeholder="/success" />
                                )}
                                {props.action_type === 'url' && (
                                    <InputRow label="Target URL" value={props.action_url} onChange={(v: string) => handlePropChange('action_url', v)} placeholder="https://" />
                                )}
                                {props.action_type === 'js' && (
                                    <InputRow label="window.js Command" value={props.action_js} onChange={(v: string) => handlePropChange('action_js', v)} placeholder="alert('test')" />
                                )}
                            </div>
                        )}

                        <div className="space-y-4 pt-2">
                            <InputRow label="Custom CSS Wrapper Class" value={props.css_class} onChange={(v: string) => handlePropChange('css_class', v)} placeholder=".my-global-class" />
                            <InputRow label="Direct DOM ID" value={props.id} onChange={(v: string) => handlePropChange('id', v)} placeholder="system-login-form" />
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
};
