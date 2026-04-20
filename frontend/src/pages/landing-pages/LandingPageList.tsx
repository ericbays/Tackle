import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Plus, Search, FileCode2, Clock, FileEdit, Trash2, ChevronLeft, ChevronRight } from 'lucide-react';
import { api } from '../../services/api';
import toast from 'react-hot-toast';
import { formatDistanceToNow } from 'date-fns';
import { Input } from '../../components/ui/Input';
import { Button } from '../../components/ui/Button';

interface LandingPage {
  id: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
  created_by: string;
  status?: string;
}

interface PaginatedResponse {
  data: LandingPage[];
  meta: {
    page: number;
    per_page: number;
    total: number;
    total_pages: number;
  }
}

export default function LandingPageList() {
  const navigate = useNavigate();
  const [searchTerm, setSearchTerm] = useState('');
  const [page, setPage] = useState(1);
  const perPage = 12;

  // Use a debounced search term for the API call to prevent spamming
  const [debouncedSearch, setDebouncedSearch] = useState('');

  // Simple debounce
  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedSearch(searchTerm);
      setPage(1); // Reset to page 1 on new search
    }, 300);
    return () => clearTimeout(handler);
  }, [searchTerm]);

  const { data: responseData, isLoading, refetch } = useQuery<PaginatedResponse>({
    queryKey: ['landing-pages', page, debouncedSearch],
    queryFn: async () => {
      const res = await api.get('/landing-pages', {
        params: {
          page,
          per_page: perPage,
          ...(debouncedSearch ? { name: debouncedSearch } : {})
        }
      });
      // Handle array vs envelope
      const rawData = res.data.data || res.data;
      const pagination = res.data.pagination;
      
      if (pagination) {
          return {
              data: rawData,
              meta: {
                  page: pagination.page,
                  per_page: pagination.per_page,
                  total: pagination.total,
                  total_pages: pagination.total_pages
              }
          };
      }
      
      return { 
          data: rawData, 
          meta: { page: 1, per_page: 100, total: rawData.length, total_pages: 1 } 
      };
    }
  });

  const handleCreateNew = async () => {
    try {
      const uniqueSuffix = new Date().getTime().toString().slice(-4);
      const res = await api.post('/landing-pages', {
        name: `New Landing Page ${uniqueSuffix}`,
        description: 'Auto-generated via UI'
      });
      const newPage = res.data.data || res.data;
      navigate(`/builder/${newPage.id}`);
    } catch (error: any) {
      toast.error(error.response?.data?.message || 'Failed to create landing page');
    }
  };

  const [deletingId, setDeletingId] = useState<string | null>(null);

  const handleDelete = async (id: string) => {
    try {
      await api.delete(`/landing-pages/${id}`);
      toast.success('Landing page deleted');
      setDeletingId(null);
      refetch();
    } catch (error) {
      toast.error('Failed to delete landing page');
    }
  };

  const pages = responseData?.data || [];
  const meta = responseData?.meta;

  return (
    <div className="flex-1 max-w-7xl mx-auto w-full px-6 py-8">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-slate-100 flex items-center gap-3">
            <FileCode2 className="w-7 h-7 text-blue-500" />
            Landing Applications
          </h1>
          <p className="text-sm text-slate-400 mt-2">
            Build and manage full React+Go web applications deployed on dynamic framework ports.
          </p>
        </div>
        <Button 
          onClick={handleCreateNew}
          
         variant="primary">
          <Plus className="w-5 h-5" />
          Create Application
        </Button>
      </div>

      <div className="flex justify-between items-center bg-slate-800/50 p-4 rounded-lg border border-slate-700/50 mb-6">
        <div className="relative w-96">
          <Search className="w-5 h-5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
          <Input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Search landing applications..."
            className="w-full bg-slate-900 border border-slate-700 rounded-md py-2 pl-10 pr-4 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
          />
        </div>
        
        {/* Pagination Controls */}
        {meta && meta.total_pages > 1 && (
            <div className="flex items-center gap-4 text-sm text-slate-300">
                <span>Page {meta.page} of {meta.total_pages}</span>
                <div className="flex items-center gap-1">
                    <Button variant="outline" 
                        disabled={meta.page <= 1}
                        onClick={ () => setPage(p => Math.max(1, p - 1))}
                        className="p-1 px-2 rounded bg-slate-700 hover:bg-slate-600 disabled:opacity-30 disabled:hover:bg-slate-700 transition"
                    >
                        <ChevronLeft className="w-4 h-4" />
                    </Button>
                    <Button variant="outline"
                        disabled={meta.page >= meta.total_pages}
                        onClick={() => setPage(p => Math.min(meta.total_pages, p + 1))}
                        className="p-1 px-2 rounded bg-slate-700 hover:bg-slate-600 disabled:opacity-30 disabled:hover:bg-slate-700 transition"
                    >
                        <ChevronRight className="w-4 h-4" />
                    </Button>
                </div>
            </div>
        )}
      </div>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-blue-500"></div>
        </div>
      ) : pages.length === 0 ? (
        <div className="bg-slate-800/20 border border-slate-700/50 rounded-lg p-12 text-center">
            <FileCode2 className="w-12 h-12 text-slate-500 mx-auto mb-4" />
            <h3 className="text-lg font-medium text-slate-300">No landing applications found</h3>
            <p className="text-sm text-slate-400 mt-2 max-w-md mx-auto">
              You haven't built any landing applications yet, or none match your search. Create a new one to get started.
            </p>
            <Button 
                onClick={handleCreateNew}
                
             variant="outline">
                <Plus className="w-5 h-5" /> Create Application
            </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {pages.map(page => (
            <div key={page.id} className="bg-slate-800/50 border border-slate-700 rounded-lg overflow-hidden group hover:border-slate-500 transition-colors flex flex-col">
              <div className="p-6 flex-1">
                <div className="flex justify-between items-start mb-4">
                  <h3 className="font-semibold text-lg text-slate-200 line-clamp-1">{page.name}</h3>
                </div>
                <p className="text-xs text-slate-400 mb-6 line-clamp-2">
                  {page.description || 'No description provided.'}
                </p>
                <div className="flex items-center gap-4 text-xs text-slate-500">
                  <div className="flex items-center gap-1.5" title={`Updated at: ${new Date(page.updated_at).toLocaleString()}`}>
                    <Clock className="w-3.5 h-3.5" />
                    <span>Updated {formatDistanceToNow(new Date(page.updated_at), { addSuffix: true })}</span>
                  </div>
                </div>
              </div>
              <div className="bg-slate-900 border-t border-slate-700 p-4 flex gap-2 justify-end">
                {deletingId === page.id ? (
                  <div className="flex items-center gap-2 mr-auto bg-red-950/30 px-2 rounded border border-red-900/50">
                    <span className="text-xs text-red-400">Confirm delete?</span>
                    <Button variant="outline" onClick={ (e) => { e.stopPropagation(); handleDelete(page.id); }} className="text-xs text-white bg-red-600 hover:bg-red-500 px-2 py-1 rounded">Yes</Button>
                    <Button variant="outline" onClick={ (e) => { e.stopPropagation(); setDeletingId(null); }} className="text-xs text-slate-300 hover:bg-slate-700 px-2 py-1 rounded">No</Button>
                  </div>
                ) : (
                  <Button variant="outline" 
                    onClick={ (e) => { e.stopPropagation(); setDeletingId(page.id); }}
                    className="p-2 text-slate-400 hover:text-red-400 hover:bg-slate-800 rounded-md transition-colors"
                    title="Delete"
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                )}
                <Button variant="outline" 
                  onClick={ () => navigate(`/builder/${page.id}`)}
                  className="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600/10 hover:bg-blue-600/20 text-blue-400 text-sm font-medium rounded transition-colors"
                >
                  <FileEdit className="w-4 h-4" />
                  Builder Editor
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
