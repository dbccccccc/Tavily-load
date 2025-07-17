'use client'

import { useState, useEffect } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { apiClient } from '@/lib/api-client'
import { StrategyConfig } from '@/lib/types'
import { Settings, Save, RefreshCw, CheckCircle, AlertTriangle } from 'lucide-react'

export default function StrategySettings() {
  const [config, setConfig] = useState<StrategyConfig>({
    strategy: 'round_robin',
    auto_optimization: false,
    blacklist_threshold: 1,
    max_retries: 3,
  })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [lastSaved, setLastSaved] = useState<Date | null>(null)

  const loadConfig = async () => {
    try {
      const data = await apiClient.getStrategy()
      setConfig(data)
    } catch (error) {
      console.error('Failed to load strategy config:', error)
    } finally {
      setLoading(false)
    }
  }

  const saveConfig = async () => {
    setSaving(true)
    try {
      await apiClient.setStrategy(config)
      setLastSaved(new Date())
    } catch (error) {
      console.error('Failed to save strategy config:', error)
    } finally {
      setSaving(false)
    }
  }

  useEffect(() => {
    loadConfig()
  }, [])

  const strategyOptions = [
    {
      value: 'round_robin',
      label: 'Round Robin',
      description: 'Balanced usage across all available keys'
    },
    {
      value: 'plan_first',
      label: 'Plan First',
      description: 'Prefer plan credits over pay-as-you-go'
    },
    {
      value: 'cost_optimized',
      label: 'Cost Optimized',
      description: 'Minimize costs with intelligent routing'
    },
    {
      value: 'balanced',
      label: 'Balanced',
      description: 'Balance between plan and paygo usage'
    }
  ]

  const getStrategyBadge = (strategy: string) => {
    switch (strategy) {
      case 'plan_first':
        return <Badge className="bg-green-100 text-green-800">Cost Effective</Badge>
      case 'cost_optimized':
        return <Badge className="bg-blue-100 text-blue-800">Optimized</Badge>
      case 'balanced':
        return <Badge className="bg-purple-100 text-purple-800">Balanced</Badge>
      default:
        return <Badge className="bg-gray-100 text-gray-800">Standard</Badge>
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold">Strategy Configuration</h2>
          <p className="text-muted-foreground">Configure load balancing and key selection strategies</p>
        </div>
        <div className="flex items-center space-x-2">
          <Button onClick={loadConfig} variant="outline">
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
          <Button onClick={saveConfig} disabled={saving}>
            <Save className={`h-4 w-4 mr-2 ${saving ? 'animate-spin' : ''}`} />
            {saving ? 'Saving...' : 'Save Changes'}
          </Button>
        </div>
      </div>

      {lastSaved && (
        <div className="flex items-center space-x-2 text-sm text-green-600">
          <CheckCircle className="h-4 w-4" />
          <span>Configuration saved at {lastSaved.toLocaleTimeString()}</span>
        </div>
      )}

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center space-x-2">
              <Settings className="h-5 w-5" />
              <span>Selection Strategy</span>
            </CardTitle>
            <CardDescription>Choose how API keys are selected for requests</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Current Strategy</label>
              <div className="flex items-center space-x-2">
                <Select
                  value={config.strategy}
                  onValueChange={(value) => setConfig({ ...config, strategy: value as any })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {strategyOptions.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        <div>
                          <div className="font-medium">{option.label}</div>
                          <div className="text-sm text-muted-foreground">{option.description}</div>
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {getStrategyBadge(config.strategy)}
              </div>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Auto Optimization</label>
              <Select
                value={config.auto_optimization ? 'enabled' : 'disabled'}
                onValueChange={(value) => setConfig({ ...config, auto_optimization: value === 'enabled' })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="enabled">Enabled</SelectItem>
                  <SelectItem value="disabled">Disabled</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-sm text-muted-foreground">
                Automatically optimize strategy based on usage patterns
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center space-x-2">
              <AlertTriangle className="h-5 w-5" />
              <span>Error Handling</span>
            </CardTitle>
            <CardDescription>Configure error handling and retry behavior</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Blacklist Threshold</label>
              <Select
                value={config.blacklist_threshold.toString()}
                onValueChange={(value) => setConfig({ ...config, blacklist_threshold: parseInt(value) })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">1 error</SelectItem>
                  <SelectItem value="2">2 errors</SelectItem>
                  <SelectItem value="3">3 errors</SelectItem>
                  <SelectItem value="5">5 errors</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-sm text-muted-foreground">
                Number of errors before blacklisting a key
              </p>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Max Retries</label>
              <Select
                value={config.max_retries.toString()}
                onValueChange={(value) => setConfig({ ...config, max_retries: parseInt(value) })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">1 retry</SelectItem>
                  <SelectItem value="2">2 retries</SelectItem>
                  <SelectItem value="3">3 retries</SelectItem>
                  <SelectItem value="5">5 retries</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-sm text-muted-foreground">
                Maximum retry attempts with different keys
              </p>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Strategy Recommendations</CardTitle>
          <CardDescription>Recommended settings based on your usage patterns</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <div className="flex items-start space-x-3">
              <CheckCircle className="h-5 w-5 text-green-600 mt-0.5" />
              <div>
                <p className="font-medium">Cost Optimization</p>
                <p className="text-sm text-muted-foreground">
                  Use "Plan First" strategy to minimize costs by prioritizing plan credits
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <CheckCircle className="h-5 w-5 text-green-600 mt-0.5" />
              <div>
                <p className="font-medium">High Availability</p>
                <p className="text-sm text-muted-foreground">
                  Enable auto-optimization and set blacklist threshold to 2 for better reliability
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <CheckCircle className="h-5 w-5 text-green-600 mt-0.5" />
              <div>
                <p className="font-medium">Balanced Performance</p>
                <p className="text-sm text-muted-foreground">
                  Use "Balanced" strategy with 3 max retries for optimal performance
                </p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}