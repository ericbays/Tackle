import React, { useState, useEffect, useRef, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Search, Loader2, Target, Users, Mail, Globe, Command, FileCode2 } from 'lucide-react';
import { api } from '../services/api';
import { Input } from './ui/Input';

export interface SearchResult {
	type: string;
	id: string;
	name: string;
	path: string;
}

export interface SearchResponse {
	campaigns: SearchResult[];
	targets: SearchResult[];
	templates: SearchResult[];
	domains: SearchResult[];
	applications: SearchResult[];
}

export function GlobalSearch() {
    const [query, setQuery] = useState('');
    const [debouncedQuery, setDebouncedQuery] = useState('');
    const [isSearching, setIsSearching] = useState(false);
    const [isOpen, setIsOpen] = useState(false);
    const [results, setResults] = useState<SearchResponse | null>(null);
    const [activeIndex, setActiveIndex] = useState(-1);
    
    const inputRef = useRef<HTMLInputElement>(null);
    const dropdownRef = useRef<HTMLDivElement>(null);
    const navigate = useNavigate();

    // Debounce input
    useEffect(() => {
        const timer = setTimeout(() => {
            setDebouncedQuery(query);
        }, 300);
        return () => clearTimeout(timer);
    }, [query]);

    // Fetch API
    useEffect(() => {
        const fetchSearch = async () => {
            if (!debouncedQuery.trim()) {
                setResults(null);
                setIsOpen(false);
                return;
            }
            
            setIsSearching(true);
            try {
                const res = await api.get(`/search?q=${encodeURIComponent(debouncedQuery)}`);
                const data = res.data?.data || res.data;
                setResults(data);
                setIsOpen(true);
                setActiveIndex(-1); // reset selection
            } catch (err) {
                console.error("Search failed", err);
            } finally {
                setIsSearching(false);
            }
        };
        fetchSearch();
    }, [debouncedQuery]);

    // Flatten results for keyboard navigation
    const flatResults = useMemo(() => {
        if (!results) return [];
        return [
            ...(results.campaigns || []),
            ...(results.targets || []),
            ...(results.templates || []),
            ...(results.domains || []),
            ...(results.applications || [])
        ];
    }, [results]);

    // Global shortcut Ctrl+K
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
                e.preventDefault();
                inputRef.current?.focus();
            }
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, []);

    // Outside click listener
    useEffect(() => {
        const handleClickOutside = (e: MouseEvent) => {
            if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node) &&
                inputRef.current && !inputRef.current.contains(e.target as Node)) {
                setIsOpen(false);
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, []);

    // Input Keydown (Up/Down/Enter/Escape)
    const handleInputKeyDown = (e: React.KeyboardEvent) => {
        if (!isOpen) return;

        if (e.key === 'ArrowDown') {
            e.preventDefault();
            setActiveIndex(prev => (prev < flatResults.length - 1 ? prev + 1 : prev));
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            setActiveIndex(prev => (prev > 0 ? prev - 1 : 0));
        } else if (e.key === 'Enter') {
            e.preventDefault();
            if (activeIndex >= 0 && activeIndex < flatResults.length) {
                navigateItem(flatResults[activeIndex]);
            }
        } else if (e.key === 'Escape') {
            e.preventDefault();
            setIsOpen(false);
            inputRef.current?.blur();
        }
    };

    const navigateItem = (item: SearchResult) => {
        navigate(item.path);
        setIsOpen(false);
        // We do NOT clear query text here based on user preference
        inputRef.current?.blur();
    };

    const hasResults = flatResults.length > 0;

    return (
        <div className="relative w-96 z-50">
            <div className="relative flex items-center w-full group">
                <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 group-focus-within:text-blue-500 transition-colors pointer-events-none" />
                <Input
                    ref={inputRef}
                    type="text"
                    value={query}
                    onChange={(e) => {
                        setQuery(e.target.value);
                        if (!isOpen && e.target.value.trim().length > 0) setIsOpen(true);
                    }}
                    onFocus={() => {
                        if (debouncedQuery.trim() && results) setIsOpen(true);
                    }}
                    onKeyDown={handleInputKeyDown}
                    placeholder="Search campaigns, targets, templates..."
                    className="pl-9 pr-14 shadow-sm"
                />
                
                <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-2">
                    {isSearching ? (
                        <Loader2 className="w-4 h-4 text-blue-500 animate-spin" />
                    ) : (
                        <kbd className="hidden sm:inline-flex items-center gap-1 font-sans text-[10px] font-medium text-slate-500 bg-slate-800 border border-slate-700 rounded px-1.5 py-0.5 pointer-events-none">
                            <Command className="w-2.5 h-2.5" />K
                        </kbd>
                    )}
                </div>
            </div>

            {isOpen && query.trim() !== '' && (
                <div 
                    ref={dropdownRef}
                    className="absolute top-full left-0 right-0 mt-2 bg-slate-900/95 backdrop-blur-md border border-slate-700 rounded-lg shadow-2xl overflow-hidden max-h-[60vh] overflow-y-auto"
                >
                    {!isSearching && !hasResults ? (
                        <div className="p-6 text-center text-slate-500 text-sm">
                            No matching records found.
                        </div>
                    ) : (
                        <div className="py-2">
                            {/* Campaigns */}
                            {results?.campaigns && results.campaigns.length > 0 && (
                                <div className="mb-2">
                                    <div className="px-3 py-1 text-[10px] font-semibold text-slate-500 uppercase tracking-wider bg-slate-800/30">
                                        Campaigns
                                    </div>
                                    {results.campaigns.map((item) => {
                                        const globIdx = flatResults.findIndex(r => r.id === item.id && r.type === item.type);
                                        return (
                                            <div 
                                                key={`camp-${item.id}`}
                                                className={`px-3 py-2.5 flex items-center gap-3 cursor-pointer transition-colors ${activeIndex === globIdx ? 'bg-blue-600/20 border-l-2 border-blue-500' : 'hover:bg-slate-800 border-l-2 border-transparent'}`}
                                                onClick={() => navigateItem(item)}
                                                onMouseEnter={() => setActiveIndex(globIdx)}
                                            >
                                                <Target className={`w-4 h-4 ${activeIndex === globIdx ? 'text-blue-400' : 'text-emerald-500'}`} />
                                                <span className={`${activeIndex === globIdx ? 'text-blue-100' : 'text-slate-300'} truncate font-medium text-sm`}>{item.name}</span>
                                            </div>
                                        );
                                    })}
                                </div>
                            )}

                            {/* Targets */}
                            {results?.targets && results.targets.length > 0 && (
                                <div className="mb-2">
                                    <div className="px-3 py-1 text-[10px] font-semibold text-slate-500 uppercase tracking-wider bg-slate-800/30">
                                        Targets
                                    </div>
                                    {results.targets.map((item) => {
                                        const globIdx = flatResults.findIndex(r => r.id === item.id && r.type === item.type);
                                        return (
                                            <div 
                                                key={`tgt-${item.id}`}
                                                className={`px-3 py-2.5 flex items-center gap-3 cursor-pointer transition-colors ${activeIndex === globIdx ? 'bg-blue-600/20 border-l-2 border-blue-500' : 'hover:bg-slate-800 border-l-2 border-transparent'}`}
                                                onClick={() => navigateItem(item)}
                                                onMouseEnter={() => setActiveIndex(globIdx)}
                                            >
                                                <Users className={`w-4 h-4 ${activeIndex === globIdx ? 'text-blue-400' : 'text-violet-500'}`} />
                                                <span className={`${activeIndex === globIdx ? 'text-blue-100' : 'text-slate-300'} truncate font-medium text-sm`}>{item.name}</span>
                                            </div>
                                        );
                                    })}
                                </div>
                            )}

                            {/* Templates */}
                            {results?.templates && results.templates.length > 0 && (
                                <div className="mb-2">
                                    <div className="px-3 py-1 text-[10px] font-semibold text-slate-500 uppercase tracking-wider bg-slate-800/30">
                                        Email Templates
                                    </div>
                                    {results.templates.map((item) => {
                                        const globIdx = flatResults.findIndex(r => r.id === item.id && r.type === item.type);
                                        return (
                                            <div 
                                                key={`tmpl-${item.id}`}
                                                className={`px-3 py-2.5 flex items-center gap-3 cursor-pointer transition-colors ${activeIndex === globIdx ? 'bg-blue-600/20 border-l-2 border-blue-500' : 'hover:bg-slate-800 border-l-2 border-transparent'}`}
                                                onClick={() => navigateItem(item)}
                                                onMouseEnter={() => setActiveIndex(globIdx)}
                                            >
                                                <Mail className={`w-4 h-4 ${activeIndex === globIdx ? 'text-blue-400' : 'text-amber-500'}`} />
                                                <span className={`${activeIndex === globIdx ? 'text-blue-100' : 'text-slate-300'} truncate font-medium text-sm`}>{item.name}</span>
                                            </div>
                                        );
                                    })}
                                </div>
                            )}

                            {/* Domains */}
                            {results?.domains && results.domains.length > 0 && (
                                <div className="mb-2">
                                    <div className="px-3 py-1 text-[10px] font-semibold text-slate-500 uppercase tracking-wider bg-slate-800/30">
                                        Domains
                                    </div>
                                    {results.domains.map((item) => {
                                        const globIdx = flatResults.findIndex(r => r.id === item.id && r.type === item.type);
                                        return (
                                            <div 
                                                key={`dom-${item.id}`}
                                                className={`px-3 py-2.5 flex items-center gap-3 cursor-pointer transition-colors ${activeIndex === globIdx ? 'bg-blue-600/20 border-l-2 border-blue-500' : 'hover:bg-slate-800 border-l-2 border-transparent'}`}
                                                onClick={() => navigateItem(item)}
                                                onMouseEnter={() => setActiveIndex(globIdx)}
                                            >
                                                <Globe className={`w-4 h-4 ${activeIndex === globIdx ? 'text-blue-400' : 'text-cyan-500'}`} />
                                                <span className={`${activeIndex === globIdx ? 'text-blue-100' : 'text-slate-300'} truncate font-medium text-sm`}>{item.name}</span>
                                            </div>
                                        );
                                    })}
                                </div>
                            )}

                            {/* Applications */}
                            {results?.applications && results.applications.length > 0 && (
                                <div className="mb-2">
                                    <div className="px-3 py-1 text-[10px] font-semibold text-slate-500 uppercase tracking-wider bg-slate-800/30">
                                        Landing Applications
                                    </div>
                                    {results.applications.map((item) => {
                                        const globIdx = flatResults.findIndex(r => r.id === item.id && r.type === item.type);
                                        return (
                                            <div 
                                                key={`app-${item.id}`}
                                                className={`px-3 py-2.5 flex items-center gap-3 cursor-pointer transition-colors ${activeIndex === globIdx ? 'bg-blue-600/20 border-l-2 border-blue-500' : 'hover:bg-slate-800 border-l-2 border-transparent'}`}
                                                onClick={() => navigateItem(item)}
                                                onMouseEnter={() => setActiveIndex(globIdx)}
                                            >
                                                <FileCode2 className={`w-4 h-4 ${activeIndex === globIdx ? 'text-blue-400' : 'text-teal-500'}`} />
                                                <span className={`${activeIndex === globIdx ? 'text-blue-100' : 'text-slate-300'} truncate font-medium text-sm`}>{item.name}</span>
                                            </div>
                                        );
                                    })}
                                </div>
                            )}
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
