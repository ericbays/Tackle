import { useState, useRef, useEffect, useMemo } from 'react';
import { Bold, Italic, Strikethrough, Heading1, Heading2, Heading3, List, ListOrdered, Link as LinkIcon, Monitor, Smartphone, Maximize, Minimize, AlertTriangle } from 'lucide-react';
import Prism from 'prismjs';
import 'prismjs/components/prism-markup';
import 'prismjs/themes/prism-tomorrow.css';

import beautify from 'js-beautify';
import VariableInsertMenu from './VariableInsertMenu';

// Add custom parsing for Handlebars-style template variables {{...}}
if (Prism.languages && Prism.languages.markup) {
    Prism.languages.insertBefore('markup', 'tag', {
        'template-variable': {
            pattern: /\{\{.*?\}\}/,
            alias: 'important'
        }
    });
}

function formatHTML(html: string) {
    if (!html) return '';
    try {
        return beautify.html(html, {
            indent_size: 4,
            indent_char: ' ',
            max_preserve_newlines: 1,
            preserve_newlines: true,
            indent_inner_html: true,
            wrap_line_length: 0
        });
    } catch {
        return html;
    }
}

function checkOutlookCompatibility(html: string): string[] {
    const warnings: string[] = [];
    if (!html) return warnings;
    
    if (/display\s*:\s*(flex|grid)/i.test(html)) {
        warnings.push("Flex/Grid layouts (Use classic <table> structures)");
    }
    if (/border-radius\s*:/i.test(html)) {
        warnings.push("Rounded corners (Degrades to hard squares)");
    }
    if (/box-shadow\s*:/i.test(html)) {
        warnings.push("Box shadows (Completely ignored)");
    }
    if (/background-image\s*:/i.test(html)) {
        warnings.push("Background images (May require VML to render)");
    }
    if (/position\s*:\s*(absolute|fixed)/i.test(html)) {
        warnings.push("Absolute positioning (Layout will collapse)");
    }
    
    return warnings;
}

interface HtmlEmailEditorProps {
    content: string;
    onChange: (html: string) => void;
}

export default function HtmlEmailEditor({ content, onChange }: HtmlEmailEditorProps) {
    const [mode, setMode] = useState<'wysiwyg' | 'html' | 'preview'>('wysiwyg');
    const [previewMode, setPreviewMode] = useState<'desktop' | 'mobile'>('desktop');
    const [htmlInput, setHtmlInput] = useState(content);
    const [isFullscreen, setIsFullscreen] = useState(false);
    const iframeRef = useRef<HTMLIFrameElement>(null);
    const isUpdatingRef = useRef(false);
    const cleanupRef = useRef<(() => void) | null>(null);

    const outlookWarnings = useMemo(() => checkOutlookCompatibility(htmlInput), [htmlInput]);

    const setupIframe = (initialHtml: string) => {
        const doc = iframeRef.current?.contentDocument;
        if (!doc) return null;
        
        doc.open();
        doc.write(initialHtml || '<!DOCTYPE html>\n<html><head></head><body></body></html>');
        doc.close();
        doc.designMode = "on";
        
        const syncChanges = () => {
            if (isUpdatingRef.current) return;
            let html = doc.documentElement.outerHTML;
            if (doc.doctype) {
                html = new XMLSerializer().serializeToString(doc.doctype) + '\n' + html;
            }
            
            // Format HTML unconditionally before broadcasting to parent to guarantee beautiful, standard output
            html = formatHTML(html);
            
            // Update parent state
            onChange(html);
            
            // Sync local input state silently without triggering re-render of iframe
            isUpdatingRef.current = true;
            setHtmlInput(html);
            isUpdatingRef.current = false;
        };

        const observer = new MutationObserver(syncChanges);
        observer.observe(doc.documentElement, { childList: true, subtree: true, characterData: true, attributes: true });
        doc.addEventListener('keyup', syncChanges);
        
        return () => {
            observer.disconnect();
            doc.removeEventListener('keyup', syncChanges);
        };
    };

    // Initialize iframe on mount or wysiwyg mode switch
    useEffect(() => {
        if (mode === 'wysiwyg') {
            cleanupRef.current = setupIframe(htmlInput);
        } else if (mode === 'html') {
            // Auto-format HTML when explicitly switching into code view
            const formatted = formatHTML(htmlInput);
            if (formatted !== htmlInput) {
                setHtmlInput(formatted);
                onChange(formatted);
            }
        }
        return () => {
            if (cleanupRef.current) cleanupRef.current();
        };
    }, [mode]);

    // Handle external content prop sync (like loading a template from API)
    useEffect(() => {
        if (content !== htmlInput && !isUpdatingRef.current) {
            setHtmlInput(content);
            if (mode === 'wysiwyg') {
                if (cleanupRef.current) cleanupRef.current();
                cleanupRef.current = setupIframe(content);
            }
        }
    }, [content]);



    const execCmd = (cmd: string, val?: string) => {
        if (iframeRef.current?.contentDocument) {
            iframeRef.current.contentDocument.execCommand(cmd, false, val);
            iframeRef.current.contentWindow?.focus();
        }
    };

    const setLink = () => {
        const url = window.prompt('URL:');
        if (url) {
            execCmd('createLink', url);
        } else if (url === '') {
            execCmd('unlink');
        }
    };

    const insertVariable = (variable: string) => {
        execCmd('insertHTML', variable);
    };

    return (
        <div className={`flex flex-col border-slate-700 overflow-hidden bg-slate-900 flex-1 ${
            isFullscreen ? 'fixed inset-0 z-50 border-0 rounded-none' : 'border rounded-md h-full'
        }`}>
            {/* Tabs */}
            <div className="flex justify-between items-center bg-slate-950 border-b border-slate-800 p-1 shrink-0">
                <div className="flex gap-1">
                    <button 
                        onClick={() => setMode('wysiwyg')}
                        className={`px-4 py-1.5 text-sm rounded font-medium transition-colors ${mode === 'wysiwyg' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}
                    >
                        WYSIWYG
                    </button>
                    <button 
                        onClick={() => setMode('html')}
                        className={`px-4 py-1.5 text-sm rounded font-medium transition-colors ${mode === 'html' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}
                    >
                        Code
                    </button>
                    <button 
                        onClick={() => setMode('preview')}
                        className={`px-4 py-1.5 text-sm rounded font-medium transition-colors ${mode === 'preview' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}
                    >
                        Preview
                    </button>
                </div>
                <button
                    onClick={() => setIsFullscreen(!isFullscreen)}
                    className="p-1.5 mr-1 text-slate-400 hover:text-white rounded hover:bg-slate-800/50 transition-colors"
                    title={isFullscreen ? "Exit Fullscreen" : "Fullscreen Editor"}
                >
                    {isFullscreen ? <Minimize className="w-4 h-4" /> : <Maximize className="w-4 h-4" />}
                </button>
            </div>

            {/* Toolbar */}
            {mode === 'wysiwyg' && (
                <div className="flex flex-wrap items-center gap-1 p-2 border-b border-slate-800 bg-slate-900 shrink-0">
                    <div className="flex gap-1 pr-2 border-r border-slate-700">
                         <button onClick={() => execCmd('bold')} className="p-1.5 rounded hover:bg-slate-700 text-slate-400"><Bold className="w-4 h-4" /></button>
                         <button onClick={() => execCmd('italic')} className="p-1.5 rounded hover:bg-slate-700 text-slate-400"><Italic className="w-4 h-4" /></button>
                         <button onClick={() => execCmd('strikeThrough')} className="p-1.5 rounded hover:bg-slate-700 text-slate-400"><Strikethrough className="w-4 h-4" /></button>
                     </div>
                     <div className="flex gap-1 px-2 border-r border-slate-700">
                         <button onClick={() => execCmd('formatBlock', 'H1')} className="p-1.5 rounded hover:bg-slate-700 text-slate-400"><Heading1 className="w-4 h-4" /></button>
                         <button onClick={() => execCmd('formatBlock', 'H2')} className="p-1.5 rounded hover:bg-slate-700 text-slate-400"><Heading2 className="w-4 h-4" /></button>
                         <button onClick={() => execCmd('formatBlock', 'H3')} className="p-1.5 rounded hover:bg-slate-700 text-slate-400"><Heading3 className="w-4 h-4" /></button>
                     </div>
                     <div className="flex gap-1 px-2 border-r border-slate-700">
                         <button onClick={() => execCmd('insertUnorderedList')} className="p-1.5 rounded hover:bg-slate-700 text-slate-400"><List className="w-4 h-4" /></button>
                         <button onClick={() => execCmd('insertOrderedList')} className="p-1.5 rounded hover:bg-slate-700 text-slate-400"><ListOrdered className="w-4 h-4" /></button>
                     </div>
                     <div className="flex gap-1 px-2 border-r border-slate-700">
                         <button onClick={setLink} className="p-1.5 rounded hover:bg-slate-700 text-slate-400"><LinkIcon className="w-4 h-4" /></button>
                     </div>
                     <div className="flex pl-2">
                         <VariableInsertMenu 
                             onInsert={insertVariable}
                             buttonClassName="px-2 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium rounded transition-colors"
                         />
                     </div>
                </div>
            )}

            {mode === 'preview' && (
                <div className="flex items-center justify-end p-2 border-b border-slate-800 bg-slate-900 shrink-0">
                    <div className="flex bg-slate-800 rounded-md p-1">
                        <button 
                            onClick={() => setPreviewMode('desktop')}
                            className={`p-1.5 rounded-sm ${previewMode === 'desktop' ? 'bg-slate-700 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}`}
                        >
                            <Monitor className="w-4 h-4" />
                        </button>
                        <button 
                            onClick={() => setPreviewMode('mobile')}
                            className={`p-1.5 rounded-sm ${previewMode === 'mobile' ? 'bg-slate-700 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}`}
                        >
                            <Smartphone className="w-4 h-4" />
                        </button>
                    </div>
                </div>
            )}

            {outlookWarnings.length > 0 && (
                <div className="flex flex-col bg-amber-500/10 border-b border-amber-500/20 px-4 py-2 shrink-0">
                    <div className="flex items-center gap-2 text-amber-500 mb-1">
                        <AlertTriangle className="w-4 h-4" />
                        <span className="text-xs font-semibold uppercase tracking-wider">Outlook Classic Degradation Detected</span>
                    </div>
                    <ul className="list-disc pl-6 text-xs text-amber-400/80 space-y-0.5">
                        {outlookWarnings.map((warning, i) => (
                            <li key={i}>{warning}</li>
                        ))}
                    </ul>
                </div>
            )}

            <div className={`relative flex-1 min-h-0 overflow-hidden ${mode === 'preview' ? 'bg-slate-950 p-4' : 'bg-white'}`}>
                {mode === 'wysiwyg' && (
                    <iframe 
                        ref={iframeRef}
                        className="absolute inset-0 w-full h-full border-none"
                        title="WYSIWYG Editor"
                    />
                )}
                {mode === 'html' && (
                    <>
                        <style>{`
                            .token.template-variable {
                                color: #facc15 !important;
                                font-weight: bold;
                                background: rgba(250, 204, 21, 0.1);
                                padding: 0 2px;
                                border-radius: 2px;
                            }
                            textarea { outline: none !important; }
                        `}</style>
                        <div className="absolute inset-0 overflow-auto bg-slate-950 font-mono text-sm group flex rounded-b-xl border-t border-slate-800" style={{ fontFamily: '"Fira Code", "JetBrains Mono", monospace' }}>
                            {/* Line Number Gutter */}
                            <div className="w-12 shrink-0 bg-slate-900 border-r border-slate-800 text-slate-500 py-4 flex flex-col items-end pr-3 select-none">
                                {Array.from({ length: (htmlInput || '').split('\n').length || 1 }).map((_, i) => (
                                    <div key={i} className="h-[21px] leading-[21px] text-[13px]">{i + 1}</div>
                                ))}
                            </div>
                            
                            {/* Editor Area */}
                            <div className="relative min-h-full flex-1">
                                <textarea
                                    value={htmlInput || ''}
                                    onChange={(e) => {
                                        const code = e.target.value;
                                        isUpdatingRef.current = true;
                                        setHtmlInput(code);
                                        onChange(code);
                                        isUpdatingRef.current = false;
                                    }}
                                    onScroll={(e) => {
                                        const target = e.currentTarget;
                                        const pre = target.nextSibling as HTMLPreElement;
                                        if (pre) {
                                            pre.scrollTop = target.scrollTop;
                                            pre.scrollLeft = target.scrollLeft;
                                        }
                                    }}
                                    className="absolute inset-0 w-full min-h-full p-4 bg-transparent text-transparent caret-white resize-none border-none outline-none z-10 whitespace-pre overflow-hidden group-hover:overflow-auto leading-[21px] text-[13px]"
                                    spellCheck="false"
                                />
                                <pre
                                    className="absolute inset-0 w-full min-h-full p-4 m-0 overflow-hidden pointer-events-none whitespace-pre text-[#d4d4d4] leading-[21px] text-[13px]"
                                    aria-hidden="true"
                                    dangerouslySetInnerHTML={{
                                        __html: (() => {
                                            try {
                                                const raw = htmlInput || '';
                                                let highlighted = Prism.highlight(raw, Prism.languages.markup || Prism.languages.html || {}, 'markup');
                                                // Ensure the pre block spans the identical height
                                                if (highlighted.endsWith('\n')) {
                                                    highlighted += ' ';
                                                }
                                                return highlighted;
                                            } catch (err) {
                                                return htmlInput || '';
                                            }
                                        })()
                                    }}
                                />
                            </div>
                        </div>
                    </>
                )}
                {mode === 'preview' && (
                    <div className={`absolute top-4 left-4 right-4 bottom-4 transition-all duration-300 bg-white ${previewMode === 'mobile' ? 'w-[375px] h-[667px] mx-auto rounded-3xl border-8 border-slate-800 overflow-hidden shadow-2xl relative left-auto right-auto top-auto bottom-auto max-h-full' : 'rounded shadow'}`}>
                        <iframe 
                            srcDoc={htmlInput || ""}
                            className="absolute inset-0 w-full h-full border-none"
                            sandbox="allow-same-origin"
                        />
                    </div>
                )}
            </div>
            
            <div className="p-2 border-t border-slate-800 text-xs text-slate-500 text-right bg-slate-950">
                {htmlInput.length} characters
            </div>
        </div>
    );
}
