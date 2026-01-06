import { useState, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useToast } from '@/hooks/use-toast';
import { ApiClient, type AccountResponse, type UsageResponse } from '@/lib/api';
import { getStoredApiKey, clearApiKey } from '@/lib/auth';

export function Dashboard() {
  const [account, setAccount] = useState<AccountResponse | null>(null);
  const [usage, setUsage] = useState<UsageResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const { toast } = useToast();
  const navigate = useNavigate();

  useEffect(() => {
    const fetchData = async () => {
      const storedKey = getStoredApiKey();
      if (!storedKey) {
        navigate('/');
        return;
      }

      try {
        const client = new ApiClient(storedKey);
        const [accountInfo, usageStats] = await Promise.all([
          client.getAccount(),
          client.getUsage(),
        ]);

        setAccount(accountInfo);
        setUsage(usageStats);
      } catch {
        toast({
          title: 'Error',
          description: 'Failed to fetch account data. Please try logging in again.',
          variant: 'destructive',
        });
        clearApiKey();
        navigate('/');
      } finally {
        setIsLoading(false);
      }
    };

    fetchData();
  }, [navigate, toast]);

  const handleCopyApiKey = () => {
    if (account?.api_key_preview) {
      // We can only copy the preview since the full key is not stored
      navigator.clipboard.writeText(account.api_key_preview);
      toast({
        title: 'Copied!',
        description: 'API key preview copied to clipboard',
      });
    }
  };

  const handleLogout = () => {
    clearApiKey();
    navigate('/');
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  // Calculate quota values
  const quotaUsed = usage?.quota_used ?? 0;
  const dailyLimit = usage?.daily_limit ?? 0;
  const isUnlimited = dailyLimit === -1;
  const quotaPercentage = isUnlimited ? 0 : ((quotaUsed / dailyLimit) * 100);

  return (
    <div className="space-y-8">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
          <p className="text-muted-foreground">
            Manage your API keys and monitor usage
          </p>
        </div>
        <Button variant="outline" onClick={handleLogout}>
          Logout
        </Button>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Account</CardTitle>
            <CardDescription>
              Your account information
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>API Key</Label>
              <div className="flex gap-2">
                <Input
                  type="password"
                  value={account?.api_key_preview || ''}
                  readOnly
                  className="font-mono"
                />
                <Button onClick={handleCopyApiKey} variant="outline">
                  Copy
                </Button>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span className="text-muted-foreground">Tier:</span>
                <span className="ml-2 font-medium capitalize">{account?.tier}</span>
              </div>
              {account?.email && (
                <div>
                  <span className="text-muted-foreground">Email:</span>
                  <span className="ml-2 font-medium">{account.email}</span>
                </div>
              )}
            </div>
            <p className="text-sm text-muted-foreground">
              Keep your API key secure. Do not share it publicly.
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Usage Statistics</CardTitle>
            <CardDescription>
              Your current usage and quota information
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">Sessions Today</span>
                <span className="font-medium">{usage?.sessions_today ?? 0}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">Active Sessions</span>
                <span className="font-medium">{usage?.active_sessions ?? 0}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">Daily Quota</span>
                <span className="font-medium">
                  {isUnlimited
                    ? `${quotaUsed} / Unlimited`
                    : `${quotaUsed} / ${dailyLimit}`}
                </span>
              </div>
              {!isUnlimited && (
                <div className="mt-2 h-2 bg-secondary rounded-full overflow-hidden">
                  <div
                    className="h-full bg-primary"
                    style={{ width: `${Math.min(quotaPercentage, 100)}%` }}
                  />
                </div>
              )}
            </div>
            <div className="grid grid-cols-2 gap-2 text-sm text-muted-foreground">
              <div>
                Concurrent Limit: {usage?.concurrent_limit === -1 ? 'Unlimited' : usage?.concurrent_limit}
              </div>
              <div>
                Max Duration: {Math.floor((usage?.max_duration_seconds ?? 0) / 60)}min
              </div>
              <div>
                Max Memory: {usage?.max_memory_mb}MB
              </div>
            </div>
            <div className="pt-2">
              <Link to="/request-quota">
                <Button variant="outline" className="w-full">
                  Request More Quota
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Getting Started</CardTitle>
          <CardDescription>
            Quick example of how to use the API
          </CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="bg-muted p-4 rounded-lg overflow-x-auto text-sm">
            <code>{`# Create a session
curl -X POST https://api.execbox-cloud.com/v1/sessions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "image": "python:3.11",
    "command": ["python", "-c", "print(\\"Hello, World!\\")"]
  }'

# WebSocket attach to session
wscat -c "wss://api.execbox-cloud.com/v1/sessions/SESS_ID/attach" \\
  -H "Authorization: Bearer YOUR_API_KEY"`}</code>
          </pre>
        </CardContent>
      </Card>
    </div>
  );
}
