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

const queryClient = new QueryClient();

function App() {
  return (
    <QueryClientProvider client={queryClient}>
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
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
            </Route>
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
