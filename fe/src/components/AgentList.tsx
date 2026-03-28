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
      <div
        id="agents-loading"
        className="flex h-40 items-center justify-center rounded-[28px] border border-slate-200 bg-slate-50/90"
      >
        <div className="flex items-center gap-3 rounded-full border border-slate-200 bg-white px-4 py-2 text-sm text-slate-600">
          <Loader2 className="h-4 w-4 animate-spin text-primary-500" />
          <span>Syncing agents...</span>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div
        id="agents-error"
        className="rounded-[28px] border border-red-200 bg-red-50/90 p-4 text-red-700 shadow-inner shadow-red-100/70"
      >
        <p className="mb-1 text-xs font-semibold uppercase tracking-[0.24em] text-red-400">Unavailable</p>
        <p id="agents-error-message" className="text-sm leading-6">{error}</p>
      </div>
    )
  }

  if (agents.length === 0) {
    return (
      <div
        id="agents-empty"
        className="rounded-[28px] border border-slate-200 bg-slate-50/90 px-5 py-8 text-center text-slate-500"
      >
        <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-white ring-1 ring-slate-200">
          <Server className="h-6 w-6 text-slate-400" />
        </div>
        <p id="agents-empty-message" className="text-sm font-medium">No agents found</p>
      </div>
    )
  }

  return (
    <div id="agents-list" className="space-y-3">
      {agents.map((agent) => (
        <button
          id={`agent-item-${agent.agentName}`}
          key={agent.agentName}
          onClick={() => onSelectAgent(agent)}
          className={`group w-full rounded-[24px] border px-4 py-4 text-left transition-all duration-200 ${
            selectedAgent?.agentName === agent.agentName
              ? 'border-primary-200 bg-gradient-to-br from-primary-50 via-white to-accent-50/40 shadow-[0_18px_40px_rgba(13,148,136,0.12)]'
              : 'border-slate-200 bg-white/82 hover:border-slate-300 hover:bg-white'
          }`}
        >
          <div className="flex items-start justify-between gap-3">
            <div className="flex min-w-0 items-start gap-3">
              <div
                className={`mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl ring-1 ${
                  selectedAgent?.agentName === agent.agentName
                    ? 'bg-primary-50 text-primary-700 ring-primary-100'
                    : 'bg-slate-50 text-slate-500 ring-slate-200'
                }`}
              >
                <Server className={`h-4 w-4 ${
                selectedAgent?.agentName === agent.agentName
                  ? 'text-primary-700'
                  : 'text-slate-500'
              }`} />
              </div>
              <div className="min-w-0">
                <span
                  id={`agent-name-${agent.agentName}`}
                  className={`block truncate text-base font-semibold tracking-tight ${
                    selectedAgent?.agentName === agent.agentName
                      ? 'text-slate-950'
                      : 'text-slate-800'
                  }`}
                >
                  {agent.agentName}
                </span>
                <div
                  id={`agent-port-${agent.agentName}`}
                  className={`mt-1 text-xs font-medium uppercase tracking-[0.18em] ${
                    selectedAgent?.agentName === agent.agentName
                      ? 'text-slate-500'
                      : 'text-slate-400'
                  }`}
                >
                  Hub Port {agent.hubPort}
                </div>
              </div>
            </div>
            <div className="flex shrink-0 items-center">
              {agent.online ? (
                <div className="rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.22em] text-emerald-700">
                  Online
                </div>
              ) : (
                <div className="rounded-full border border-rose-200 bg-rose-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.22em] text-rose-700">
                  Offline
                </div>
              )}
            </div>
          </div>
          <div className="mt-4 flex items-center justify-between border-t border-slate-100 pt-3">
            <div className={`text-sm ${
              selectedAgent?.agentName === agent.agentName ? 'text-slate-600' : 'text-slate-500'
            }`}>
              {agent.online ? 'Ready for forwarding' : 'Waiting for heartbeat'}
            </div>
            {agent.online ? (
              <CheckCircle className="h-4 w-4 text-emerald-500" />
            ) : (
              <XCircle className="h-4 w-4 text-rose-500" />
            )}
          </div>
        </button>
      ))}
    </div>
  )
}
