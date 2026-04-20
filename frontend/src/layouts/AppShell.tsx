import { Outlet, NavLink } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';
import { LayoutDashboard, Target, Server, Mail, FileCode2, Users, LogOut, Send, Contact, Activity } from 'lucide-react';
import { GlobalSearch } from '../components/GlobalSearch';
import { Button } from '../components/ui/Button';
export default function AppShell() {
  const logout = useAuthStore((state) => state.logout);

  return (
    <div className="min-h-screen bg-[#0a0f1a] text-slate-200 flex">
      <aside className="w-64 border-r border-slate-800 bg-[#12182b] flex flex-col">
        <div className="h-16 px-6 border-b border-slate-800 border-opacity-50 flex items-center gap-3 shrink-0">
          <img src="/logo.png" alt="Tackle Logo" className="h-12 w-auto object-contain" />
          <h2 className="text-2xl font-bold bg-gradient-to-r from-blue-400 to-blue-600 bg-clip-text text-transparent">Tackle</h2>
        </div>
        <nav className="flex-1 p-4 flex flex-col gap-2">
          <NavLink to="/dashboard" className={({isActive}) => `flex items-center gap-3 p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}><LayoutDashboard className="w-5 h-5" />Dashboard</NavLink>
          <NavLink to="/campaigns" className={({isActive}) => `flex items-center gap-3 p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}><Target className="w-5 h-5" />Campaigns</NavLink>
          <NavLink to="/targets" className={({isActive}) => `flex items-center gap-3 p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}><Contact className="w-5 h-5" />Target Directory</NavLink>
          <NavLink to="/engineering" className={({isActive}) => `flex items-center gap-3 p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}><Server className="w-5 h-5" />Engineering</NavLink>
          <NavLink to="/email-templates" className={({isActive}) => `flex items-center gap-3 p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}><Mail className="w-5 h-5" />Email Templates</NavLink>
          <NavLink to="/landing-pages" className={({isActive}) => `flex items-center gap-3 p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}><FileCode2 className="w-5 h-5" />Landing Applications</NavLink>
          <NavLink to="/smtp-profiles" className={({isActive}) => `flex items-center gap-3 p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}><Send className="w-5 h-5" />SMTP Profiles</NavLink>
          <NavLink to="/users" className={({isActive}) => `flex items-center gap-3 p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}><Users className="w-5 h-5" />Users & Roles</NavLink>
          <NavLink to="/logs" className={({isActive}) => `flex items-center gap-3 p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}><Activity className="w-5 h-5" />Global Logs</NavLink>
        </nav>
        <div className="p-4 border-t border-slate-800 border-opacity-50">
          <Button variant="ghost" onClick={logout} className="w-full flex items-center justify-start gap-3 text-red-400 hover:text-red-300"><LogOut className="w-5 h-5" />Log out</Button>
        </div>
      </aside>
      <main className="flex-1 flex flex-col">
        <header className="h-16 border-b border-slate-800 border-opacity-50 flex items-center px-6 bg-[#12182b]/80 backdrop-blur-sm z-50">
          <GlobalSearch />
        </header>
        <div className="flex-1 p-8 overflow-y-auto">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
