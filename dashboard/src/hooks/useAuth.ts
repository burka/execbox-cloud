import { useState, useEffect } from 'react';
import { getStoredApiKey, setStoredApiKey, clearApiKey, isAuthenticated } from '@/lib/auth';
import { ApiClient } from '@/lib/api';

export function useAuth() {
  const [apiKey, setApiKey] = useState<string | null>(getStoredApiKey());
  const [isLoading, setIsLoading] = useState(true);
  const [isValid, setIsValid] = useState(false);

  useEffect(() => {
    const validateApiKey = async () => {
      const storedKey = getStoredApiKey();
      if (!storedKey) {
        setIsLoading(false);
        setIsValid(false);
        return;
      }

      try {
        const client = new ApiClient(storedKey);
        await client.getAccount();
        setIsValid(true);
        setApiKey(storedKey);
      } catch (error) {
        console.error('API key validation failed:', error);
        setIsValid(false);
        clearApiKey();
        setApiKey(null);
      } finally {
        setIsLoading(false);
      }
    };

    validateApiKey();
  }, []);

  const login = (key: string) => {
    setStoredApiKey(key);
    setApiKey(key);
    setIsValid(true);
  };

  const logout = () => {
    clearApiKey();
    setApiKey(null);
    setIsValid(false);
  };

  return {
    apiKey,
    isAuthenticated: isAuthenticated(),
    isValid,
    isLoading,
    login,
    logout,
  };
}
