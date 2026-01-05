import { Link } from 'react-router-dom';
import { Button } from '@/components/ui/button';

export function Landing() {
  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-gradient-to-b from-background to-muted/20">
      <div className="container max-w-4xl px-4 text-center space-y-8">
        <h1 className="text-5xl font-bold tracking-tight sm:text-6xl">
          execbox-cloud
        </h1>

        <p className="text-xl text-muted-foreground max-w-2xl mx-auto">
          Secure sandboxed code execution as a service
        </p>

        <div className="flex justify-center pt-4">
          <Link to="/dashboard">
            <Button size="lg" className="text-lg px-8 py-6">
              Get API Key
            </Button>
          </Link>
        </div>

        <div className="pt-12 max-w-3xl mx-auto">
          <h2 className="text-2xl font-semibold mb-6">Features</h2>
          <div className="grid md:grid-cols-3 gap-6 text-left">
            <div className="space-y-2">
              <h3 className="font-medium">Secure Sandboxing</h3>
              <p className="text-sm text-muted-foreground">
                Execute code in isolated containers with strict resource limits
              </p>
            </div>
            <div className="space-y-2">
              <h3 className="font-medium">Multiple Languages</h3>
              <p className="text-sm text-muted-foreground">
                Support for Python, JavaScript, Go, and more
              </p>
            </div>
            <div className="space-y-2">
              <h3 className="font-medium">RESTful API</h3>
              <p className="text-sm text-muted-foreground">
                Simple HTTP API for easy integration
              </p>
            </div>
            <div className="space-y-2">
              <h3 className="font-medium">Fast Execution</h3>
              <p className="text-sm text-muted-foreground">
                Low-latency code execution with optimized containers
              </p>
            </div>
            <div className="space-y-2">
              <h3 className="font-medium">Usage Tracking</h3>
              <p className="text-sm text-muted-foreground">
                Monitor your API usage and quota in real-time
              </p>
            </div>
            <div className="space-y-2">
              <h3 className="font-medium">Scalable</h3>
              <p className="text-sm text-muted-foreground">
                Automatic scaling to handle your workload
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
