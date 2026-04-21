import { Link, useParams } from 'react-router-dom';
import { LayoutTemplate, ExternalLink, MousePointerClick, RefreshCw, AlertCircle } from 'lucide-react';
import { useCampaignStore } from '../../../store/campaignStore';
import { useLandingPages } from '../../../hooks/useConfigurations';

export default function LandingPageTab() {
    const { id } = useParams();
    const { draft: { landingPageId: selectedPage }, updateLandingPage, isSaving, saveCampaign } = useCampaignStore();
    
    const { data: landingPages = [], isLoading } = useLandingPages();

    return (
        <div className="space-y-8">
            <section className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden p-6">
                <div className="flex items-center justify-between mb-6">
                    <div className="flex items-center gap-3">
                        <LayoutTemplate className="w-5 h-5 text-purple-400" />
                        <h2 className="text-sm font-semibold uppercase tracking-widest text-slate-500">Landing Page Assignment</h2>
                    </div>
                    <Link 
                        to={selectedPage ? `/builder/${selectedPage}` : `#`} 
                        className={`text-sm px-3 py-1.5 rounded-md transition-colors flex items-center gap-2 ${selectedPage ? 'bg-slate-800 hover:bg-slate-700 text-white' : 'bg-slate-800/50 text-slate-500 cursor-not-allowed pointer-events-none'}`}
                    >
                        <ExternalLink className="w-4 h-4" /> Open Builder
                    </Link>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {isLoading ? (
                         <div className="col-span-2 text-slate-500 text-sm py-8 flex items-center justify-center">
                             Loading landing applications from backend...
                         </div>
                    ) : landingPages.length === 0 ? (
                        <div className="col-span-2 text-amber-500 text-sm bg-amber-950/20 border border-amber-900/50 rounded-lg p-6 flex flex-col items-center justify-center">
                             <AlertCircle className="w-8 h-8 mb-3 text-amber-500/80" />
                             <p>No landing applications found.</p>
                             <Link to="/landing-pages" className="text-blue-400 hover:underline mt-2">Create your first landing application</Link>
                         </div>
                    ) : (
                        landingPages.map(page => (
                            <div 
                                key={page.id}
                                onClick={() => updateLandingPage(page.id)}
                                className={`border rounded-lg p-5 cursor-pointer transition-all ${
                                    selectedPage === page.id 
                                    ? 'border-purple-500 bg-purple-500/10' 
                                    : 'border-slate-800 bg-slate-950/50 hover:border-slate-700 hover:bg-slate-800/50'
                                }`}
                            >
                                <div className="flex items-start justify-between mb-3">
                                    <div>
                                        <h3 className="font-semibold text-slate-200">{page.name}</h3>
                                        <span className="text-xs text-slate-500">{page.template_type || 'Custom'}</span>
                                    </div>
                                    <div className={`w-4 h-4 rounded-full border-2 flex items-center justify-center ${
                                        selectedPage === page.id ? 'border-purple-500' : 'border-slate-700'
                                    }`}>
                                        {selectedPage === page.id && <div className="w-2 h-2 bg-purple-500 rounded-full" />}
                                    </div>
                                </div>
                                
                                <div className="text-xs text-slate-400 flex items-center gap-4 mt-4">
                                    <span className="flex items-center gap-1.5"><MousePointerClick className="w-3 h-3" /> {page.views || 0} Clicks</span>
                                    <span className="flex items-center gap-1.5"><RefreshCw className="w-3 h-3" /> Updated {page.updated_at ? new Date(page.updated_at).toLocaleDateString() : 'Recently'}</span>
                                </div>
                            </div>
                        ))
                    )}
                </div>

                <div className="mt-8 border border-slate-800 rounded-lg overflow-hidden bg-slate-950/30">
                    <div className="p-4 border-b border-slate-800 bg-slate-900/50 flex justify-between items-center">
                        <span className="text-sm font-medium text-slate-300">Preview: {landingPages.find(p => p.id === selectedPage)?.name || 'None selected'}</span>
                    </div>
                    <div className="h-64 flex items-center justify-center text-slate-500 text-sm italic">
                        {selectedPage ? 'Interactive preview loading...' : 'Select a page above to preview'}
                    </div>
                </div>
            </section>

            <div className="flex justify-end pt-4">
                <button 
                    onClick={() => saveCampaign(id)}
                    disabled={isSaving}
                    className="bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white px-6 py-2.5 rounded-md font-medium transition-colors"
                >
                    {isSaving ? 'Saving...' : 'Save Landing Page'}
                </button>
            </div>
        </div>
    );
}
