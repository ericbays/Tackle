import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useBuilderStore } from '../../store/builderStore';
import { RenderNode } from './RenderNode';

export const Canvas = () => {
    const iframeRef = useRef<HTMLIFrameElement>(null);
    const [iframeBody, setIframeBody] = useState<HTMLElement | null>(null);
    const activePageId = useBuilderStore(state => state.activePageId);
    const project = useBuilderStore(state => state.project);
    const zoom = useBuilderStore(state => state.zoom);
    const devicePreview = useBuilderStore(state => state.devicePreview);

    useEffect(() => {
        const iframe = iframeRef.current;
        if (!iframe) return;

        const handleLoad = () => {
            const doc = iframe.contentDocument;
            if (doc) {
                // Initialize standard Tailwind reset + generic HTML styles inside iframe context
                const style = doc.createElement('style');
                style.innerHTML = `
                    body {
                        margin: 0;
                        padding: 0;
                        font-family: Inter, sans-serif;
                        min-height: 100vh;
                        background: #ffffff;
                    }
                    * {
                        box-sizing: border-box;
                    }
                `;
                doc.head.appendChild(style);
                setIframeBody(doc.body);
            }
        };

        // If the iframe already loaded (e.g. fast refresh)
        if (iframe.contentDocument?.readyState === 'complete') {
            handleLoad();
        }

        iframe.addEventListener('load', handleLoad);
        return () => iframe.removeEventListener('load', handleLoad);
    }, []);

    let deviceWidth = '100%';
    if (devicePreview === 'desktop') deviceWidth = '1440px';
    if (devicePreview === 'tablet') deviceWidth = '768px';
    if (devicePreview === 'mobile') deviceWidth = '375px';

    const activePage = project?.pages.find(p => p.id === activePageId);

    return (
        <div className="flex-1 bg-slate-950 overflow-auto flex items-center justify-center relative p-8">
            <div 
                style={{ 
                    width: deviceWidth, 
                    transform: `scale(${zoom / 100})`, 
                    transition: 'all 0.2s ease-out',
                    transformOrigin: 'top center',
                    minHeight: '100%',
                    boxShadow: '0 0 40px rgba(0,0,0,0.5)'
                }}
                className="bg-white rounded-md overflow-hidden relative"
            >
                <iframe 
                    ref={iframeRef}
                    src="about:blank"
                    className="w-full h-full min-h-[800px] border-none"
                    title="Landing Page Canvas"
                />
                
                {iframeBody && activePage && createPortal(
                    <div className="canvas-root min-h-screen">
                        <RenderNode node={activePage.rootComponent} />
                    </div>,
                    iframeBody
                )}
            </div>
        </div>
    );
};
