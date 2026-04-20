import React, { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { userApi } from '../services/userApi';
import { X } from 'lucide-react';
import toast from 'react-hot-toast';
import { Input } from '../../../components/ui/Input';
import { Button } from '../../../components/ui/Button';

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
          <Button onClick={onClose}  variant="ghost">
            <X size={20} />
          </Button>
        </div>
        
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Username</label>
            <Input 
              type="text" 
              required
              
              value={formData.username}
              onChange={e => setFormData({...formData, username: e.target.value})}
            />
          </div>
          
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Email Address</label>
            <Input 
              type="email" 
              required
              
              value={formData.email}
              onChange={e => setFormData({...formData, email: e.target.value})}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Display Name</label>
            <Input 
              type="text" 
              required
              
              value={formData.display_name}
              onChange={e => setFormData({...formData, display_name: e.target.value})}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Initial Password</label>
            <Input 
              type="password" 
              required
              
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
            <Button 
              type="button" 
              onClick={onClose}
              
             variant="ghost">
              Cancel
            </Button>
            <Button 
              type="submit" 
              disabled={mutation.isPending}
              
             variant="primary">
              {mutation.isPending ? 'Creating...' : 'Create User'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
