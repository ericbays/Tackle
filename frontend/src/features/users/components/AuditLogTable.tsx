import React, { useState } from 'react';
import { useQuery, keepPreviousData } from '@tanstack/react-query';
import { auditApi } from '../services/auditApi';
import { ShieldAlert, Info, AlertTriangle, AlertOctagon, Search, ChevronLeft, ChevronRight } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '../../../components/ui/Card';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/Table';
import { Input } from '../../../components/ui/Input';
import { Button } from '../../../components/ui/Button';
import { Badge } from '../../../components/ui/Badge';

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

  const getSeverityVariant = (severity: string) => {
    switch (severity?.toLowerCase()) {
      case 'info': return 'default';
      case 'warning': return 'warning';
      case 'error': return 'destructive';
      case 'critical': return 'destructive';
      default: return 'outline';
    }
  };

  return (
    <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2 duration-500">
      <Card className="shadow-xl">
        <CardHeader className="bg-[#1a2235] border-b border-slate-800 rounded-t-xl flex-row items-center justify-between gap-4 py-5">
          <div className="flex flex-col space-y-1.5">
            <CardTitle>User Activity Logs</CardTitle>
            <CardDescription>Immutable record of all authenticated actions and behaviors within the platform.</CardDescription>
          </div>
          <div className="relative group w-full sm:w-64">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 group-focus-within:text-blue-400 transition-colors z-10" size={18} />
            <Input
              type="text"
              placeholder="Search actions or details..."
              value={searchTerm}
              onChange={handleSearch}
              className="pl-10 h-9"
            />
          </div>
        </CardHeader>
        
        <Table className="border-0 shadow-none rounded-none rounded-b-xl border-t-0 p-0 relative min-h-[300px]">
          {isFetching && !isPending && (
            <div className="absolute inset-0 bg-[#12182b]/50 backdrop-blur-sm z-10 flex items-center justify-center">
               <div className="animate-spin rounded-full h-6 w-6 border-t-2 border-b-2 border-blue-500"></div>
            </div>
          )}
          <TableHeader>
            <TableRow>
              <TableHead>Timestamp</TableHead>
              <TableHead>Actor</TableHead>
              <TableHead>Action</TableHead>
              <TableHead>Target</TableHead>
              <TableHead>Source IP</TableHead>
              <TableHead>Severity</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {logs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="py-12 text-center text-slate-500">
                  {searchTerm ? 'No user activity logs match your search.' : 'No user activity logs available.'}
                </TableCell>
              </TableRow>
            ) : (
              logs.map((log) => (
                <TableRow key={log.id} className="group">
                  <TableCell className="text-slate-400 text-xs font-mono group-hover:text-slate-300 transition-colors">
                    {new Date(log.timestamp).toLocaleString()}
                  </TableCell>
                  <TableCell className="font-medium text-slate-200">
                    {log.actor_label || 'unknown'}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-blue-400/90 group-hover:text-blue-400 transition-colors">
                    {log.action}
                  </TableCell>
                  <TableCell className="text-slate-400">
                    {log.target_label || '-'}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-500">
                    {log.source_ip || '-'}
                  </TableCell>
                  <TableCell className="whitespace-nowrap">
                    <Badge variant={getSeverityVariant(log.severity) as any}>
                      {getSeverityIcon(log.severity)}
                      <span className="ml-1.5">{log.severity}</span>
                    </Badge>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
          <CardFooter className="py-4 border-t border-slate-800 bg-[#0f141f] flex items-center justify-between text-sm w-full mx-0 px-6 rounded-b-xl">
            <div className="text-slate-500">
              {logs.length > 0 ? 'Showing results' : 'No results'}
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="icon"
                onClick={handlePrevPage}
                disabled={cursorStack.length === 0 || isFetching}
                aria-label="Previous page"
              >
                <ChevronLeft size={16} />
              </Button>
              <Button
                variant="outline"
                size="icon"
                onClick={handleNextPage}
                disabled={!hasNextPage || isFetching}
                aria-label="Next page"
              >
                <ChevronRight size={16} />
              </Button>
            </div>
          </CardFooter>
        </Table>
      </Card>
    </div>
  );
}
