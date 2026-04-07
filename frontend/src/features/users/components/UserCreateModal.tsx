import React, { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { userApi } from '../services/userApi';
import { X } from 'lucide-react';
import toast from 'react-hot-toast';

interface UserCreateModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export default function UserCreateModal({ isOpen, onClose }: UserCreateModalProps) {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState({
    username: '',
    email: '',
    display_name: '',
    password: '',
    force_password_change: true,
  });

  const mutation = useMutation({
    mutationFn: userApi.createUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      toast.success('User created successfully');
      onClose();
      setFormData({ username: '', email: '', display_name: '', password: '', force_password_change: true });
    },
    onError: (err: any) => {
      toast.error(`Failed to create user: ${err.response?.data?.message || err.message}`);
    }
  });

  if (!isOpen) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    mutation.mutate(formData);
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-[#12182b] border border-slate-800 rounded-xl shadow-2xl w-full max-w-md overflow-hidden">
        <div className="flex justify-between items-center p-6 border-b border-slate-800">
          <h2 className="text-xl font-bold text-slate-100">Create New User</h2>
          <button onClick={onClose} className="text-slate-400 hover:text-white transition-colors">
            <X size={20} />
          </button>
        </div>
        
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Username</label>
            <input 
              type="text" 
              required
              className="w-full bg-[#0a0f1a] border border-slate-700 rounded-lg px-4 py-2.5 text-slate-200 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
              value={formData.username}
              onChange={e => setFormData({...formData, username: e.target.value})}
            />
          </div>
          
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Email Address</label>
            <input 
              type="email" 
              required
              className="w-full bg-[#0a0f1a] border border-slate-700 rounded-lg px-4 py-2.5 text-slate-200 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
              value={formData.email}
              onChange={e => setFormData({...formData, email: e.target.value})}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Display Name</label>
            <input 
              type="text" 
              required
              className="w-full bg-[#0a0f1a] border border-slate-700 rounded-lg px-4 py-2.5 text-slate-200 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
              value={formData.display_name}
              onChange={e => setFormData({...formData, display_name: e.target.value})}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Initial Password</label>
            <input 
              type="password" 
              required
              className="w-full bg-[#0a0f1a] border border-slate-700 rounded-lg px-4 py-2.5 text-slate-200 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
              value={formData.password}
              onChange={e => setFormData({...formData, password: e.target.value})}
            />
          </div>

          <div className="pt-2 flex items-center">
            <input 
              type="checkbox" 
              id="force_pw"
              checked={formData.force_password_change}
              onChange={e => setFormData({...formData, force_password_change: e.target.checked})}
              className="w-4 h-4 rounded border-slate-700 bg-[#0a0f1a] text-blue-500 focus:ring-blue-500 focus:ring-offset-slate-900"
            />
            <label htmlFor="force_pw" className="ml-2 text-sm text-slate-400">
              Force password change on first login
            </label>
          </div>

          <div className="pt-6 flex justify-end gap-3">
            <button 
              type="button" 
              onClick={onClose}
              className="px-4 py-2 rounded-lg text-slate-300 hover:text-white hover:bg-slate-800 transition-colors"
            >
              Cancel
            </button>
            <button 
              type="submit" 
              disabled={mutation.isPending}
              className="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white px-6 py-2 rounded-lg font-medium transition-colors"
            >
              {mutation.isPending ? 'Creating...' : 'Create User'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
