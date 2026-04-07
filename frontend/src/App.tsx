import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthGuard, GuestGuard } from './components/AuthGuard';
import AuthLayout from './layouts/AuthLayout';
import AppShell from './layouts/AppShell';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Builder from './pages/Builder';
import CampaignList from './pages/campaigns/CampaignList';
import WorkspaceLayout from './pages/campaigns/WorkspaceLayout';
import EmailTemplatesPage from './pages/email-templates/EmailTemplatesPage';
import EmailTemplateEditor from './pages/email-templates/EmailTemplateEditor';
import UserManagementPage from './features/users/pages/UserManagementPage';
import EngineeringPage from './features/engineering/pages/EngineeringPage';
import { Toaster } from 'react-hot-toast';

const queryClient = new QueryClient();

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <Toaster 
        position="bottom-right" 
        toastOptions={{
          style: {
            background: '#1e293b', /* bg-slate-800 */
            color: '#f8fafc',      /* text-slate-50 */
            border: '1px solid #334155', /* border-slate-700 */
            fontSize: '14px',
            boxShadow: '0 10px 15px -3px rgb(0 0 0 / 0.5), 0 4px 6px -4px rgb(0 0 0 / 0.5)',
            marginBottom: '40px'
          },
          success: {
            iconTheme: {
              primary: '#3b82f6',
              secondary: '#1e293b',
            },
          },
          error: {
            iconTheme: {
              primary: '#ef4444',
              secondary: '#1e293b',
            },
          },
        }}
      />
      <BrowserRouter>
        <Routes>
          <Route element={<GuestGuard />}>
            <Route element={<AuthLayout />}>
              <Route path="/login" element={<Login />} />
            </Route>
          </Route>

          <Route element={<AuthGuard />}>
            <Route path="/builder/:id" element={<Builder />} />
            <Route element={<AppShell />}>
              <Route path="/dashboard" element={<Dashboard />} />
              <Route path="/campaigns" element={<CampaignList />} />
              <Route path="/campaigns/:id/*" element={<WorkspaceLayout />} />
              <Route path="/email-templates" element={<EmailTemplatesPage />} />
              <Route path="/email-templates/:id" element={<EmailTemplateEditor />} />
              <Route path="/engineering" element={<EngineeringPage />} />
              <Route path="/users" element={<UserManagementPage />} />
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
            </Route>
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
