'use client'

import { useState, useEffect } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { apiClient } from '@/lib/api-client'
import { ApiKey } from '@/lib/types'
import { Key, Plus, Trash2, RefreshCw, CheckCircle, XCircle, Upload, FileText, AlertTriangle } from 'lucide-react'

export default function KeyManagement() {
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [addDialogOpen, setAddDialogOpen] = useState(false)
  
  // Add single key form
  const [newKey, setNewKey] = useState('')
  const [keyName, setKeyName] = useState('')
  const [keyDescription, setKeyDescription] = useState('')
  const [addingKey, setAddingKey] = useState(false)
  
  // Bulk import form
  const [bulkKeys, setBulkKeys] = useState('')
  const [bulkPrefix, setBulkPrefix] = useState('')
  const [importing, setImporting] = useState(false)
  
  // File upload form
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [filePrefix, setFilePrefix] = useState('')
  const [uploading, setUploading] = useState(false)
  
  // Results
  const [importResults, setImportResults] = useState<any>(null)
  const [error, setError] = useState<string | null>(null)

  const loadKeys = async () => {
    try {
      const data = await apiClient.getApiKeys()
      setKeys(data.keys)
    } catch (error) {
      console.error('Failed to load API keys:', error)
      setError('Failed to load API keys')
    } finally {
      setLoading(false)
    }
  }

  const refreshKeys = async () => {
    setRefreshing(true)
    await loadKeys()
    setRefreshing(false)
  }

  useEffect(() => {
    loadKeys()
  }, [])

  const addSingleKey = async () => {
    if (!newKey.trim() || !keyName.trim()) return
    
    setAddingKey(true)
    setError(null)
    
    try {
      await apiClient.addApiKey({
        key: newKey.trim(),
        name: keyName.trim(),
        description: keyDescription.trim() || undefined
      })
      
      // Reset form
      setNewKey('')
      setKeyName('')
      setKeyDescription('')
      setAddDialogOpen(false)
      
      // Reload keys
      await loadKeys()
    } catch (error: any) {
      setError(error.message || 'Failed to add key')
    } finally {
      setAddingKey(false)
    }
  }
  
  const handleBulkImport = async () => {
    if (!bulkKeys.trim()) return
    
    setImporting(true)
    setError(null)
    setImportResults(null)
    
    try {
      const results = await apiClient.bulkImportKeys(bulkKeys, bulkPrefix || undefined)
      setImportResults(results)
      setBulkKeys('')
      setBulkPrefix('')
      
      if (results.imported_count > 0) {
        await loadKeys()
      }
    } catch (error: any) {
      setError(error.message || 'Failed to import keys')
    } finally {
      setImporting(false)
    }
  }
  
  const handleFileUpload = async () => {
    if (!selectedFile) return
    
    setUploading(true)
    setError(null)
    setImportResults(null)
    
    try {
      const results = await apiClient.uploadKeysFile(selectedFile, filePrefix || undefined)
      setImportResults(results)
      setSelectedFile(null)
      setFilePrefix('')
      
      if (results.imported_count > 0) {
        await loadKeys()
      }
    } catch (error: any) {
      setError(error.message || 'Failed to upload file')
    } finally {
      setUploading(false)
    }
  }
  
  const deleteKey = async (keyId: number) => {
    if (!confirm('Are you sure you want to delete this key?')) return
    
    try {
      await apiClient.deleteApiKey(keyId.toString())
      await loadKeys()
    } catch (error: any) {
      setError(error.message || 'Failed to delete key')
    }
  }

  const getStatusIcon = (key: ApiKey) => {
    if (key.is_blacklisted) {
      return <XCircle className="h-4 w-4 text-red-600" />
    }
    if (key.is_active) {
      return <CheckCircle className="h-4 w-4 text-green-600" />
    }
    return <AlertTriangle className="h-4 w-4 text-yellow-600" />
  }

  const getStatusColor = (key: ApiKey) => {
    if (key.is_blacklisted) {
      return 'bg-red-100 text-red-800'
    }
    if (key.is_active) {
      return 'bg-green-100 text-green-800'
    }
    return 'bg-yellow-100 text-yellow-800'
  }
  
  const getStatusText = (key: ApiKey) => {
    if (key.is_blacklisted) {
      return 'Blacklisted'
    }
    if (key.is_active) {
      return 'Active'
    }
    return 'Inactive'
  }

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
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
          <h2 className="text-2xl font-bold">API Key Management</h2>
          <p className="text-muted-foreground">Manage your Tavily API keys and monitor their status</p>
        </div>
        <div className="flex items-center space-x-2">
          <Button onClick={refreshKeys} disabled={refreshing} variant="outline">
            <RefreshCw className={`h-4 w-4 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          
          <Dialog open={addDialogOpen} onOpenChange={setAddDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4 mr-2" />
                Add Key
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-2xl">
              <DialogHeader>
                <DialogTitle>Add API Keys</DialogTitle>
                <DialogDescription>
                  Add API keys individually, paste multiple keys, or upload a file
                </DialogDescription>
              </DialogHeader>
              
              <Tabs defaultValue="single" className="w-full">
                <TabsList className="grid w-full grid-cols-3">
                  <TabsTrigger value="single">Single Key</TabsTrigger>
                  <TabsTrigger value="bulk">Paste Keys</TabsTrigger>
                  <TabsTrigger value="upload">Upload File</TabsTrigger>
                </TabsList>
                
                <TabsContent value="single" className="space-y-4">
                  <div className="space-y-4">
                    <div>
                      <Label htmlFor="key">API Key *</Label>
                      <Input
                        id="key"
                        placeholder="tvly-..."
                        value={newKey}
                        onChange={(e) => setNewKey(e.target.value)}
                        className="font-mono"
                      />
                    </div>
                    <div>
                      <Label htmlFor="name">Name *</Label>
                      <Input
                        id="name"
                        placeholder="Production Key 1"
                        value={keyName}
                        onChange={(e) => setKeyName(e.target.value)}
                      />
                    </div>
                    <div>
                      <Label htmlFor="description">Description</Label>
                      <Input
                        id="description"
                        placeholder="Optional description"
                        value={keyDescription}
                        onChange={(e) => setKeyDescription(e.target.value)}
                      />
                    </div>
                    <Button 
                      onClick={addSingleKey} 
                      disabled={addingKey || !newKey.trim() || !keyName.trim()}
                      className="w-full"
                    >
                      {addingKey ? 'Adding...' : 'Add Key'}
                    </Button>
                  </div>
                </TabsContent>
                
                <TabsContent value="bulk" className="space-y-4">
                  <div className="space-y-4">
                    <div>
                      <Label htmlFor="bulk-keys">API Keys (one per line) *</Label>
                      <Textarea
                        id="bulk-keys"
                        placeholder="tvly-key1...
tvly-key2...
tvly-key3..."
                        value={bulkKeys}
                        onChange={(e) => setBulkKeys(e.target.value)}
                        className="font-mono min-h-[120px]"
                      />
                    </div>
                    <div>
                      <Label htmlFor="bulk-prefix">Name Prefix</Label>
                      <Input
                        id="bulk-prefix"
                        placeholder="Imported Key"
                        value={bulkPrefix}
                        onChange={(e) => setBulkPrefix(e.target.value)}
                      />
                    </div>
                    <Button 
                      onClick={handleBulkImport} 
                      disabled={importing || !bulkKeys.trim()}
                      className="w-full"
                    >
                      {importing ? 'Importing...' : 'Import Keys'}
                    </Button>
                  </div>
                </TabsContent>
                
                <TabsContent value="upload" className="space-y-4">
                  <div className="space-y-4">
                    <div>
                      <Label htmlFor="file">Upload .txt file *</Label>
                      <Input
                        id="file"
                        type="file"
                        accept=".txt"
                        onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
                      />
                    </div>
                    <div>
                      <Label htmlFor="file-prefix">Name Prefix</Label>
                      <Input
                        id="file-prefix"
                        placeholder="Uploaded Key"
                        value={filePrefix}
                        onChange={(e) => setFilePrefix(e.target.value)}
                      />
                    </div>
                    <Button 
                      onClick={handleFileUpload} 
                      disabled={uploading || !selectedFile}
                      className="w-full"
                    >
                      <Upload className="h-4 w-4 mr-2" />
                      {uploading ? 'Uploading...' : 'Upload Keys'}
                    </Button>
                  </div>
                </TabsContent>
              </Tabs>
              
              {error && (
                <div className="bg-red-50 border border-red-200 rounded-md p-3">
                  <p className="text-red-800 text-sm">{error}</p>
                </div>
              )}
              
              {importResults && (
                <div className="bg-blue-50 border border-blue-200 rounded-md p-3">
                  <h4 className="font-medium text-blue-900 mb-2">Import Results</h4>
                  <div className="text-sm text-blue-800 space-y-1">
                    <p>Total keys: {importResults.total_keys}</p>
                    <p>Imported: {importResults.imported_count}</p>
                    <p>Skipped: {importResults.skipped_count}</p>
                    <p>Errors: {importResults.error_count}</p>
                    {importResults.errors && importResults.errors.length > 0 && (
                      <div className="mt-2">
                        <p className="font-medium">Errors:</p>
                        <ul className="list-disc list-inside">
                          {importResults.errors.map((error: string, i: number) => (
                            <li key={i}>{error}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                </div>
              )}
            </DialogContent>
          </Dialog>
        </div>
      </div>

      <div className="grid gap-4">
        {keys.map((key) => (
          <Card key={key.id}>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-3">
                  <Key className="h-5 w-5 text-muted-foreground" />
                  <div>
                    <CardTitle className="text-lg">{key.name}</CardTitle>
                    <CardDescription className="font-mono text-sm">{key.key_preview}</CardDescription>
                    {key.description && (
                      <CardDescription className="text-sm mt-1">{key.description}</CardDescription>
                    )}
                  </div>
                </div>
                <div className="flex items-center space-x-2">
                  <Badge className={getStatusColor(key)}>
                    {getStatusIcon(key)}
                    <span className="ml-1">{getStatusText(key)}</span>
                  </Badge>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-sm text-muted-foreground">Created</p>
                  <p className="text-sm font-medium">{formatDate(key.created_at)}</p>
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Last Updated</p>
                  <p className="text-sm font-medium">{formatDate(key.updated_at)}</p>
                </div>
                {key.is_blacklisted && (
                  <>
                    <div>
                      <p className="text-sm text-muted-foreground">Blacklist Reason</p>
                      <p className="text-sm font-medium text-red-600">{key.blacklist_reason || 'No reason provided'}</p>
                    </div>
                    {key.blacklisted_until && (
                      <div>
                        <p className="text-sm text-muted-foreground">Blacklisted Until</p>
                        <p className="text-sm font-medium">{formatDate(key.blacklisted_until)}</p>
                      </div>
                    )}
                  </>
                )}
              </div>
              
              <div className="mt-4 flex items-center justify-end">
                <Button 
                  variant="destructive" 
                  size="sm"
                  onClick={() => deleteKey(key.id)}
                >
                  <Trash2 className="h-4 w-4 mr-1" />
                  Delete
                </Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {keys.length === 0 && !loading && (
        <Card>
          <CardContent className="text-center py-8">
            <Key className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
            <p className="text-muted-foreground mb-4">No API keys found. Add your first key to get started.</p>
            <Button onClick={() => setAddDialogOpen(true)}>
              <Plus className="h-4 w-4 mr-2" />
              Add Your First Key
            </Button>
          </CardContent>
        </Card>
      )}
    </div>
  )
}