import { useState, useRef, useEffect } from 'react';
import { Type } from 'lucide-react';

const VARIABLES = [
    { label: 'First Name', value: '{{.FirstName}}' },
    { label: 'Last Name', value: '{{.LastName}}' },
    { label: 'Email', value: '{{.Email}}' },
    { label: 'Tracking URL', value: '{{.TrackingURL}}' },
    { label: 'Target URL', value: '{{.TargetURL}}' },
    { label: 'Campaign Name', value: '{{.CampaignName}}' },
];

interface VariableInsertMenuProps {
    onInsert: (value: string) => void;
    buttonClassName?: string;
}

export default function VariableInsertMenu({ onInsert, buttonClassName = "" }: VariableInsertMenuProps) {
    const [isOpen, setIsOpen] = useState(false);
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

    return (
        <div className="relative inline-block" ref={menuRef}>
            <button 
                type="button"
                onClick={() => setIsOpen(!isOpen)}
                className={`flex items-center gap-1 ${buttonClassName}`}
                title="Insert Variable"
            >
                <Type className="w-3.5 h-3.5" />
                <span>Var</span>
            </button>

            {isOpen && (
                <div className="absolute right-0 top-full mt-1 w-52 bg-slate-800 border border-slate-700 rounded-md shadow-xl z-50 py-1">
                    <div className="px-3 py-1.5 text-xs font-semibold text-slate-400 uppercase tracking-wider border-b border-slate-700 mb-1">
                        Template Variables
                    </div>
                    {VARIABLES.map(v => (
                        <button
                            key={v.value}
                            type="button"
                            className="w-full text-left px-3 py-2 text-sm text-slate-200 hover:bg-slate-700 hover:text-white transition-colors"
                            onClick={() => {
                                onInsert(v.value);
                                setIsOpen(false);
                            }}
                        >
                            <span className="block font-medium">{v.label}</span>
                            <span className="block text-xs text-slate-500 font-mono mt-0.5">{v.value}</span>
                        </button>
                    ))}
                </div>
            )}
        </div>
    );
}
