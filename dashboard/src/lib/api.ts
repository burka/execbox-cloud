const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

export interface ExecutionRequest {
  language: string;
  code: string;
  timeout?: number;
  memory_limit?: number;
}

export interface ExecutionResponse {
  session_id: string;
  stdout: string;
  stderr: string;
  exit_code: number;
  execution_time: number;
}

export interface ApiKeyInfo {
  api_key: string;
  created_at: string;
  quota: {
    total: number;
    used: number;
    remaining: number;
  };
}

export class ApiClient {
  private apiKey: string;

  constructor(apiKey: string) {
    this.apiKey = apiKey;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const response = await fetch(`${API_BASE_URL}${endpoint}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.apiKey}`,
        ...options.headers,
      },
    });

    if (!response.ok) {
      const error = await response.text();
      throw new Error(`API Error: ${response.status} - ${error}`);
    }

    return response.json();
  }

  async execute(request: ExecutionRequest): Promise<ExecutionResponse> {
    return this.request<ExecutionResponse>('/v1/execute', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  }

  async getApiKeyInfo(): Promise<ApiKeyInfo> {
    return this.request<ApiKeyInfo>('/v1/account/info');
  }

  async getUsageStats(): Promise<{
    sessions_today: number;
    quota_used: number;
    quota_total: number;
  }> {
    return this.request('/v1/account/usage');
  }
}

// Stub functions for when no API key is available
export const stubApiClient = {
  getApiKeyInfo: async (): Promise<ApiKeyInfo> => {
    return {
      api_key: 'sk_test_1234567890abcdef',
      created_at: new Date().toISOString(),
      quota: {
        total: 5000,
        used: 4000,
        remaining: 1000,
      },
    };
  },

  getUsageStats: async () => {
    return {
      sessions_today: 42,
      quota_used: 4000,
      quota_total: 5000,
    };
  },

  execute: async (request: ExecutionRequest): Promise<ExecutionResponse> => {
    console.log('Stub execution request:', request);
    return {
      session_id: `stub_${Date.now()}`,
      stdout: 'Hello, World!\n',
      stderr: '',
      exit_code: 0,
      execution_time: 0.123,
    };
  },
};
