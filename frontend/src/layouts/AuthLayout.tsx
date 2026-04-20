import { Outlet } from 'react-router-dom';

export default function AuthLayout() {
  return (
    <div className="min-h-screen bg-[#0a0f1a] flex flex-col justify-center items-center px-4 pt-4 pb-32">
      <div className="flex flex-col items-center mb-8">
        <img src="/logo.png" alt="Tackle Logo" className="w-[350px] mb-4 object-contain" />
        <h1 className="text-5xl font-bold bg-gradient-to-r from-blue-400 to-blue-600 bg-clip-text text-transparent">Tackle</h1>
      </div>
      <div className="glass-panel rounded-2xl w-full max-w-md p-8">
        <Outlet />
      </div>
    </div>
  );
}
