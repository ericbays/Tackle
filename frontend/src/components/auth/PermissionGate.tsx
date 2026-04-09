import { ReactNode } from 'react';
import { useAuthStore } from '../../store/authStore';

interface PermissionGateProps {
  children: ReactNode;
  permission: string;
}

/**
 * PermissionGate wraps UI elements and only renders them if the currently
 * authenticated user has the required permission string in their role claims.
 * Admins inherently bypass this check.
 */
export default function PermissionGate({ children, permission }: PermissionGateProps) {
  const user = useAuthStore((state) => state.user);

  if (!user) return null;

  // Global admin override
  if (user.role === 'admin') return <>{children}</>;

  // Check specific permission claim
  if (user.permissions && user.permissions.includes(permission)) {
    return <>{children}</>;
  }

  // Not authorized to view
  return null;
}
