import { Link } from 'react-router-dom';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

export function Dashboard() {
  const apiKey = 'sk_test_1234567890abcdef'; // Placeholder
  const sessionsToday = 42; // Placeholder
  const quotaRemaining = 1000; // Placeholder
  const quotaTotal = 5000; // Placeholder

  const handleCopyApiKey = () => {
    navigator.clipboard.writeText(apiKey);
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-muted-foreground">
          Manage your API keys and monitor usage
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>API Key</CardTitle>
            <CardDescription>
              Use this key to authenticate your API requests
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="api-key">Your API Key</Label>
              <div className="flex gap-2">
                <Input
                  id="api-key"
                  type="password"
                  value={apiKey}
                  readOnly
                  className="font-mono"
                />
                <Button onClick={handleCopyApiKey} variant="outline">
                  Copy
                </Button>
              </div>
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
                <span className="font-medium">{sessionsToday}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">Quota Remaining</span>
                <span className="font-medium">
                  {quotaRemaining} / {quotaTotal}
                </span>
              </div>
              <div className="mt-2 h-2 bg-secondary rounded-full overflow-hidden">
                <div
                  className="h-full bg-primary"
                  style={{
                    width: `${((quotaTotal - quotaRemaining) / quotaTotal) * 100}%`,
                  }}
                />
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
            <code>{`curl -X POST https://api.execbox-cloud.com/v1/execute \\
  -H "Authorization: Bearer ${apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "language": "python",
    "code": "print(\\"Hello, World!\\")"
  }'`}</code>
          </pre>
        </CardContent>
      </Card>
    </div>
  );
}
