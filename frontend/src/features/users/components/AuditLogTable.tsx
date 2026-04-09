import React, { useState } from 'react';
import { useQuery, keepPreviousData } from '@tanstack/react-query';
import { auditApi } from '../services/auditApi';
import { ShieldAlert, Info, AlertTriangle, AlertOctagon, Search, ChevronLeft, ChevronRight } from 'lucide-react';

export default function AuditLogTable() {
  const [searchTerm, setSearchTerm] = useState('');
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const [currentCursor, setCurrentCursor] = useState<string | undefined>(undefined);

  // We explicitly fetch only user_activity per requirements.
  const { data: response, isPending, isFetching, isError, error } = useQuery({
    queryKey: ['audit-logs', currentCursor, searchTerm],
    queryFn: () => auditApi.getLogs({
      category: 'user_activity',
      limit: 15, // standard page size
      cursor: currentCursor,
      search: searchTerm || undefined,
    }),
    placeholderData: keepPreviousData,
  });

  const handleNextPage = () => {
    if (response?.next_cursor) {
      setCursorStack([...cursorStack, currentCursor || '']);
      setCurrentCursor(response.next_cursor);
    }
  };

  const handlePrevPage = () => {
    if (cursorStack.length > 0) {
      const newStack = [...cursorStack];
      const prevCursor = newStack.pop();
      setCursorStack(newStack);
      setCurrentCursor(prevCursor === '' ? undefined : prevCursor);
    }
  };

  const handleSearch = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchTerm(e.target.value);
    setCursorStack([]);
    setCurrentCursor(undefined);
  };

  if (isPending) {
    return <div className="flex bg-[#12182b] border border-slate-800 rounded-lg p-10 items-center justify-center min-h-[400px]">
      <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-blue-500"></div>
    </div>;
  }

  if (isError) {
    return <div className="bg-red-900/20 border border-red-500/50 rounded-lg p-6 text-red-400">
      <h3 className="text-lg font-medium mb-2">Error loading audit logs</h3>
      <p>{error instanceof Error ? error.message : 'Unknown error occurred'}</p>
    </div>;
  }

  const logs = response?.data || [];
  const hasNextPage = !!response?.next_cursor;

  const getSeverityIcon = (severity: string) => {
    switch (severity?.toLowerCase()) {
      case 'info': return <Info size={16} className="text-blue-400" />;
      case 'warning': return <AlertTriangle size={16} className="text-yellow-400" />;
      case 'error': return <AlertOctagon size={16} className="text-red-400" />;
      case 'critical': return <ShieldAlert size={16} className="text-red-500" />;
      default: return <Info size={16} className="text-slate-400" />;
    }
  };

  const getSeverityColor = (severity: string) => {
    switch (severity?.toLowerCase()) {
      case 'info': return 'bg-blue-500/10 text-blue-400 border-blue-500/20';
      case 'warning': return 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20';
      case 'error': return 'bg-red-500/10 text-red-400 border-red-500/20';
      case 'critical': return 'bg-red-500/20 text-red-500 border-red-500/30';
      default: return 'bg-slate-500/10 text-slate-400 border-slate-500/20';
    }
  };

  return (
    <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2 duration-500">
      <div className="bg-[#12182b] border border-slate-800 rounded-lg overflow-hidden shadow-xl">
        <div className="p-5 border-b border-slate-800 bg-[#1a2235] flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold text-slate-100 tracking-wide">User Activity Logs</h2>
            <p className="text-sm text-slate-400 mt-1">Immutable record of all authenticated actions and behaviors within the platform.</p>
          </div>
          <div className="relative group">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 group-focus-within:text-blue-400 transition-colors" size={18} />
            <input
              type="text"
              placeholder="Search actions or details..."
              value={searchTerm}
              onChange={handleSearch}
              className="bg-[#0f141f] border border-slate-700 text-slate-200 rounded-lg pl-10 pr-4 py-2 text-sm focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-all w-full sm:w-64 placeholder:text-slate-600"
            />
          </div>
        </div>
        
        <div className="overflow-x-auto relative min-h-[300px]">
          {isFetching && !isPending && (
            <div className="absolute inset-0 bg-[#12182b]/50 backdrop-blur-sm z-10 flex items-center justify-center">
               <div className="animate-spin rounded-full h-6 w-6 border-t-2 border-b-2 border-blue-500"></div>
            </div>
          )}
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="bg-[#0f141f] text-slate-400 font-medium border-b border-slate-800/80">
              <tr>
                <th className="px-6 py-4">Timestamp</th>
                <th className="px-6 py-4">Actor</th>
                <th className="px-6 py-4">Action</th>
                <th className="px-6 py-4">Target</th>
                <th className="px-6 py-4">Source IP</th>
                <th className="px-6 py-4">Severity</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/40 text-slate-300">
              {logs.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-slate-500 bg-[#151b2b]">
                    {searchTerm ? 'No user activity logs match your search.' : 'No user activity logs available.'}
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr key={log.id} className="hover:bg-slate-800/40 transition-colors group">
                    <td className="px-6 py-4 text-slate-400 text-xs font-mono group-hover:text-slate-300 transition-colors">
                      {new Date(log.timestamp).toLocaleString()}
                    </td>
                    <td className="px-6 py-4 font-medium text-slate-200">
                      {log.actor_label || 'unknown'}
                    </td>
                    <td className="px-6 py-4 font-mono text-xs text-blue-400/90 group-hover:text-blue-400 transition-colors">
                      {log.action}
                    </td>
                    <td className="px-6 py-4 text-slate-400">
                      {log.target_label || '-'}
                    </td>
                    <td className="px-6 py-4 font-mono text-xs text-slate-500">
                      {log.source_ip || '-'}
                    </td>
                    <td className="px-6 py-4">
                      <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium tracking-wide ${getSeverityColor(log.severity)}`}>
                        {getSeverityIcon(log.severity)}
                        {log.severity}
                      </span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        
        {/* Pagination Controls */}
        <div className="p-4 border-t border-slate-800 bg-[#0f141f] flex items-center justify-between text-sm">
          <div className="text-slate-500">
            {logs.length > 0 ? 'Showing results' : 'No results'}
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={handlePrevPage}
              disabled={cursorStack.length === 0 || isFetching}
              className="p-2 rounded-md border border-slate-700 text-slate-300 bg-[#1a2235] hover:bg-slate-700 hover:text-white disabled:opacity-50 disabled:cursor-not-allowed transition-all"
              aria-label="Previous page"
            >
              <ChevronLeft size={16} />
            </button>
            <button
              onClick={handleNextPage}
              disabled={!hasNextPage || isFetching}
              className="p-2 rounded-md border border-slate-700 text-slate-300 bg-[#1a2235] hover:bg-slate-700 hover:text-white disabled:opacity-50 disabled:cursor-not-allowed transition-all"
              aria-label="Next page"
            >
              <ChevronRight size={16} />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
