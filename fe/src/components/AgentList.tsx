import { useEffect, useState } from 'react'
import { rpcClient } from '@/lib/rpc'
import { Server, CheckCircle, XCircle, Loader2 } from 'lucide-react'

export interface AgentInfo {
  agentName: string
  hubPort: number
  online: boolean
}

interface AgentListProps {
  selectedAgent: AgentInfo | null
  onSelectAgent: (agent: AgentInfo) => void
}

export function AgentList({ selectedAgent, onSelectAgent }: AgentListProps) {
  const [agents, setAgents] = useState<AgentInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchAgents = async () => {
      try {
        const response = await rpcClient.listAgents({})
        setAgents(response.agents.map(a => ({
          agentName: a.agentName,
          hubPort: a.hubPort,
          online: a.online,
        })))
        setError(null)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch agents')
      } finally {
        setLoading(false)
      }
    }

    fetchAgents()
    const interval = setInterval(fetchAgents, 5000)
    return () => clearInterval(interval)
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-32">
        <Loader2 className="w-6 h-6 animate-spin text-primary-500" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-4 text-red-600 bg-red-50 rounded-lg">
        <p className="text-sm">{error}</p>
      </div>
    )
  }

  if (agents.length === 0) {
    return (
      <div className="p-4 text-slate-500 text-center">
        <Server className="w-8 h-8 mx-auto mb-2 opacity-50" />
        <p className="text-sm">No agents found</p>
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {agents.map((agent) => (
        <button
          key={agent.agentName}
          onClick={() => onSelectAgent(agent)}
          className={`w-full p-3 rounded-lg border transition-all text-left ${
            selectedAgent?.agentName === agent.agentName
              ? 'border-primary-500 bg-primary-50'
              : 'border-slate-200 hover:border-slate-300 bg-white'
          }`}
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Server className={`w-4 h-4 ${
                selectedAgent?.agentName === agent.agentName
                  ? 'text-primary-500'
                  : 'text-slate-400'
              }`} />
              <span className={`font-medium ${
                selectedAgent?.agentName === agent.agentName
                  ? 'text-primary-700'
                  : 'text-slate-700'
              }`}>
                {agent.agentName}
              </span>
            </div>
            <div className="flex items-center gap-2">
              {agent.online ? (
                <CheckCircle className="w-4 h-4 text-emerald-500" />
              ) : (
                <XCircle className="w-4 h-4 text-red-400" />
              )}
            </div>
          </div>
          <div className="mt-1 text-xs text-slate-500">
            Port: {agent.hubPort}
          </div>
        </button>
      ))}
    </div>
  )
}