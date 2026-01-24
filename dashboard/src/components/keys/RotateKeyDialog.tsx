import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { type APIKeyResponse } from '@/lib/api';

interface RotateKeyDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  apiKey: APIKeyResponse | null;
  onConfirm: () => Promise<void>;
}

export function RotateKeyDialog({
  open,
  onOpenChange,
  apiKey,
  onConfirm,
}: RotateKeyDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Rotate API Key?</AlertDialogTitle>
          <AlertDialogDescription>
            This will generate a new key for "{apiKey?.name}". The old key will be
            immediately invalidated. You will see the new key only once.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm}>Rotate Key</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
