import { motion } from 'framer-motion';
import { Shield, Server, Mail, User } from 'lucide-react';

export interface ActivityLog {
  id: string;
  type: 'campaign' | 'infrastructure' | 'system' | 'security';
  timestamp: string;
  actor: string;
  description: string;
}

interface ActivityFeedProps {
  logs: ActivityLog[];
}

const icons = {
  campaign: { icon: Mail, color: 'text-green-400', bg: 'bg-green-500/10' },
  infrastructure: { icon: Server, color: 'text-blue-400', bg: 'bg-blue-500/10' },
  system: { icon: User, color: 'text-slate-400', bg: 'bg-slate-500/10' },
  security: { icon: Shield, color: 'text-red-400', bg: 'bg-red-500/10' },
};

const containerVars = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { staggerChildren: 0.1 }
  }
};

const itemVars = {
  hidden: { opacity: 0, x: -10 },
  show: { opacity: 1, x: 0 }
};

export function ActivityFeed({ logs }: ActivityFeedProps) {
  return (
    <div className="glass-panel rounded-xl p-6 h-full flex flex-col">
      <h3 className="text-lg font-bold text-slate-200 mb-4 border-b border-slate-800 pb-2">Recent Activity</h3>
      
      <motion.div 
        variants={containerVars}
        initial="hidden"
        animate="show"
        className="flex-1 overflow-auto flex flex-col gap-3 pr-2"
      >
        {logs.map((log) => {
          const cfg = icons[log.type];
          const Icon = cfg.icon;
          return (
            <motion.div key={log.id} variants={itemVars} className="flex gap-3 items-start p-2 rounded-md hover:bg-slate-800/30 transition-colors">
              <div className={`p-1.5 rounded-md ${cfg.bg} mt-0.5`}>
                <Icon className={`w-4 h-4 ${cfg.color}`} />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm text-slate-300 truncate">{log.description}</p>
                <div className="flex items-center gap-2 mt-1">
                  <span className="text-xs text-slate-500">{log.timestamp}</span>
                  <span className="text-xs text-slate-600 font-medium">• {log.actor}</span>
                </div>
              </div>
            </motion.div>
          );
        })}
      </motion.div>
      
      <div className="pt-4 mt-2 border-t border-slate-800">
        <button className="text-sm text-blue-400 hover:text-blue-300 flex items-center transition-colors">
          View all logs <span className="ml-1">→</span>
        </button>
      </div>
    </div>
  );
}
