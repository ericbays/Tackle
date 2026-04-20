import { useState } from 'react';
import { useAuthStore } from '../store/authStore';
import { api } from '../services/api';
import toast from 'react-hot-toast';
import { Input } from '../components/ui/Input';
import { Button } from '../components/ui/Button';

export default function Login() {
  const setToken = useAuthStore((state) => state.setToken);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await api.post('/auth/login', { username: email, password });
      setToken(res.data.data.access_token);
    } catch (error) {
      toast.error('Login failed. Please check your credentials and try again.');
      console.error('Login error:', error);
    }
  };

  return (
    <form onSubmit={handleLogin} className="flex flex-col gap-4">
      <div>
        <label className="block text-sm font-medium text-slate-400 mb-1">Username / Email</label>
        <Input 
          type="text" 
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-slate-400 mb-1">Password</label>
        <Input 
          type="password" 
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </div>
      <Button variant="primary" type="submit" className="mt-4 w-full">
        Log In
      </Button>
    </form>
  );
}
