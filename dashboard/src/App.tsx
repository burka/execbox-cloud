import { lazy, Suspense } from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { Layout } from './components/Layout';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Toaster } from './components/ui/toaster';

const Landing = lazy(() => import('./pages/Landing').then(m => ({ default: m.Landing })));
const Dashboard = lazy(() => import('./pages/Dashboard').then(m => ({ default: m.Dashboard })));
const RequestQuota = lazy(() => import('./pages/RequestQuota').then(m => ({ default: m.RequestQuota })));
const KeysPage = lazy(() => import('./pages/KeysPage').then(m => ({ default: m.KeysPage })));

function App() {
  return (
    <Router>
      <Layout>
        <Suspense fallback={<div className="flex items-center justify-center min-h-screen"><p>Loading...</p></div>}>
          <Routes>
            <Route path="/" element={<Landing />} />
            <Route
              path="/dashboard"
              element={
                <ProtectedRoute>
                  <Dashboard />
                </ProtectedRoute>
              }
            />
            <Route path="/request-quota" element={<RequestQuota />} />
            <Route
              path="/keys"
              element={
                <ProtectedRoute>
                  <KeysPage />
                </ProtectedRoute>
              }
            />
          </Routes>
        </Suspense>
      </Layout>
      <Toaster />
    </Router>
  );
}

export default App;
