import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Copy, Edit, RefreshCw, Trash2 } from 'lucide-react';
import { type APIKeyResponse } from '@/lib/api';
import { useToast } from '@/hooks/use-toast';

interface KeyCardProps {
  apiKey: APIKeyResponse;
  onEdit: (key: APIKeyResponse) => void;
  onDelete: (key: APIKeyResponse) => void;
  onRotate: (key: APIKeyResponse) => void;
  onToggleStatus?: (key: APIKeyResponse) => void;
}

function formatDate(dateString?: string): string {
  if (!dateString) return 'Never';
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffMinutes = Math.floor(diffMs / (1000 * 60));

  if (diffMinutes < 60) {
    return diffMinutes <= 1 ? 'Just now' : `${diffMinutes} minutes ago`;
  } else if (diffHours < 24) {
    return diffHours === 1 ? '1 hour ago' : `${diffHours} hours ago`;
  } else if (diffDays < 30) {
    return diffDays === 1 ? '1 day ago' : `${diffDays} days ago`;
  } else {
    return date.toLocaleDateString();
  }
}

export function KeyCard({
  apiKey,
  onEdit,
  onDelete,
  onRotate,
  onToggleStatus,
}: KeyCardProps) {
  const { toast } = useToast();

  const handleCopyKey = () => {
    navigator.clipboard.writeText(apiKey.key_preview);
    toast({
      title: 'Copied',
      description: 'API key copied to clipboard',
    });
  };

  return (
    <Card className={!apiKey.is_active ? 'opacity-60' : ''}>
      <CardHeader>
        <div className="flex justify-between items-start">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <CardTitle className="text-lg">{apiKey.name}</CardTitle>
              <Badge variant={apiKey.is_active ? 'secondary' : 'outline'}>
                {apiKey.is_active ? 'Active' : 'Inactive'}
              </Badge>
            </div>
            {apiKey.description && (
              <CardDescription>{apiKey.description}</CardDescription>
            )}
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => onEdit(apiKey)}
            >
              <Edit className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => onRotate(apiKey)}
            >
              <RefreshCw className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => onDelete(apiKey)}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          <div className="flex gap-2">
            <Input
              type="password"
              value={apiKey.key_preview}
              readOnly
              className="font-mono text-sm"
            />
            <Button
              variant="outline"
              size="sm"
              onClick={handleCopyKey}
            >
              <Copy className="h-4 w-4" />
            </Button>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">ID:</span>
              <span className="ml-2 font-mono text-xs">
                {apiKey.id.slice(0, 8)}
              </span>
            </div>
            <div>
              <span className="text-muted-foreground">Created:</span>
              <span className="ml-2">{formatDate(apiKey.created_at)}</span>
            </div>
            <div>
              <span className="text-muted-foreground">Last Used:</span>
              <span className="ml-2">{formatDate(apiKey.last_used_at)}</span>
            </div>
            <div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onToggleStatus?.(apiKey)}
                className="h-auto p-0"
              >
                {apiKey.is_active ? 'Deactivate' : 'Activate'}
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
