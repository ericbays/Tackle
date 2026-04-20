import { useState } from 'react';
import { useBuilderStore } from '../../store/builderStore';
import { Route, Plus, Trash2, Clock, GitCommit, MousePointerClick, CheckSquare } from 'lucide-react';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { Button } from '../ui/Button';

export const WorkflowEditor = () => {
    const project = useBuilderStore(state => state.project);
    const updateNavigation = useBuilderStore(state => state.updateNavigation);

    if (!project || !project.definition_json) return null;

    const pages = project.definition_json.pages || [];
    const navigation = project.definition_json.navigation || [];

    const handleAddFlow = () => {
        const newFlow = {
            id: `flow-${crypto.randomUUID().slice(0, 8)}`,
            source_page: pages[0]?.route || '/',
            trigger: 'redirect',
            target_page: '',
            delay_ms: 0,
            component_id: ''
        };
        updateNavigation([...navigation, newFlow]);
    };

    const handleUpdateFlow = (index: number, key: string, value: any) => {
        const newNav = [...navigation];
        newNav[index] = { ...newNav[index], [key]: value };
        updateNavigation(newNav);
    };

    const handleRemoveFlow = (index: number) => {
        const newNav = [...navigation];
        newNav.splice(index, 1);
        updateNavigation(newNav);
    };

    const getTriggerIcon = (trigger: string) => {
        switch(trigger) {
            case 'form_submit': return <CheckSquare className="w-3.5 h-3.5 text-green-400" />;
            case 'click': return <MousePointerClick className="w-3.5 h-3.5 text-blue-400" />;
            case 'redirect': return <Clock className="w-3.5 h-3.5 text-amber-400" />;
            default: return <GitCommit className="w-3.5 h-3.5 text-slate-400" />;
        }
    };

    return (
        <div className="flex-1 flex flex-col bg-slate-900 overflow-hidden">
            <div className="p-4 border-b border-slate-800 space-y-3">
                <div className="text-xs text-slate-400 leading-relaxed font-mono">
                    Global Interceptors. Map event triggers to routing actions and simulated delays.
                </div>
                <Button 
                    onClick={handleAddFlow}
                    
                 variant="primary" size="sm">
                    <Plus className="w-4 h-4" /> ADD EVENT FLOW
                </Button>
            </div>

            <div className="flex-1 overflow-y-auto p-3 space-y-3">
                {navigation.map((flow, idx) => (
                    <div key={flow.id || idx} className="bg-slate-800/40 border border-slate-700/60 rounded p-3 relative group">
                        <Button variant="destructive" 
                            
                            onClick={ () => handleRemoveFlow(idx)}
                            title="Delete Flow"
                        >
                            <Trash2 className="w-3.5 h-3.5" />
                        </Button>
                        
                        <div className="flex items-center gap-2 mb-4 pr-6">
                            {getTriggerIcon(flow.trigger)}
                            <span className="text-xs font-bold text-slate-300 uppercase tracking-wider">
                                {flow.trigger.replace('_', ' ')}
                            </span>
                        </div>

                        <div className="space-y-3">
                            <div className="space-y-1">
                                <label className="text-[10px] text-slate-500 uppercase tracking-widest">Source Page</label>
                                <Select 
                                    
                                    value={flow.source_page}
                                    onChange={e => handleUpdateFlow(idx, 'source_page', e.target.value)}
                                >
                                    <option value="">-- Any Page --</option>
                                    {pages.map((p: any) => (
                                        <option key={p.page_id} value={p.route}>{p.name} ({p.route})</option>
                                    ))}
                                </Select>
                            </div>

                            <div className="space-y-1">
                                <label className="text-[10px] text-slate-500 uppercase tracking-widest">Trigger Mechanism</label>
                                <Select 
                                    
                                    value={flow.trigger}
                                    onChange={e => handleUpdateFlow(idx, 'trigger', e.target.value)}
                                >
                                    <option value="redirect">Auto Delay (Timer)</option>
                                    <option value="form_submit">Form Submission</option>
                                    <option value="click">Element Click</option>
                                </Select>
                            </div>

                            {(flow.trigger === 'form_submit' || flow.trigger === 'click') && (
                                <div className="space-y-1">
                                    <label className="text-[10px] text-slate-500 uppercase tracking-widest">Trigger Target DOM ID (Optional)</label>
                                    <Input 
                                        
                                        value={flow.component_id || ''}
                                        onChange={e => handleUpdateFlow(idx, 'component_id', e.target.value)}
                                        placeholder="system-login-form"
                                    />
                                </div>
                            )}

                            <div className="flex gap-2">
                                <div className="space-y-1 flex-[2]">
                                    <label className="text-[10px] text-slate-500 uppercase tracking-widest">Target Route</label>
                                    <Input 
                                        
                                        value={flow.target_page || ''}
                                        onChange={e => handleUpdateFlow(idx, 'target_page', e.target.value)}
                                        placeholder="/mfa"
                                    />
                                </div>
                                <div className="space-y-1 flex-1">
                                    <label className="text-[10px] text-slate-500 uppercase tracking-widest">Delay (Ms)</label>
                                    <Input 
                                        type="number"
                                        
                                        value={flow.delay_ms || 0}
                                        onChange={e => handleUpdateFlow(idx, 'delay_ms', parseInt(e.target.value) || 0)}
                                    />
                                </div>
                            </div>
                        </div>
                    </div>
                ))}
                
                {navigation.length === 0 && (
                    <div className="text-center p-6 border border-dashed border-slate-700/50 rounded text-slate-500 text-sm mt-4 relative overflow-hidden bg-slate-900">
                        <Route className="w-8 h-8 text-slate-700 mx-auto mb-2 opacity-50" />
                        No event workflows defined.<br /> Add a flow to intercept actions.
                    </div>
                )}
            </div>
        </div>
    );
};
