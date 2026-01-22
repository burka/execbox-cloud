import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useToast } from '@/hooks/use-toast';
import { setStoredApiKey, getStoredApiKey } from '@/lib/auth';
import { joinWaitlist, ApiClient } from '@/lib/api';

export function Landing() {
  const [showModal, setShowModal] = useState(false);
  const [showLoginModal, setShowLoginModal] = useState(false);
  const [email, setEmail] = useState('');
  const [existingKey, setExistingKey] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  
  // Waitlist form state
  const [name, setName] = useState('');
  const [company, setCompany] = useState('');
  const [useCase, setUseCase] = useState('');
  const [usageIntent, setUsageIntent] = useState('');
  const [budgetRange, setBudgetRange] = useState('');
  
  const { toast } = useToast();
  const navigate = useNavigate();

  // Check if already logged in
  const isLoggedIn = !!getStoredApiKey();

  const handleJoinWaitlist = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!name || !email || !useCase || !usageIntent || !budgetRange) {
      toast({
        title: 'Missing fields',
        description: 'Please fill in all required fields',
        variant: 'destructive',
      });
      return;
    }
    
    setIsLoading(true);

    try {
      // Call the waitlist API
      const response = await joinWaitlist(email, name);
      
      // Store the API key
      setStoredApiKey(response.key);

      // Show success message with API key
      toast({
        title: 'Welcome to execbox-cloud!',
        description: `Your API key: ${response.key}. Redirecting to dashboard...`,
        duration: 5000,
      });

      // Reset form and close modal
      setName('');
      setEmail('');
      setCompany('');
      setUseCase('');
      setUsageIntent('');
      setBudgetRange('');
      setShowModal(false);

      // Redirect to dashboard after a short delay
      setTimeout(() => {
        navigate('/dashboard');
      }, 1000);
    } catch (error) {
      toast({
        title: 'Error',
        description: error instanceof Error ? error.message : 'Failed to join waitlist',
        variant: 'destructive',
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);

    try {
      // Basic format validation
      if (!existingKey.startsWith('sk_')) {
        toast({
          title: 'Invalid API key format',
          description: 'API keys start with sk_',
          variant: 'destructive',
        });
        return;
      }

      // Validate the key by calling the API
      const client = new ApiClient(existingKey);
      await client.getAccount();

      // If validation succeeds, store the key
      setStoredApiKey(existingKey);

      toast({
        title: 'Logged in!',
        description: 'Redirecting to dashboard...',
      });

      setTimeout(() => {
        navigate('/dashboard');
      }, 500);
    } catch (error) {
      toast({
        title: 'Authentication failed',
        description: error instanceof Error ? error.message : 'Invalid API key',
        variant: 'destructive',
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-gradient-to-b from-background to-muted/20">
      <div className="container max-w-4xl px-4 text-center space-y-8">
        <h1 className="text-5xl font-bold tracking-tight sm:text-6xl">
          execbox-cloud
        </h1>

        <p className="text-xl text-muted-foreground max-w-2xl mx-auto">
          Secure sandboxed code execution as a service
        </p>

        <div className="flex justify-center gap-4 pt-4">
          {isLoggedIn ? (
            <>
              <Button size="lg" className="text-lg px-8 py-6" onClick={() => navigate('/dashboard')}>
                Go to Dashboard
              </Button>
              <Button size="lg" variant="ghost" className="text-lg px-8 py-6" asChild>
                <a href="https://github.com/burka/execbox" target="_blank" rel="noopener noreferrer">
                  GitHub
                </a>
              </Button>
            </>
          ) : (
            <>
              <Button size="lg" className="text-lg px-8 py-6" onClick={() => setShowModal(true)}>
                Join Waitlist
              </Button>
              <Button size="lg" variant="outline" className="text-lg px-8 py-6" onClick={() => setShowLoginModal(true)}>
                Login
              </Button>
              <Button size="lg" variant="ghost" className="text-lg px-8 py-6" asChild>
                <a href="https://github.com/burka/execbox" target="_blank" rel="noopener noreferrer">
                  GitHub
                </a>
              </Button>
            </>
          )}
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

      {showModal && (
        <div 
          className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" 
          onClick={() => setShowModal(false)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setShowModal(false);
          }}
          tabIndex={-1}
        >
          <Card className="w-full max-w-lg m-4 max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
            <CardHeader>
              <CardTitle>Join the Waitlist</CardTitle>
              <CardDescription>
                Get early access to execbox-cloud and help shape the future of code execution
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleJoinWaitlist} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="name">Name *</Label>
                  <Input
                    id="name"
                    type="text"
                    required
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Your name"
                    disabled={isLoading}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="email">Email *</Label>
                  <Input
                    id="email"
                    type="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="your@email.com"
                    disabled={isLoading}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="company">Company</Label>
                  <Input
                    id="company"
                    type="text"
                    value={company}
                    onChange={(e) => setCompany(e.target.value)}
                    placeholder="Your company (optional)"
                    disabled={isLoading}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="useCase">Use Case *</Label>
                  <Textarea
                    id="useCase"
                    required
                    value={useCase}
                    onChange={(e) => setUseCase(e.target.value)}
                    placeholder="Describe what you want to build with execbox-cloud..."
                    rows={3}
                    disabled={isLoading}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="usageIntent">Usage Intent *</Label>
                  <Select value={usageIntent} onValueChange={setUsageIntent} disabled={isLoading}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select your usage intent" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="personal">Personal projects (light usage)</SelectItem>
                      <SelectItem value="team">Small team collaboration</SelectItem>
                      <SelectItem value="production">Production workloads (high value)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="budgetRange">Budget Range *</Label>
                  <Select value={budgetRange} onValueChange={setBudgetRange} disabled={isLoading}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select your budget range" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="free">Free tier only</SelectItem>
                      <SelectItem value="10-50">$10-50/month</SelectItem>
                      <SelectItem value="50+">$50+/month</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex gap-2">
                  <Button type="submit" className="flex-1" disabled={isLoading}>
                    {isLoading ? 'Joining...' : 'Join Waitlist'}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setShowModal(false)}
                    disabled={isLoading}
                  >
                    Cancel
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </div>
      )}

      {showLoginModal && (
        <div 
          className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" 
          onClick={() => setShowLoginModal(false)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setShowLoginModal(false);
          }}
          tabIndex={-1}
        >
          <Card className="w-full max-w-md m-4" onClick={(e) => e.stopPropagation()}>
            <CardHeader>
              <CardTitle>Login with API Key</CardTitle>
              <CardDescription>
                Enter your existing API key to access the dashboard
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleLogin} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="apiKey">API Key</Label>
                  <Input
                    id="apiKey"
                    type="password"
                    required
                    value={existingKey}
                    onChange={(e) => setExistingKey(e.target.value)}
                    placeholder="sk_live_..."
                    disabled={isLoading}
                  />
                </div>
                <div className="flex gap-2">
                  <Button type="submit" className="flex-1" disabled={isLoading}>
                    {isLoading ? 'Logging in...' : 'Login'}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setShowLoginModal(false)}
                    disabled={isLoading}
                  >
                    Cancel
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
