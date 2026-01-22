import { useState, useEffect, useCallback } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useToast } from '@/hooks/use-toast';
import {
  ApiClient,
  type AccountResponse,
  type UsageResponse,
  type EnhancedUsageResponse,
  type DayUsage,
  type SessionResponse
} from '@/lib/api';
import { getStoredApiKey, clearApiKey } from '@/lib/auth';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  BarChart,
  Bar
} from 'recharts';



export function Dashboard() {
  const [account, setAccount] = useState<AccountResponse | null>(null);
  const [usage, setUsage] = useState<UsageResponse | null>(null);
  const [enhancedUsage, setEnhancedUsage] = useState<EnhancedUsageResponse | null>(null);
  const [sessions, setSessions] = useState<SessionResponse[]>([]);
  const [selectedDays, setSelectedDays] = useState<string>('7');
  const [isLoading, setIsLoading] = useState(true);
  const { toast } = useToast();
  const navigate = useNavigate();

  const fetchData = useCallback(async (days: number = 7) => {
    const storedKey = getStoredApiKey();
    if (!storedKey) {
      navigate('/');
      return;
    }

    const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

    const fetchWithRetry = async <T,>(fn: () => Promise<T>, retries = 3): Promise<T> => {
      for (let i = 0; i < retries; i++) {
        try {
          return await fn();
        } catch (e) {
          if (i === retries - 1) throw e;
          await delay(1000 * Math.pow(2, i));
        }
      }
      throw new Error('Retry failed');
    };

    try {
      const client = new ApiClient(storedKey);
      const [accountInfo, usageStats, enhancedStats, sessionsList] = await fetchWithRetry(() =>
        Promise.all([
          client.getAccount(),
          client.getUsage(),
          client.getEnhancedUsage(days),
          client.listSessions(),
        ])
      );

      setAccount(accountInfo);
      setUsage(usageStats);
      setEnhancedUsage(enhancedStats);
      setSessions(sessionsList);
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
  }, [navigate, toast]);

  useEffect(() => {
    fetchData(parseInt(selectedDays));
  }, [fetchData, selectedDays]);

  const handleDaysChange = (value: string) => {
    setSelectedDays(value);
    setIsLoading(true);
    fetchData(parseInt(value));
  };

  const handleCopyApiKey = () => {
    const fullApiKey = getStoredApiKey();

    if (fullApiKey) {
      navigator.clipboard.writeText(fullApiKey);
      toast({
        title: 'API key copied to clipboard',
      });
    } else {
      toast({
        title: 'Full API key only shown once at creation',
        description: 'For security, the full API key is only available when you first create it.',
      });
    }
  };

  const handleExportUsage = async () => {
    const storedKey = getStoredApiKey();
    if (!storedKey) return;

    try {
      const client = new ApiClient(storedKey);
      const data = await client.exportUsage(parseInt(selectedDays));

      const csv = convertToCSV(data);
      const blob = new Blob([csv], { type: 'text/csv' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `usage-export-${new Date().toISOString().split('T')[0]}.csv`;
      a.click();
      URL.revokeObjectURL(url);

      toast({
        title: 'Export Complete',
        description: 'Usage data has been downloaded as CSV',
      });
    } catch {
      toast({
        title: 'Export Failed',
        description: 'Failed to export usage data',
        variant: 'destructive',
      });
    }
  };

  const convertToCSV = (data: DayUsage[]): string => {
    const headers = ['Date', 'Executions', 'Duration (ms)', 'Cost (cents)', 'Errors'];
    const rows = data.map(d => [d.date, d.executions, d.duration_ms, d.cost_cents, d.errors].join(','));
    return [headers.join(','), ...rows].join('\n');
  };

  const handleLogout = () => {
    clearApiKey();
    navigate('/');
  };

  const handleStopSession = async (id: string) => {
    const storedKey = getStoredApiKey();
    if (!storedKey) return;

    try {
      const client = new ApiClient(storedKey);
      await client.stopSession(id);
      toast({
        title: 'Session stopped',
        description: `Session ${id.slice(0, 8)} has been stopped`,
      });
      await fetchData(parseInt(selectedDays));
    } catch {
      toast({
        title: 'Failed to stop session',
        description: 'An error occurred while stopping the session',
        variant: 'destructive',
      });
    }
  };

  const handleKillSession = async (id: string) => {
    const storedKey = getStoredApiKey();
    if (!storedKey) return;

    try {
      const client = new ApiClient(storedKey);
      await client.killSession(id);
      toast({
        title: 'Session killed',
        description: `Session ${id.slice(0, 8)} has been killed`,
      });
      await fetchData(parseInt(selectedDays));
    } catch {
      toast({
        title: 'Failed to kill session',
        description: 'An error occurred while killing the session',
        variant: 'destructive',
      });
    }
  };

  if (isLoading && !account) {
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
  const quotaPercentage = isUnlimited || dailyLimit === 0 ? 0 : ((quotaUsed / dailyLimit) * 100);

  // Prepare chart data
  const dailyChartData = (enhancedUsage?.daily_history ?? [])
    .slice()
    .reverse()
    .map(d => ({
      date: d.date?.slice(5) || '', // Show MM-DD format
      executions: d.executions ?? 0,
      cost: (d.cost_cents ?? 0) / 100,
    }));

  const hourlyChartData = (enhancedUsage?.hourly_usage ?? []).map(h => ({
    hour: h.hour?.slice(11, 16) || '', // Show HH:MM format
    executions: h.executions ?? 0,
  }));

  const totalCost = (enhancedUsage?.cost_estimate_cents ?? 0) / 100;

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
              <div>
                Est. Cost: ${totalCost.toFixed(2)}
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

      {/* Usage History Section */}
      <Card>
        <CardHeader>
          <div className="flex justify-between items-center">
            <div>
              <CardTitle>Usage History</CardTitle>
              <CardDescription>
                Track your execution history and costs over time
              </CardDescription>
            </div>
            <div className="flex gap-2">
              <Select value={selectedDays} onValueChange={handleDaysChange}>
                <SelectTrigger className="w-[120px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="7">Last 7 days</SelectItem>
                  <SelectItem value="30">Last 30 days</SelectItem>
                  <SelectItem value="90">Last 90 days</SelectItem>
                </SelectContent>
              </Select>
              <Button variant="outline" size="sm" onClick={handleExportUsage}>
                Export CSV
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {dailyChartData.length > 0 ? (
            <div className="h-[300px]">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={dailyChartData}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis
                    dataKey="date"
                    className="text-xs"
                    tick={{ fill: 'hsl(var(--muted-foreground))' }}
                  />
                  <YAxis
                    className="text-xs"
                    tick={{ fill: 'hsl(var(--muted-foreground))' }}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'hsl(var(--background))',
                      border: '1px solid hsl(var(--border))',
                      borderRadius: '6px'
                    }}
                    labelStyle={{ color: 'hsl(var(--foreground))' }}
                  />
                  <Area
                    type="monotone"
                    dataKey="executions"
                    stroke="hsl(var(--primary))"
                    fill="hsl(var(--primary))"
                    fillOpacity={0.3}
                    name="Executions"
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <div className="h-[300px] flex items-center justify-center text-muted-foreground">
              No usage data available yet
            </div>
          )}
        </CardContent>
      </Card>

      {/* Hourly Activity Section */}
      {hourlyChartData.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Today's Activity</CardTitle>
            <CardDescription>
              Hourly breakdown of executions in the last 24 hours
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[200px]">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={hourlyChartData}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis
                    dataKey="hour"
                    className="text-xs"
                    tick={{ fill: 'hsl(var(--muted-foreground))' }}
                  />
                  <YAxis
                    className="text-xs"
                    tick={{ fill: 'hsl(var(--muted-foreground))' }}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'hsl(var(--background))',
                      border: '1px solid hsl(var(--border))',
                      borderRadius: '6px'
                    }}
                    labelStyle={{ color: 'hsl(var(--foreground))' }}
                  />
                  <Bar
                    dataKey="executions"
                    fill="hsl(var(--primary))"
                    radius={[4, 4, 0, 0]}
                    name="Executions"
                  />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Sessions Section */}
      <Card>
        <CardHeader>
          <CardTitle>Sessions</CardTitle>
          <CardDescription>
            Your active and recent sessions
          </CardDescription>
        </CardHeader>
        <CardContent>
          {sessions.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-2 px-2 font-medium">ID</th>
                    <th className="text-left py-2 px-2 font-medium">Status</th>
                    <th className="text-left py-2 px-2 font-medium">Image</th>
                    <th className="text-left py-2 px-2 font-medium">Created</th>
                    <th className="text-right py-2 px-2 font-medium">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {sessions.map((session) => (
                    <tr key={session.id} className="border-b last:border-0">
                      <td className="py-3 px-2 font-mono text-xs">
                        {session.id.slice(0, 8)}
                      </td>
                      <td className="py-3 px-2">
                        <span
                          className={`inline-block px-2 py-1 rounded-full text-xs font-medium ${
                            session.status === 'running'
                              ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                              : session.status === 'stopped'
                              ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                              : 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200'
                          }`}
                        >
                          {session.status}
                        </span>
                      </td>
                      <td className="py-3 px-2 text-muted-foreground">
                        {session.image}
                      </td>
                      <td className="py-3 px-2 text-muted-foreground">
                        {new Date(session.createdAt).toLocaleString()}
                      </td>
                      <td className="py-3 px-2 text-right">
                        <div className="flex gap-2 justify-end">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleStopSession(session.id)}
                            disabled={session.status !== 'running'}
                          >
                            Stop
                          </Button>
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={() => handleKillSession(session.id)}
                          >
                            Kill
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="text-center py-8 text-muted-foreground">
              No sessions
            </div>
          )}
        </CardContent>
      </Card>

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
