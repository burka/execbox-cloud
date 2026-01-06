/**
 * API client using openapi-fetch for type-safe API calls
 */
import createClient from 'openapi-fetch';
import type { paths, components } from '../generated/api';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';

// Export types for use in components
export type AccountResponse = components['schemas']['AccountResponse'];
export type UsageResponse = components['schemas']['UsageResponse'];
export type CreateKeyRequest = components['schemas']['CreateKeyRequest'];
export type CreateKeyResponse = components['schemas']['CreateKeyResponse'];
export type QuotaRequestRequest = components['schemas']['QuotaRequestRequest'];
export type QuotaRequestResponse = components['schemas']['QuotaRequestResponse'];
export type SessionResponse = components['schemas']['SessionResponse'];
export type CreateSessionRequest = components['schemas']['CreateSessionRequest'];
export type CreateSessionResponse = components['schemas']['CreateSessionResponse'];

/**
 * Create a type-safe API client instance
 */
export function createApiClient(apiKey?: string) {
  return createClient<paths>({
    baseUrl: API_BASE_URL,
    headers: apiKey ? { Authorization: `Bearer ${apiKey}` } : {},
  });
}

/**
 * Create a new API key (public endpoint, no auth required)
 */
export async function createApiKey(email: string, name?: string): Promise<CreateKeyResponse> {
  const client = createApiClient();
  const { data, error } = await client.POST('/v1/keys', {
    body: { email, name },
  });

  if (error) {
    throw new Error(error.detail || 'Failed to create API key');
  }

  return data!;
}

/**
 * Submit a quota increase request (public endpoint)
 */
export async function submitQuotaRequest(request: QuotaRequestRequest): Promise<QuotaRequestResponse> {
  const client = createApiClient();
  const { data, error } = await client.POST('/v1/quota-requests', {
    body: request,
  });

  if (error) {
    throw new Error(error.detail || 'Failed to submit quota request');
  }

  return data!;
}

/**
 * API client class for authenticated requests
 */
export class ApiClient {
  private client: ReturnType<typeof createApiClient>;

  constructor(apiKey: string) {
    this.client = createApiClient(apiKey);
  }

  /**
   * Get account information
   */
  async getAccount(): Promise<AccountResponse> {
    const { data, error } = await this.client.GET('/v1/account');

    if (error) {
      throw new Error(error.detail || 'Failed to get account info');
    }

    return data!;
  }

  /**
   * Get usage statistics
   */
  async getUsage(): Promise<UsageResponse> {
    const { data, error } = await this.client.GET('/v1/account/usage');

    if (error) {
      throw new Error(error.detail || 'Failed to get usage stats');
    }

    return data!;
  }

  /**
   * List all sessions
   */
  async listSessions(): Promise<SessionResponse[]> {
    const { data, error } = await this.client.GET('/v1/sessions');

    if (error) {
      throw new Error(error.detail || 'Failed to list sessions');
    }

    return data?.sessions || [];
  }

  /**
   * Create a new session
   */
  async createSession(request: CreateSessionRequest): Promise<CreateSessionResponse> {
    const { data, error } = await this.client.POST('/v1/sessions', {
      body: request,
    });

    if (error) {
      throw new Error(error.detail || 'Failed to create session');
    }

    return data!;
  }

  /**
   * Get a specific session
   */
  async getSession(id: string): Promise<SessionResponse> {
    const { data, error } = await this.client.GET('/v1/sessions/{id}', {
      params: { path: { id } },
    });

    if (error) {
      throw new Error(error.detail || 'Failed to get session');
    }

    return data!;
  }

  /**
   * Stop a session
   */
  async stopSession(id: string): Promise<void> {
    const { error } = await this.client.POST('/v1/sessions/{id}/stop', {
      params: { path: { id } },
    });

    if (error) {
      throw new Error(error.detail || 'Failed to stop session');
    }
  }

  /**
   * Kill a session
   */
  async killSession(id: string): Promise<void> {
    const { error } = await this.client.DELETE('/v1/sessions/{id}', {
      params: { path: { id } },
    });

    if (error) {
      throw new Error(error.detail || 'Failed to kill session');
    }
  }
}
