export interface ApiKey {
  id: number
  name: string
  description?: string
  key_preview: string
  is_active: boolean
  is_blacklisted: boolean
  blacklisted_until?: string
  blacklist_reason?: string
  created_at: string
  updated_at: string
}

export interface ServerStats {
  total_requests: number
  successful_requests: number
  error_count: number
  active_keys: number
  blacklisted_keys: number
  avg_response_time: number
  uptime: number
  success_rate: number
}

export interface UsageAnalytics {
  daily_requests: Array<{ date: string; requests: number }>
  error_breakdown: Array<{ error_type: string; count: number }>
  key_performance: Array<{ key_id: string; success_rate: number; avg_response_time: number }>
  strategy_efficiency: {
    current_strategy: string
    cost_savings: number
    recommendations: string[]
  }
}

export interface StrategyConfig {
  strategy: 'round_robin' | 'plan_first' | 'cost_optimized' | 'balanced'
  auto_optimization: boolean
  blacklist_threshold: number
  max_retries: number
}