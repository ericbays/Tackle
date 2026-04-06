import { Outlet, NavLink } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';

export default function AppShell() {
  const logout = useAuthStore((state) => state.logout);

  return (
    <div className="min-h-screen bg-[#0a0f1a] text-slate-200 flex">
      <aside className="w-64 border-r border-slate-800 bg-[#12182b] flex flex-col">
        <div className="p-6 border-b border-slate-800 border-opacity-50">
          <h2 className="text-2xl font-bold bg-gradient-to-r from-blue-400 to-blue-600 bg-clip-text text-transparent">Tackle</h2>
        </div>
        <nav className="flex-1 p-4 flex flex-col gap-2">
          <NavLink to="/dashboard" className={({isActive}) => `p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}>Dashboard</NavLink>
          <NavLink to="/campaigns" className={({isActive}) => `p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}>Campaigns</NavLink>
          <NavLink to="/email-templates" className={({isActive}) => `p-3 rounded-md transition-all ${isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'}`}>Email Templates</NavLink>
        </nav>
        <div className="p-4 border-t border-slate-800 border-opacity-50">
          <button onClick={logout} className="w-full text-left p-3 rounded-md text-red-400 hover:bg-slate-800 transition-colors">Log out</button>
        </div>
      </aside>
      <main className="flex-1 flex flex-col">
        <header className="h-16 border-b border-slate-800 border-opacity-50 flex items-center px-6 bg-[#12182b]/80 backdrop-blur-sm">
          <p className="text-sm text-slate-500 bg-slate-900 border border-slate-700 px-3 py-1.5 rounded-md">Search... <kbd className="font-mono text-xs text-slate-600 ml-2">Ctrl K</kbd></p>
        </header>
        <div className="flex-1 p-8 overflow-y-auto">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
