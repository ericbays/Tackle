import { useState } from 'react';
import { Users, ShieldAlert } from 'lucide-react';
import TargetList from './components/TargetList';
import BlocklistManager from './components/BlocklistManager';
import { Button } from '../../components/ui/Button';

export default function TargetManagementPage() {
    const [activeTab, setActiveTab] = useState<'targets' | 'blocklist'>('targets');

    return (
        <div className="max-w-7xl mx-auto space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold text-white flex items-center gap-3">
                        <Users className="w-8 h-8 text-blue-500" />
                        Target Management
                    </h1>
                    <p className="text-slate-400 mt-1">Directory of individuals and global blocklist controls.</p>
                </div>
            </div>

            <div className="flex space-x-1 border-b border-slate-700/50">
                <Button variant="outline"
                    onClick={ () => setActiveTab('targets')}
                    className={`flex items-center gap-2 px-6 py-3 text-sm font-medium border-b-2 transition-colors
                        ${activeTab === 'targets' 
                            ? 'border-blue-500 text-blue-400' 
                            : 'border-transparent text-slate-400 hover:text-slate-300 hover:border-slate-600'}`
                    }
                >
                    <Users className="w-4 h-4" />
                    Global Targets
                </Button>
                <Button variant="outline"
                    onClick={ () => setActiveTab('blocklist')}
                    className={`flex items-center gap-2 px-6 py-3 text-sm font-medium border-b-2 transition-colors
                        ${activeTab === 'blocklist' 
                            ? 'border-red-500 text-red-400' 
                            : 'border-transparent text-slate-400 hover:text-slate-300 hover:border-slate-600'}`
                    }
                >
                    <ShieldAlert className="w-4 h-4" />
                    Blocklist Rules
                </Button>
            </div>

            <div className="mt-6">
                {activeTab === 'targets' && <TargetList />}
                {activeTab === 'blocklist' && <BlocklistManager />}
            </div>
        </div>
    );
}
