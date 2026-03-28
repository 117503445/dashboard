import { useState } from 'react'
import { Server } from 'lucide-react'
import { AgentList } from '@/components/AgentList'
import type { AgentInfo } from '@/components/AgentList'
import { AgentPanel } from '@/components/AgentPanel'

function App() {
  const [selectedAgent, setSelectedAgent] = useState<AgentInfo | null>(null)

  return (
    <div id="app-root" className="min-h-screen">
      {/* Main content */}
      <div
        id="app-layout"
        className="flex min-h-screen gap-4 overflow-hidden p-4 lg:p-5"
      >
        {/* Sidebar - Agent List */}
        <aside
          id="agents-sidebar"
          className="flex w-80 shrink-0 flex-col overflow-hidden rounded-[32px] border border-white/70 bg-white/72 text-slate-900 shadow-[0_24px_70px_rgba(15,23,42,0.12)] backdrop-blur-xl"
        >
          <div
            id="agents-sidebar-header"
            className="border-b border-slate-200/80 bg-[radial-gradient(circle_at_top_left,_rgba(22,176,147,0.14),_transparent_40%),linear-gradient(160deg,_rgba(255,255,255,0.92),_rgba(248,250,252,0.98))] px-5 py-5"
          >
            <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-primary-50 ring-1 ring-primary-100 backdrop-blur-sm">
              <Server className="h-5 w-5 text-primary-600" />
            </div>
            <div className="space-y-1">
              <p className="text-xs font-semibold uppercase tracking-[0.28em] text-slate-400">Control Surface</p>
              <h2 id="agents-sidebar-title" className="text-2xl font-semibold tracking-tight text-slate-950">Agents</h2>
              <p className="text-sm leading-6 text-slate-500">
                Browse active machines, open forwarded ports, and launch remote workspaces.
              </p>
            </div>
          </div>
          <div
            id="agents-list-container"
            className="flex-1 overflow-y-auto bg-[linear-gradient(180deg,_rgba(248,250,252,0.55),_transparent)] px-4 py-4"
          >
            <AgentList
              selectedAgent={selectedAgent}
              onSelectAgent={setSelectedAgent}
            />
          </div>
        </aside>

        {/* Main panel */}
        <main
          id="agent-main-panel"
          className="flex min-w-0 flex-1 flex-col overflow-hidden rounded-[36px] border border-white/60 bg-white/72 shadow-[0_32px_90px_rgba(15,23,42,0.10)] backdrop-blur-xl"
        >
          {selectedAgent ? (
            <AgentPanel agent={selectedAgent} />
          ) : (
            <div
              id="agent-empty-state"
              className="relative flex flex-1 items-center justify-center overflow-hidden bg-[radial-gradient(circle_at_top,_rgba(22,176,147,0.10),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(249,115,22,0.12),_transparent_28%),linear-gradient(180deg,_rgba(255,255,255,0.66),_rgba(248,250,252,0.86))] p-8"
            >
              <div className="absolute left-10 top-10 h-32 w-32 rounded-full bg-primary-200/35 blur-3xl" />
              <div className="absolute bottom-10 right-10 h-36 w-36 rounded-full bg-accent-200/35 blur-3xl" />
              <div className="relative w-full max-w-xl rounded-[32px] border border-white/75 bg-white/78 px-10 py-14 text-center shadow-[0_28px_80px_rgba(15,23,42,0.12)] backdrop-blur-sm">
              <div className="mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-[24px] border border-primary-100 bg-primary-50 text-primary-700 shadow-[0_20px_40px_rgba(13,148,136,0.12)]">
                <Server className="h-8 w-8 text-primary-600" />
              </div>
                <p className="mb-3 text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">Remote Workspace</p>
                <p id="agent-empty-title" className="text-3xl font-semibold tracking-tight text-slate-950">
                  Select an Agent
                </p>
                <p id="agent-empty-description" className="mx-auto mt-3 max-w-md text-sm leading-7 text-slate-500">
                  Pick a machine from the left rail to manage tunnels, inspect forwarded apps, and open a code-server session.
                </p>
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  )
}

export default App
