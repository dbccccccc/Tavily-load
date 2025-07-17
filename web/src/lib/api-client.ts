import { ApiKey, ServerStats, UsageAnalytics, StrategyConfig } from './types'

const API_BASE_URL = process.env.NODE_ENV === 'production' ? '' : 'http://localhost:3000'

class ApiClient {
  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const response = await fetch(`${API_BASE_URL}${endpoint}`, {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      ...options,
    })

    if (!response.ok) {
      throw new Error(`API request failed: ${response.statusText}`)
    }

    return response.json()
  }

  async getHealth(): Promise<{ status: string; uptime: number; version: string }> {
    return this.request('/health')
  }

  async getStats(): Promise<ServerStats> {
    return this.request('/stats')
  }

  async getUsageAnalytics(): Promise<UsageAnalytics> {
    return this.request('/usage-analytics')
  }

  async updateUsage(): Promise<{ message: string }> {
    return this.request('/update-usage', { method: 'POST' })
  }

  async getStrategy(): Promise<StrategyConfig> {
    return this.request('/strategy')
  }

  async setStrategy(strategy: StrategyConfig): Promise<{ message: string }> {
    return this.request('/strategy', {
      method: 'POST',
      body: JSON.stringify(strategy),
    })
  }

  async getBlacklist(): Promise<{ blacklisted_keys: string[]; reasons: Record<string, string> }> {
    return this.request('/blacklist')
  }

  async resetKeys(): Promise<{ message: string }> {
    return this.request('/reset-keys')
  }

  // Key management endpoints
  async getApiKeys(): Promise<{ keys: ApiKey[]; count: number }> {
    return this.request('/api/keys')
  }

  async addApiKey(keyData: { key: string; name: string; description?: string }): Promise<{ status: string; message: string; key: ApiKey }> {
    return this.request('/api/keys', {
      method: 'POST',
      body: JSON.stringify(keyData),
    })
  }

  async deleteApiKey(id: string): Promise<{ status: string; message: string }> {
    return this.request(`/api/keys?id=${id}`, {
      method: 'DELETE',
    })
  }

  async bulkImportKeys(keysText: string, prefix?: string): Promise<{
    status: string;
    message: string;
    total_keys: number;
    imported_count: number;
    skipped_count: number;
    error_count: number;
    errors?: string[];
  }> {
    return this.request('/api/keys/bulk-import', {
      method: 'POST',
      body: JSON.stringify({ keys: keysText, prefix }),
    })
  }

  async uploadKeysFile(file: File, prefix?: string): Promise<{
    status: string;
    message: string;
    total_keys: number;
    imported_count: number;
    skipped_count: number;
    error_count: number;
    errors?: string[];
  }> {
    const formData = new FormData()
    formData.append('file', file)
    if (prefix) {
      formData.append('prefix', prefix)
    }

    const response = await fetch(`${API_BASE_URL}/api/keys/upload`, {
      method: 'POST',
      body: formData,
    })

    if (!response.ok) {
      throw new Error(`Upload failed: ${response.statusText}`)
    }

    return response.json()
  }
}

export const apiClient = new ApiClient()