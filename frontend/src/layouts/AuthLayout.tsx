import { Outlet } from 'react-router-dom';

export default function AuthLayout() {
  return (
    <div className="min-h-screen bg-[#0a0f1a] flex flex-col justify-center items-center px-4 pt-4 pb-32">
      <div className="flex flex-col items-center mb-8">
        <img src="/logo.png" alt="Tackle Logo" className="w-[350px] mb-4 object-contain" />
        <h1 className="text-3xl font-bold text-slate-100 border-b border-primary/50 pb-2">Tackle</h1>
      </div>
      <div className="glass-panel rounded-2xl w-full max-w-md p-8">
        <Outlet />
      </div>
    </div>
  );
}
