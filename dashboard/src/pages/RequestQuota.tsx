import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useToast } from '@/hooks/use-toast';
import { submitQuotaRequest } from '@/lib/api';

export function RequestQuota() {
  const { toast } = useToast();
  const [isLoading, setIsLoading] = useState(false);
  const [formData, setFormData] = useState({
    email: '',
    name: '',
    company: '',
    use_case: '',
    requested_limits: '',
    budget: '',
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);

    try {
      await submitQuotaRequest(formData);

      toast({
        title: 'Request Submitted',
        description: 'We will review your quota request and get back to you soon.',
      });

      // Reset form
      setFormData({
        email: '',
        name: '',
        company: '',
        use_case: '',
        requested_limits: '',
        budget: '',
      });
    } catch (error) {
      toast({
        title: 'Error',
        description: error instanceof Error ? error.message : 'Failed to submit quota request',
        variant: 'destructive',
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setFormData({
      ...formData,
      [e.target.name]: e.target.value,
    });
  };

  return (
    <div className="max-w-2xl mx-auto space-y-8">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Request More Quota</h1>
        <p className="text-muted-foreground">
          Tell us about your use case and we'll review your request
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Quota Request Form</CardTitle>
          <CardDescription>
            Fill out the form below to request increased API quota
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-6">
            <div className="space-y-2">
              <Label htmlFor="email">
                Email <span className="text-destructive">*</span>
              </Label>
              <Input
                id="email"
                name="email"
                type="email"
                required
                value={formData.email}
                onChange={handleChange}
                placeholder="your@email.com"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="name">
                Name <span className="text-destructive">*</span>
              </Label>
              <Input
                id="name"
                name="name"
                type="text"
                required
                value={formData.name}
                onChange={handleChange}
                placeholder="John Doe"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="company">Company (Optional)</Label>
              <Input
                id="company"
                name="company"
                type="text"
                value={formData.company}
                onChange={handleChange}
                placeholder="Acme Inc."
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="use_case">
                Use Case <span className="text-destructive">*</span>
              </Label>
              <textarea
                id="use_case"
                name="use_case"
                required
                value={formData.use_case}
                onChange={handleChange}
                placeholder="Describe how you plan to use execbox-cloud..."
                className="flex min-h-[120px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="requested_limits">
                Requested Limits <span className="text-destructive">*</span>
              </Label>
              <Input
                id="requested_limits"
                name="requested_limits"
                type="text"
                required
                value={formData.requested_limits}
                onChange={handleChange}
                placeholder="e.g., 100,000 executions/month"
              />
              <p className="text-sm text-muted-foreground">
                Specify the quota you need (executions per month, concurrent sessions, etc.)
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="budget">Budget (Optional)</Label>
              <Input
                id="budget"
                name="budget"
                type="text"
                value={formData.budget}
                onChange={handleChange}
                placeholder="e.g., $500/month"
              />
              <p className="text-sm text-muted-foreground">
                Your expected monthly budget helps us provide appropriate pricing
              </p>
            </div>

            <div className="flex gap-4">
              <Button type="submit" className="flex-1" disabled={isLoading}>
                {isLoading ? 'Submitting...' : 'Submit Request'}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={isLoading}
                onClick={() =>
                  setFormData({
                    email: '',
                    name: '',
                    company: '',
                    use_case: '',
                    requested_limits: '',
                    budget: '',
                  })
                }
              >
                Reset
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
