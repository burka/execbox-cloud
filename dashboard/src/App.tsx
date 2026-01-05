import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { Layout } from './components/Layout';
import { Landing } from './pages/Landing';
import { Dashboard } from './pages/Dashboard';
import { RequestQuota } from './pages/RequestQuota';
import { Toaster } from './components/ui/toaster';

function App() {
  return (
    <Router>
      <Layout>
        <Routes>
          <Route path="/" element={<Landing />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/request-quota" element={<RequestQuota />} />
        </Routes>
      </Layout>
      <Toaster />
    </Router>
  );
}

export default App;
