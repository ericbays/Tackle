import { useState, useRef, useEffect } from 'react';
import { Table as TableIcon } from 'lucide-react';

interface TableInsertMenuProps {
    onInsert: (html: string) => void;
    buttonClassName?: string;
}

export default function TableInsertMenu({ onInsert, buttonClassName = "" }: TableInsertMenuProps) {
    const [isOpen, setIsOpen] = useState(false);
    const [rows, setRows] = useState(2);
    const [cols, setCols] = useState(2);
    const [hasHeader, setHasHeader] = useState(false);
    const [fullWidth, setFullWidth] = useState(true);
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
        let html = `<p><br></p><table ${fullWidth ? 'width="100%"' : ''} border="0" cellpadding="0" cellspacing="0" style="margin: 0;">`;
        if (hasHeader) {
            html += '<thead><tr>';
            for (let i = 0; i < cols; i++) {
                html += `<th style="padding: 10px; border: 1px solid #ddd; background-color: #f1f5f9; text-align: left; font-weight: bold;">Header</th>`;
            }
            html += '</tr></thead>';
        }
        html += '<tbody>';
        for (let r = 0; r < rows; r++) {
            html += '<tr>';
            for (let c = 0; c < cols; c++) {
                html += `<td style="padding: 10px; border: 1px solid #ddd;">Cell</td>`;
            }
            html += '</tr>';
        }
        html += '</tbody></table><p><br></p>';
        
        onInsert(html);
        setIsOpen(false);
    };

    return (
        <div className="relative" ref={menuRef}>
            <button 
                onClick={() => setIsOpen(!isOpen)}
                className={buttonClassName}
                title="Insert Configurable Table"
            >
                <TableIcon className="w-4 h-4" />
            </button>

            {isOpen && (
                <div className="absolute top-full left-0 mt-1 w-64 bg-slate-800 border border-slate-700 rounded-md shadow-xl z-50 p-4 shrink-0">
                    <h3 className="text-xs font-semibold text-slate-300 mb-3 uppercase tracking-wider">Configure Table</h3>
                    <div className="space-y-3">
                        <div className="grid grid-cols-2 gap-3">
                            <div>
                                <label className="block text-xs text-slate-400 mb-1">Rows</label>
                                <input type="number" min="1" max="20" value={rows} onChange={e => setRows(Number(e.target.value))} className="w-full bg-slate-900 border border-slate-700 rounded text-sm p-1.5 text-white focus:outline-none focus:border-blue-500" />
                            </div>
                            <div>
                                <label className="block text-xs text-slate-400 mb-1">Columns</label>
                                <input type="number" min="1" max="10" value={cols} onChange={e => setCols(Number(e.target.value))} className="w-full bg-slate-900 border border-slate-700 rounded text-sm p-1.5 text-white focus:outline-none focus:border-blue-500" />
                            </div>
                        </div>
                        <div className="flex items-center gap-2">
                            <input type="checkbox" id="hasHeader" checked={hasHeader} onChange={e => setHasHeader(e.target.checked)} className="rounded border-slate-700 bg-slate-900 text-blue-500 focus:ring-blue-500 focus:ring-offset-slate-800" />
                            <label htmlFor="hasHeader" className="text-sm text-slate-300">Include Header Row</label>
                        </div>
                        <div className="flex items-center gap-2">
                            <input type="checkbox" id="fullWidth" checked={fullWidth} onChange={e => setFullWidth(e.target.checked)} className="rounded border-slate-700 bg-slate-900 text-blue-500 focus:ring-blue-500 focus:ring-offset-slate-800" />
                            <label htmlFor="fullWidth" className="text-sm text-slate-300">100% Width Layout</label>
                        </div>
                        <button 
                            onClick={handleInsert}
                            className="w-full mt-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium py-2 rounded transition-colors"
                        >
                            Insert Table
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
