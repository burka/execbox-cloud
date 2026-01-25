/**
 * API client using openapi-fetch for type-safe API calls
 */
import createClient from 'openapi-fetch';
import type { paths, components } from '../generated/api';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';

// Export types for use in components
export type AccountResponse = components['schemas']['AccountResponse'];
export type UsageResponse = components['schemas']['UsageResponse'];
export type WaitlistRequest = components['schemas']['WaitlistRequest'];
export type WaitlistResponse = components['schemas']['WaitlistResponse'];
export type QuotaRequestRequest = components['schemas']['QuotaRequestRequest'];
export type QuotaRequestResponse = components['schemas']['QuotaRequestResponse'];
export type SessionResponse = components['schemas']['SessionResponse'];
export type CreateSessionRequest = components['schemas']['CreateSessionRequest'];
export type CreateSessionResponse = components['schemas']['CreateSessionResponse'];
export type EnhancedUsageResponse = components['schemas']['EnhancedUsageResponse'];
export type AccountLimitsResponse = components['schemas']['AccountLimitsResponse'];
export type DayUsage = components['schemas']['DayUsage'];
export type HourlyUsage = components['schemas']['HourlyUsage'];
export type APIKeyResponse = components['schemas']['APIKeyResponse'];
export type CreateAPIKeyRequest = components['schemas']['CreateAPIKeyRequest'];
export type CreateAPIKeyResponse = components['schemas']['CreateAPIKeyResponse'];
export type UpdateAPIKeyRequest = components['schemas']['UpdateAPIKeyRequest'];
export type ListAPIKeysResponse = components['schemas']['ListAPIKeysResponse'];
export type RotateAPIKeyResponse = components['schemas']['RotateAPIKeyResponse'];

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
 * Join the waitlist and get an API key (public endpoint, no auth required)
 */
export async function joinWaitlist(email: string, name?: string): Promise<WaitlistResponse> {
  const client = createApiClient();
  const { data, error } = await client.POST('/v1/waitlist', {
    body: { email, name },
  });

  if (error) {
    throw new Error(error.detail || 'Failed to join waitlist');
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

  /**
   * Get enhanced usage statistics with hourly and daily history
   */
  async getEnhancedUsage(days: number = 7): Promise<EnhancedUsageResponse> {
    const { data, error } = await this.client.GET('/v1/account/usage/enhanced', {
      params: { query: { days } },
    });

    if (error) {
      throw new Error(error.detail || 'Failed to get enhanced usage stats');
    }

    return data!;
  }

  /**
   * Get account limits
   */
  async getAccountLimits(): Promise<AccountLimitsResponse> {
    const { data, error } = await this.client.GET('/v1/account/limits');

    if (error) {
      throw new Error(error.detail || 'Failed to get account limits');
    }

    return data!;
  }

  /**
   * Export usage data as array of daily usage
   */
  async exportUsage(days: number = 30): Promise<DayUsage[]> {
    const { data, error } = await this.client.GET('/v1/account/usage/export', {
      params: { query: { days } },
    });

    if (error) {
      throw new Error(error.detail || 'Failed to export usage data');
    }

    return data!;
  }

  /**
   * List all API keys
   */
  async listAPIKeys(): Promise<APIKeyResponse[]> {
    const { data, error } = await this.client.GET('/v1/account/keys');
    if (error) {
      throw new Error(error.detail || 'Failed to list API keys');
    }
    return data?.keys || [];
  }

  /**
   * Create a new API key
   */
  async createAPIKey(request: CreateAPIKeyRequest): Promise<CreateAPIKeyResponse> {
    const { data, error } = await this.client.POST('/v1/account/keys', {
      body: request,
    });
    if (error) {
      throw new Error(error.detail || 'Failed to create API key');
    }
    return data!;
  }

  /**
   * Update an API key
   */
  async updateAPIKey(id: string, request: UpdateAPIKeyRequest): Promise<APIKeyResponse> {
    const { data, error } = await this.client.PUT('/v1/account/keys/{id}', {
      params: { path: { id } },
      body: request,
    });
    if (error) {
      throw new Error(error.detail || 'Failed to update API key');
    }
    return data!;
  }

  /**
   * Delete an API key
   */
  async deleteAPIKey(id: string): Promise<void> {
    const { error } = await this.client.DELETE('/v1/account/keys/{id}', {
      params: { path: { id } },
    });
    if (error) {
      throw new Error(error.detail || 'Failed to delete API key');
    }
  }

  /**
   * Rotate an API key
   */
  async rotateAPIKey(id: string): Promise<RotateAPIKeyResponse> {
    const { data, error } = await this.client.POST('/v1/account/keys/{id}/rotate', {
      params: { path: { id } },
    });
    if (error) {
      throw new Error(error.detail || 'Failed to rotate API key');
    }
    return data!;
  }
}
