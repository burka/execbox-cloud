import { Link, useLocation } from 'react-router-dom';

interface LayoutProps {
  children: React.ReactNode;
}

export function Layout({ children }: LayoutProps) {
  const location = useLocation();
  const isLanding = location.pathname === '/';

  return (
    <div className="min-h-screen bg-background">
      {!isLanding && (
        <header className="border-b">
          <div className="container mx-auto px-4 py-4">
            <nav className="flex items-center justify-between">
              <Link to="/" className="text-xl font-bold">
                execbox-cloud
              </Link>
              <div className="flex gap-4">
                <Link
                  to="/dashboard"
                  className="text-sm font-medium hover:text-primary"
                >
                  Dashboard
                </Link>
                <Link
                  to="/keys"
                  className="text-sm font-medium hover:text-primary"
                >
                  API Keys
                </Link>
                <Link
                  to="/request-quota"
                  className="text-sm font-medium hover:text-primary"
                >
                  Request Quota
                </Link>
              </div>
            </nav>
          </div>
        </header>
      )}
      <main className={isLanding ? '' : 'container mx-auto px-4 py-8'}>
        {children}
      </main>
    </div>
  );
}
