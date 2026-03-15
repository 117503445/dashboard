import { useEffect, useState } from 'react'
import { rpcClient } from '@/lib/rpc'
import { Activity, Server, GitBranch, CheckCircle, XCircle, Loader2 } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

interface HealthzData {
  version: string
  status: 'loading' | 'success' | 'error'
  error?: string
}

function App() {
  const [healthz, setHealthz] = useState<HealthzData>({
    version: '',
    status: 'loading',
  })

  useEffect(() => {
    const fetchHealthz = async () => {
      try {
        const response = await rpcClient.healthz({})
        if (response.code === 0n && response.payload.case === 'healthz') {
          setHealthz({
            version: response.payload.value.version || 'unknown',
            status: 'success',
          })
        } else {
          setHealthz({
            version: '',
            status: 'error',
            error: response.message || 'Unknown error',
          })
        }
      } catch (err) {
        setHealthz({
          version: '',
          status: 'error',
          error: err instanceof Error ? err.message : 'Connection failed',
        })
      }
    }

    fetchHealthz()
    const interval = setInterval(fetchHealthz, 5000)
    return () => clearInterval(interval)
  }, [])

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="w-full max-w-md space-y-6">
        {/* Header */}
        <div className="text-center space-y-2">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-500 shadow-lg shadow-primary-500/25 mb-4">
            <Activity className="w-8 h-8 text-white" />
          </div>
          <h1 className="text-3xl font-bold bg-gradient-to-r from-primary-600 to-accent-600 bg-clip-text text-transparent">
            Service Status
          </h1>
          <p className="text-slate-500">Real-time backend health monitoring</p>
        </div>

        {/* Status Card */}
        <Card className="overflow-hidden">
          <div className="h-1 bg-gradient-to-r from-primary-500 via-accent-500 to-primary-500 animate-pulse" />
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-lg">
              <Server className="w-5 h-5 text-primary-500" />
              Backend Service
            </CardTitle>
            <CardDescription>
              Connect RPC Template Service
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Status Indicator */}
            <div className="flex items-center justify-between p-3 rounded-lg bg-slate-50 border border-slate-100">
              <span className="text-sm text-slate-600">Status</span>
              <div className="flex items-center gap-2">
                {healthz.status === 'loading' && (
                  <>
                    <Loader2 className="w-4 h-4 text-amber-500 animate-spin" />
                    <span className="text-sm font-medium text-amber-600">Checking...</span>
                  </>
                )}
                {healthz.status === 'success' && (
                  <>
                    <CheckCircle className="w-4 h-4 text-emerald-500" />
                    <span className="text-sm font-medium text-emerald-600">Healthy</span>
                  </>
                )}
                {healthz.status === 'error' && (
                  <>
                    <XCircle className="w-4 h-4 text-red-500" />
                    <span className="text-sm font-medium text-red-600">Error</span>
                  </>
                )}
              </div>
            </div>

            {/* Version Info */}
            {healthz.status === 'success' && (
              <div className="flex items-center justify-between p-3 rounded-lg bg-slate-50 border border-slate-100">
                <div className="flex items-center gap-2 text-sm text-slate-600">
                  <GitBranch className="w-4 h-4" />
                  Version
                </div>
                <code className="px-2 py-0.5 rounded bg-primary-100 text-primary-700 text-sm font-mono">
                  {healthz.version}
                </code>
              </div>
            )}

            {/* Error Message */}
            {healthz.status === 'error' && (
              <div className="p-3 rounded-lg bg-red-50 border border-red-100">
                <p className="text-sm text-red-600 font-mono break-all">
                  {healthz.error}
                </p>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Footer */}
        <p className="text-center text-xs text-slate-400">
          Powered by Connect RPC + React
        </p>
      </div>
    </div>
  )
}

export default App