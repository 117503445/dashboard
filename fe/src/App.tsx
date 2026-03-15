import { useState } from 'react'
import { Activity, Server } from 'lucide-react'
import { AgentList } from '@/components/AgentList'
import type { AgentInfo } from '@/components/AgentList'
import { AgentPanel } from '@/components/AgentPanel'

function App() {
  const [selectedAgent, setSelectedAgent] = useState<AgentInfo | null>(null)

  return (
    <div className="min-h-screen flex flex-col">
      {/* Header */}
      <header className="bg-white border-b border-slate-200 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="flex items-center justify-center w-10 h-10 rounded-xl bg-gradient-to-br from-primary-500 to-accent-500 shadow-lg shadow-primary-500/25">
            <Activity className="w-5 h-5 text-white" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-slate-900">Dashboard</h1>
            <p className="text-sm text-slate-500">Linux Agent Manager</p>
          </div>
        </div>
      </header>

      {/* Main content */}
      <div className="flex-1 flex overflow-hidden">
        {/* Sidebar - Agent List */}
        <aside className="w-72 bg-white border-r border-slate-200 flex flex-col">
          <div className="p-4 border-b border-slate-200">
            <div className="flex items-center gap-2 text-slate-700">
              <Server className="w-5 h-5" />
              <h2 className="font-semibold">Agents</h2>
            </div>
          </div>
          <div className="flex-1 overflow-y-auto p-4">
            <AgentList
              selectedAgent={selectedAgent}
              onSelectAgent={setSelectedAgent}
            />
          </div>
        </aside>

        {/* Main panel */}
        <main className="flex-1 flex flex-col overflow-hidden">
          {selectedAgent ? (
            <AgentPanel agent={selectedAgent} />
          ) : (
            <div className="flex-1 flex items-center justify-center bg-slate-50">
              <div className="text-center text-slate-500">
                <Server className="w-16 h-16 mx-auto mb-4 opacity-50" />
                <p className="text-lg font-medium">Select an Agent</p>
                <p className="text-sm mt-1">Choose an agent from the list to manage</p>
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  )
}

export default App