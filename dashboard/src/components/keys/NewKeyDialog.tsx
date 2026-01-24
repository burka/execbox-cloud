import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Copy } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';

interface NewKeyDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  keyValue: string;
}

export function NewKeyDialog({
  open,
  onOpenChange,
  keyValue,
}: NewKeyDialogProps) {
  const { toast } = useToast();

  const handleCopyKey = () => {
    navigator.clipboard.writeText(keyValue);
    toast({
      title: 'Copied',
      description: 'API key copied to clipboard',
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Your New API Key</DialogTitle>
          <DialogDescription>
            Save this key securely. For security reasons, you will not be able to see it again.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>API Key</Label>
            <div className="flex gap-2">
              <Input
                value={keyValue}
                readOnly
                className="font-mono text-sm"
              />
              <Button
                variant="outline"
                onClick={handleCopyKey}
              >
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          </div>
          <div className="bg-yellow-50 dark:bg-yellow-950 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
            <p className="text-sm text-yellow-800 dark:text-yellow-200">
              Make sure to copy your API key now. You will not be able to see it again!
            </p>
          </div>
        </div>
        <DialogFooter>
          <Button onClick={() => {
            onOpenChange(false);
          }}>
            I've Saved My Key
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
