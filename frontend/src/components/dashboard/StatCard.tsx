
import { type LucideIcon } from 'lucide-react';
import { motion } from 'framer-motion';

interface StatCardProps {
  title: string;
  value: string;
  icon: LucideIcon;
  trend?: string;
  trendUp?: boolean;
}

export function StatCard({ title, value, icon: Icon, trend, trendUp }: StatCardProps) {
  return (
    <motion.div 
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      className="glass-panel p-6 rounded-xl flex items-center gap-4 hover:border-slate-600/50 transition-colors cursor-pointer group"
    >
      <div className="p-3 bg-slate-800/50 rounded-lg group-hover:bg-blue-500/10 transition-colors">
        <Icon className="w-6 h-6 text-blue-400" />
      </div>
      <div>
        <p className="text-sm text-slate-400 font-medium">{title}</p>
        <div className="flex items-end gap-2">
          <h3 className="text-2xl font-bold text-slate-100">{value}</h3>
          {trend && (
            <span className={`text-xs font-semibold mb-1 ${trendUp ? 'text-green-400' : 'text-red-400'}`}>
              {trendUp ? '↑' : '↓'} {trend}
            </span>
          )}
        </div>
      </div>
    </motion.div>
  );
}
