import { useState, useCallback } from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Key } from 'lucide-react';
import { useAPIKeys } from '@/hooks/useAPIKeys';
import { KeyCard } from '@/components/keys/KeyCard';
import { CreateKeyDialog } from '@/components/keys/CreateKeyDialog';
import { EditKeyDialog } from '@/components/keys/EditKeyDialog';
import { DeleteKeyDialog } from '@/components/keys/DeleteKeyDialog';
import { RotateKeyDialog } from '@/components/keys/RotateKeyDialog';
import { NewKeyDialog } from '@/components/keys/NewKeyDialog';
import { type APIKeyResponse } from '@/lib/api';
import { useToast } from '@/hooks/use-toast';

export function KeysPage() {
  const { keys, isLoading, createKey, updateKey, deleteKey, rotateKey } = useAPIKeys();
  const { toast } = useToast();

  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [isRotateDialogOpen, setIsRotateDialogOpen] = useState(false);
  const [isNewKeyDialogOpen, setIsNewKeyDialogOpen] = useState(false);
  const [selectedKey, setSelectedKey] = useState<APIKeyResponse | null>(null);
  const [newKeyValue, setNewKeyValue] = useState<string>('');

  const handleCreateKey = useCallback(
    async (name: string, description?: string) => {
      try {
        const key = await createKey(name, description);
        setNewKeyValue(key);
        setIsNewKeyDialogOpen(true);
      } catch {
        // Error is already handled by the hook
      }
    },
    [createKey]
  );

  const handleUpdateKey = useCallback(
    async (name: string, description?: string) => {
      if (!selectedKey) return;
      try {
        await updateKey(selectedKey.id, name, description);
        setSelectedKey(null);
      } catch {
        // Error is already handled by the hook
      }
    },
    [selectedKey, updateKey]
  );

  const handleDeleteKey = useCallback(async () => {
    if (!selectedKey) return;
    try {
      await deleteKey(selectedKey.id);
      setSelectedKey(null);
    } catch {
      // Error is already handled by the hook
    }
  }, [selectedKey, deleteKey]);

  const handleRotateKey = useCallback(async () => {
    if (!selectedKey) return;
    try {
      const key = await rotateKey(selectedKey.id);
      setNewKeyValue(key);
      setIsNewKeyDialogOpen(true);
      setSelectedKey(null);
    } catch {
      // Error is already handled by the hook
    }
  }, [selectedKey, rotateKey]);

  const handleToggleStatus = useCallback(
    async (key: APIKeyResponse) => {
      try {
        await updateKey(key.id, key.name || 'API Key', key.description);
        toast({
          title: 'Status Updated',
          description: 'API key status has been updated',
        });
      } catch {
        toast({
          title: 'Error',
          description: 'Failed to update status',
          variant: 'destructive',
        });
      }
    },
    [updateKey, toast]
  );

  const openEditDialog = (key: APIKeyResponse) => {
    setSelectedKey(key);
    setIsEditDialogOpen(true);
  };

  const openDeleteDialog = (key: APIKeyResponse) => {
    setSelectedKey(key);
    setIsDeleteDialogOpen(true);
  };

  const openRotateDialog = (key: APIKeyResponse) => {
    setSelectedKey(key);
    setIsRotateDialogOpen(true);
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">API Keys</h1>
          <p className="text-muted-foreground">
            Manage your API keys and authentication
          </p>
        </div>
        <Button onClick={() => setIsCreateDialogOpen(true)}>
          <Key className="mr-2 h-4 w-4" />
          Create Key
        </Button>
      </div>

      <div className="grid gap-4">
        {keys.length === 0 ? (
          <Card>
            <CardContent className="pt-6">
              <div className="text-center py-8 text-muted-foreground">
                No API keys found. Create one to get started.
              </div>
            </CardContent>
          </Card>
        ) : (
          keys.map((key) => (
            <KeyCard
              key={key.id}
              apiKey={key}
              onEdit={openEditDialog}
              onDelete={openDeleteDialog}
              onRotate={openRotateDialog}
              onToggleStatus={handleToggleStatus}
            />
          ))
        )}
      </div>

      <CreateKeyDialog
        open={isCreateDialogOpen}
        onOpenChange={setIsCreateDialogOpen}
        onSubmit={handleCreateKey}
      />

      <EditKeyDialog
        open={isEditDialogOpen}
        onOpenChange={setIsEditDialogOpen}
        apiKey={selectedKey}
        onSubmit={handleUpdateKey}
      />

      <DeleteKeyDialog
        open={isDeleteDialogOpen}
        onOpenChange={setIsDeleteDialogOpen}
        apiKey={selectedKey}
        onConfirm={handleDeleteKey}
      />

      <RotateKeyDialog
        open={isRotateDialogOpen}
        onOpenChange={setIsRotateDialogOpen}
        apiKey={selectedKey}
        onConfirm={handleRotateKey}
      />

      <NewKeyDialog
        open={isNewKeyDialogOpen}
        onOpenChange={(open) => {
          setIsNewKeyDialogOpen(open);
          if (!open) {
            setNewKeyValue('');
          }
        }}
        keyValue={newKeyValue}
      />
    </div>
  );
}
