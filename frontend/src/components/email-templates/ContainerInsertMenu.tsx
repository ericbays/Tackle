import { useState, useRef, useEffect } from 'react';
import { LayoutTemplate } from 'lucide-react';

interface ContainerInsertMenuProps {
    onInsert: (html: string) => void;
    buttonClassName?: string;
}

export default function ContainerInsertMenu({ onInsert, buttonClassName = "" }: ContainerInsertMenuProps) {
    const [isOpen, setIsOpen] = useState(false);
    const [bgColor, setBgColor] = useState('#f8fafc'); // Default very light slate
    const [hasBorder, setHasBorder] = useState(true);
    const [padding, setPadding] = useState('20px');
    const menuRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
                setIsOpen(false);
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, []);

    const handleInsert = () => {
        let borderStyle = hasBorder ? 'border: 1px solid #e2e8f0;' : 'border: none;';
        let html = `<p><br></p><div style="padding: ${padding}; background-color: ${bgColor}; ${borderStyle} margin-bottom: 20px;">
            Content Area
        </div><p><br></p>`;
        
        onInsert(html);
        setIsOpen(false);
    };

    return (
        <div className="relative" ref={menuRef}>
            <button 
                onClick={() => setIsOpen(!isOpen)}
                className={buttonClassName}
                title="Insert Configurable Container"
            >
                <LayoutTemplate className="w-4 h-4" />
            </button>

            {isOpen && (
                <div className="absolute top-full left-0 mt-1 w-64 bg-slate-800 border border-slate-700 rounded-md shadow-xl z-50 p-4 shrink-0">
                    <h3 className="text-xs font-semibold text-slate-300 mb-3 uppercase tracking-wider">Configure Container</h3>
                    <div className="space-y-3">
                        <div>
                            <label className="block text-xs text-slate-400 mb-1">Background Color</label>
                            <div className="flex items-center gap-2">
                                <input type="color" value={bgColor} onChange={e => setBgColor(e.target.value)} className="w-8 h-8 rounded cursor-pointer border-0 p-0 bg-transparent" />
                                <input type="text" value={bgColor} onChange={e => setBgColor(e.target.value)} className="flex-1 bg-slate-900 border border-slate-700 rounded text-sm p-1.5 text-white focus:outline-none font-mono focus:border-blue-500" />
                            </div>
                        </div>
                        <div>
                            <label className="block text-xs text-slate-400 mb-1">Padding Space</label>
                            <select value={padding} onChange={e => setPadding(e.target.value)} className="w-full bg-slate-900 border border-slate-700 rounded text-sm p-1.5 text-white focus:outline-none focus:border-blue-500">
                                <option value="10px">Small (10px)</option>
                                <option value="20px">Medium (20px)</option>
                                <option value="40px">Large (40px)</option>
                            </select>
                        </div>
                        <div className="flex items-center gap-2 pt-2">
                            <input type="checkbox" id="hasBorder" checked={hasBorder} onChange={e => setHasBorder(e.target.checked)} className="rounded border-slate-700 bg-slate-900 text-blue-500 focus:ring-blue-500 focus:ring-offset-slate-800" />
                            <label htmlFor="hasBorder" className="text-sm text-slate-300">Show Subtle Border</label>
                        </div>
                        <button 
                            onClick={handleInsert}
                            className="w-full mt-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium py-2 rounded transition-colors"
                        >
                            Insert Container
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
