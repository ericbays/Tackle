import { Outlet } from 'react-router-dom';

export default function AuthLayout() {
  return (
    <div className="min-h-screen bg-[#0a0f1a] flex flex-col justify-center items-center p-4">
      <h1 className="text-3xl font-bold text-slate-100 mb-8 border-b border-primary/50 pb-2">Tackle Dashboard</h1>
      <div className="glass-panel rounded-2xl w-full max-w-md p-8">
        <Outlet />
      </div>
    </div>
  );
}
