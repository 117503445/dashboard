import { useState } from 'react'
import type { AgentInfo } from './AgentList'
import { IframeTab } from './IframeTab'
import type { TabInfo } from './IframeTab'
import { Plus, Monitor, AlertCircle } from 'lucide-react'

interface AgentPanelProps {
  agent: AgentInfo
}

export function AgentPanel({ agent }: AgentPanelProps) {
  const [tabs, setTabs] = useState<TabInfo[]>([])
  const [activeTabId, setActiveTabId] = useState<string | null>(null)
  const [newPort, setNewPort] = useState('')
  const [showAddInput, setShowAddInput] = useState(false)
  const [iframeError, setIframeError] = useState<string | null>(null)

  const addTab = (port: number) => {
    const id = `tab-${Date.now()}`
    const url = `/proxy/agents/${agent.agentName}/ports/${port}`
    const newTab: TabInfo = { id, port, url }
    setTabs((prev) => [...prev, newTab])
    setActiveTabId(id)
    setShowAddInput(false)
    setNewPort('')
    setIframeError(null)
  }

  const handleAddPort = () => {
    const port = parseInt(newPort, 10)
    if (port > 0 && port < 65536) {
      addTab(port)
    }
  }

  const closeTab = (tabId: string) => {
    setTabs((prev) => prev.filter((t) => t.id !== tabId))
    if (activeTabId === tabId) {
      const remaining = tabs.filter((t) => t.id !== tabId)
      setActiveTabId(remaining.length > 0 ? remaining[remaining.length - 1].id : null)
    }
  }

  const activeTab = tabs.find((t) => t.id === activeTabId)

  if (!agent.online) {
    return (
      <div className="flex-1 flex items-center justify-center bg-slate-50">
        <div className="text-center text-slate-500">
          <AlertCircle className="w-12 h-12 mx-auto mb-4 text-red-400" />
          <p className="text-lg font-medium">Agent Offline</p>
          <p className="text-sm mt-1">This agent is currently not available</p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex-1 flex flex-col bg-white overflow-hidden">
      {/* Tab bar */}
      <IframeTab
        tabs={tabs}
        activeTabId={activeTabId}
        onTabClick={setActiveTabId}
        onTabClose={closeTab}
        onAddTab={() => setShowAddInput(true)}
      />

      {/* Add port input */}
      {showAddInput && (
        <div className="p-4 bg-slate-50 border-b border-slate-200">
          <div className="flex items-center gap-2">
            <input
              type="number"
              value={newPort}
              onChange={(e) => setNewPort(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddPort()}
              placeholder="Port number (e.g., 3000)"
              className="flex-1 px-3 py-2 border border-slate-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              autoFocus
            />
            <button
              onClick={handleAddPort}
              className="px-4 py-2 bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors flex items-center gap-2"
            >
              <Plus className="w-4 h-4" />
              Add
            </button>
            <button
              onClick={() => {
                setShowAddInput(false)
                setNewPort('')
              }}
              className="px-4 py-2 bg-slate-200 text-slate-700 rounded-lg hover:bg-slate-300 transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Iframe container */}
      <div className="flex-1 relative">
        {activeTab ? (
          <>
            {iframeError && (
              <div className="absolute inset-0 flex items-center justify-center bg-slate-50 z-10">
                <div className="text-center text-slate-500">
                  <AlertCircle className="w-12 h-12 mx-auto mb-4 text-red-400" />
                  <p className="text-lg font-medium">Connection Failed</p>
                  <p className="text-sm mt-1">{iframeError}</p>
                  <button
                    onClick={() => setIframeError(null)}
                    className="mt-4 px-4 py-2 bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors"
                  >
                    Retry
                  </button>
                </div>
              </div>
            )}
            <iframe
              key={activeTab.id}
              src={activeTab.url}
              className="w-full h-full border-0"
              title={`Port ${activeTab.port}`}
              onError={() => setIframeError('Failed to load the page')}
            />
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center bg-slate-50">
            <div className="text-center text-slate-500">
              <Monitor className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p className="text-lg font-medium">No Port Selected</p>
              <p className="text-sm mt-1">Click + to add a port to view</p>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}