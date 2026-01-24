import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useToast } from '@/hooks/use-toast';
import {
  ApiClient,
  type APIKeyResponse,
  type CreateAPIKeyRequest,
  type UpdateAPIKeyRequest,
} from '@/lib/api';
import { getStoredApiKey, clearApiKey } from '@/lib/auth';

export interface UseAPIKeysReturn {
  keys: APIKeyResponse[];
  isLoading: boolean;
  error: string | null;
  createKey: (name: string, description?: string) => Promise<string>;
  updateKey: (id: string, name: string, description?: string) => Promise<void>;
  deleteKey: (id: string) => Promise<void>;
  rotateKey: (id: string) => Promise<string>;
  refreshKeys: () => Promise<void>;
}

export function useAPIKeys(): UseAPIKeysReturn {
  const [keys, setKeys] = useState<APIKeyResponse[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { toast } = useToast();
  const navigate = useNavigate();

  const refreshKeys = useCallback(async () => {
    const storedKey = getStoredApiKey();
    if (!storedKey) {
      navigate('/');
      return;
    }

    try {
      const client = new ApiClient(storedKey);
      const apiKeys = await client.listAPIKeys();
      setKeys(apiKeys);
      setError(null);
    } catch {
      setError('Failed to fetch API keys');
      toast({
        title: 'Error',
        description: 'Failed to fetch API keys. Please try logging in again.',
        variant: 'destructive',
      });
      clearApiKey();
      navigate('/');
    } finally {
      setIsLoading(false);
    }
  }, [navigate, toast]);

  useEffect(() => {
    refreshKeys();
  }, [refreshKeys]);

  const createKey = useCallback(
    async (name: string, description?: string): Promise<string> => {
      if (!name.trim()) {
        const msg = 'Please provide a name for the API key';
        toast({
          title: 'Validation Error',
          description: msg,
          variant: 'destructive',
        });
        throw new Error(msg);
      }

      const storedKey = getStoredApiKey();
      if (!storedKey) throw new Error('No authentication token found');

      try {
        const client = new ApiClient(storedKey);
        const request: CreateAPIKeyRequest = {
          name,
          description: description || undefined,
        };
        const response = await client.createAPIKey(request);

        await refreshKeys();

        toast({
          title: 'API Key Created',
          description: 'Your new API key has been created successfully',
        });

        return response.key;
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'Failed to create API key';
        toast({
          title: 'Error',
          description: msg,
          variant: 'destructive',
        });
        throw err;
      }
    },
    [toast, refreshKeys]
  );

  const updateKey = useCallback(
    async (id: string, name: string, description?: string): Promise<void> => {
      if (!name.trim()) {
        throw new Error('Name is required');
      }

      const storedKey = getStoredApiKey();
      if (!storedKey) throw new Error('No authentication token found');

      try {
        const client = new ApiClient(storedKey);
        const request: UpdateAPIKeyRequest = {
          name,
          description: description || undefined,
        };
        await client.updateAPIKey(id, request);

        await refreshKeys();

        toast({
          title: 'API Key Updated',
          description: 'API key has been updated successfully',
        });
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'Failed to update API key';
        toast({
          title: 'Error',
          description: msg,
          variant: 'destructive',
        });
        throw err;
      }
    },
    [toast, refreshKeys]
  );

  const deleteKey = useCallback(
    async (id: string): Promise<void> => {
      const storedKey = getStoredApiKey();
      if (!storedKey) throw new Error('No authentication token found');

      try {
        const client = new ApiClient(storedKey);
        await client.deleteAPIKey(id);

        await refreshKeys();

        toast({
          title: 'API Key Deleted',
          description: 'API key has been deleted successfully',
        });
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'Failed to delete API key';
        toast({
          title: 'Error',
          description: msg,
          variant: 'destructive',
        });
        throw err;
      }
    },
    [toast, refreshKeys]
  );

  const rotateKey = useCallback(
    async (id: string): Promise<string> => {
      const storedKey = getStoredApiKey();
      if (!storedKey) throw new Error('No authentication token found');

      try {
        const client = new ApiClient(storedKey);
        const response = await client.rotateAPIKey(id);

        await refreshKeys();

        toast({
          title: 'API Key Rotated',
          description: 'Your API key has been rotated successfully',
        });

        return response.key;
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'Failed to rotate API key';
        toast({
          title: 'Error',
          description: msg,
          variant: 'destructive',
        });
        throw err;
      }
    },
    [toast, refreshKeys]
  );

  return {
    keys,
    isLoading,
    error,
    createKey,
    updateKey,
    deleteKey,
    rotateKey,
    refreshKeys,
  };
}
