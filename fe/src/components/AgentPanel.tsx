import { useState, useEffect } from 'react'
import type { AgentInfo } from './AgentList'
import { IframeTab } from './IframeTab'
import type { TabInfo } from './IframeTab'
import { Plus, Monitor, AlertCircle, Loader2, Code, Maximize2, Minimize2, X } from 'lucide-react'

interface AgentPanelProps {
  agent: AgentInfo
}

export function AgentPanel({ agent }: AgentPanelProps) {
  const [tabs, setTabs] = useState<TabInfo[]>([])
  const [activeTabId, setActiveTabId] = useState<string | null>(null)
  const [newPort, setNewPort] = useState('')
  const [showAddInput, setShowAddInput] = useState(false)
  const [iframeError, setIframeError] = useState<string | null>(null)
  const [codeServerLoading, setCodeServerLoading] = useState(false)
  const [codeServerError, setCodeServerError] = useState<string | null>(null)
  const [isFullscreen, setIsFullscreen] = useState(false)

  useEffect(() => {
    if (!isFullscreen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsFullscreen(false)
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isFullscreen])

  const addTab = (port: number) => {
    const existing = tabs.find((t) => t.port === port)
    if (existing) {
      setActiveTabId(existing.id)
      return
    }
    const id = `tab-${Date.now()}`
    const url = `/proxy/agents/${agent.agentName}/ports/${port}/`
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

  const handleSetupCodeServer = async () => {
    setCodeServerLoading(true)
    setCodeServerError(null)
    try {
      const resp = await fetch(`/api/agents/${agent.agentName}/setup-code-server`, { method: 'POST' })
      const data = await resp.json()
      if (data.success) {
        addTab(data.port)
      } else {
        setCodeServerError(data.message)
      }
    } catch (err) {
      setCodeServerError(err instanceof Error ? err.message : '设置失败')
    } finally {
      setCodeServerLoading(false)
    }
  }

  const activeTab = tabs.find((t) => t.id === activeTabId)

  if (!agent.online) {
    return (
      <div
        id="agent-offline-state"
        className="flex flex-1 items-center justify-center bg-[radial-gradient(circle_at_top,_rgba(248,113,113,0.14),_transparent_28%),linear-gradient(180deg,_rgba(255,255,255,0.78),_rgba(248,250,252,0.94))] p-8"
      >
        <div className="w-full max-w-lg rounded-[32px] border border-rose-200/70 bg-white/82 px-10 py-12 text-center text-slate-500 shadow-[0_28px_80px_rgba(15,23,42,0.10)] backdrop-blur-sm">
          <div className="mx-auto mb-5 flex h-18 w-18 items-center justify-center rounded-[24px] border border-rose-100 bg-white text-rose-500 shadow-[0_16px_30px_rgba(248,113,113,0.12)]">
            <AlertCircle className="h-8 w-8" />
          </div>
          <p id="agent-offline-title" className="text-2xl font-semibold tracking-tight text-slate-950">Agent Offline</p>
          <p id="agent-offline-description" className="mt-3 text-sm leading-7 text-slate-500">
            This machine is currently unavailable. Wait for it to reconnect before opening forwarded ports or remote workspaces.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div
      id={`agent-panel-${agent.agentName}`}
      className="flex flex-1 flex-col overflow-hidden bg-[linear-gradient(180deg,_rgba(255,255,255,0.50),_rgba(248,250,252,0.88))]"
    >
      <div className="flex items-center justify-between border-b border-slate-200/75 px-6 py-5">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-[0.28em] text-slate-400">Active Agent</p>
          <div className="mt-2 flex items-center gap-3">
            <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-primary-100 bg-primary-50 text-primary-700 shadow-[0_12px_30px_rgba(13,148,136,0.10)]">
              <Monitor className="h-5 w-5 text-primary-600" />
            </div>
            <div className="min-w-0">
              <p className="truncate text-2xl font-semibold tracking-tight text-slate-950">{agent.agentName}</p>
              <p className="text-sm text-slate-500">Hub port {agent.hubPort} · forwarding-ready workspace</p>
            </div>
          </div>
        </div>
        <div className="hidden rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-semibold uppercase tracking-[0.24em] text-emerald-700 md:block">
          Online
        </div>
      </div>

      {/* Tab bar + actions */}
      <div
        id="agent-toolbar"
        className="flex items-stretch gap-2 border-b border-slate-200/80 bg-[linear-gradient(180deg,_rgba(248,250,252,0.96),_rgba(241,245,249,0.90))] px-4 py-3 shadow-[inset_0_-1px_0_rgba(255,255,255,0.9)]"
      >
        <IframeTab
          tabs={tabs}
          activeTabId={activeTabId}
          onTabClick={setActiveTabId}
          onTabClose={closeTab}
          onAddTab={() => setShowAddInput(true)}
        />
        <div id="agent-toolbar-actions" className="flex items-stretch gap-2 ml-auto shrink-0">
          <button
            id="setup-code-server-button"
            onClick={handleSetupCodeServer}
            disabled={codeServerLoading}
            className="flex items-center gap-2 rounded-2xl border border-primary-200/80 bg-white px-4 text-sm font-medium text-primary-700 shadow-[0_12px_28px_rgba(13,148,136,0.10)] transition-all hover:-translate-y-0.5 hover:bg-primary-50 disabled:translate-y-0 disabled:opacity-60 disabled:cursor-not-allowed"
            title="启动 Code Server"
          >
            {codeServerLoading ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Code className="w-4 h-4" />
            )}
            <span>Code Server</span>
          </button>
          {activeTab && (
            <button
              id="open-fullscreen-button"
              onClick={() => setIsFullscreen(true)}
              className="rounded-2xl border border-slate-200 bg-white px-4 text-slate-500 shadow-[0_12px_28px_rgba(15,23,42,0.06)] transition-all hover:-translate-y-0.5 hover:bg-slate-50 hover:text-slate-700"
              title="全屏 (Esc 退出)"
            >
              <Maximize2 className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      {/* Code server error banner */}
      {codeServerError && (
        <div
          id="code-server-error-banner"
          className="mx-4 mt-4 flex items-start gap-3 rounded-2xl border border-rose-200 bg-rose-50/90 px-4 py-3 text-sm text-rose-700 shadow-sm"
        >
          <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-white text-rose-500">
            <AlertCircle className="h-4 w-4" />
          </div>
          <span id="code-server-error-message" className="flex-1 whitespace-pre-wrap leading-6">{codeServerError}</span>
          <button id="code-server-error-close" onClick={() => setCodeServerError(null)} className="shrink-0 rounded-lg p-1 text-rose-400 transition-colors hover:bg-white hover:text-rose-600">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Add port input */}
      {showAddInput && (
        <div
          id="add-port-panel"
          className="mx-4 mt-4 rounded-[28px] border border-slate-200/80 bg-white/88 p-4 shadow-[0_18px_48px_rgba(15,23,42,0.08)] backdrop-blur-sm"
        >
          <div className="flex items-center gap-2">
            <input
              id="add-port-input"
              type="number"
              value={newPort}
              onChange={(e) => setNewPort(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddPort()}
              placeholder="Port number (e.g., 3000)"
              className="flex-1 rounded-2xl border border-slate-200 bg-slate-50/80 px-4 py-3 text-slate-700 outline-none transition-all placeholder:text-slate-400 focus:border-primary-300 focus:bg-white focus:ring-4 focus:ring-primary-100"
              autoFocus
            />
            <button
              id="add-port-confirm"
              onClick={handleAddPort}
              className="flex items-center gap-2 rounded-2xl bg-primary-600 px-4 py-3 text-white transition-all hover:bg-primary-700"
            >
              <Plus className="w-4 h-4" />
              Add
            </button>
            <button
              id="add-port-cancel"
              onClick={() => {
                setShowAddInput(false)
                setNewPort('')
              }}
              className="rounded-2xl bg-slate-100 px-4 py-3 text-slate-700 transition-colors hover:bg-slate-200"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Iframe container */}
      <div id="iframe-container" className="relative flex-1 bg-[linear-gradient(180deg,_rgba(248,250,252,0.3),_rgba(255,255,255,0.1))]">
        {activeTab ? (
          <>
            {iframeError && (
              <div
                id="iframe-error-overlay"
                className="absolute inset-0 z-10 flex items-center justify-center bg-[radial-gradient(circle_at_top,_rgba(248,113,113,0.14),_transparent_26%),linear-gradient(180deg,_rgba(255,255,255,0.82),_rgba(248,250,252,0.95))] p-6"
              >
                <div className="w-full max-w-md rounded-[30px] border border-rose-200/75 bg-white/86 px-8 py-10 text-center text-slate-500 shadow-[0_28px_80px_rgba(15,23,42,0.12)] backdrop-blur-sm">
                  <div className="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-[22px] border border-rose-100 bg-white text-rose-500 shadow-[0_16px_32px_rgba(248,113,113,0.12)]">
                    <AlertCircle className="h-7 w-7" />
                  </div>
                  <p id="iframe-error-title" className="text-2xl font-semibold tracking-tight text-slate-950">Connection Failed</p>
                  <p id="iframe-error-message" className="mt-3 text-sm leading-7 text-slate-500">{iframeError}</p>
                  <button
                    id="iframe-error-retry"
                    onClick={() => setIframeError(null)}
                    className="mt-6 rounded-2xl bg-primary-600 px-5 py-3 text-sm font-medium text-white transition-colors hover:bg-primary-700"
                  >
                    Retry
                  </button>
                </div>
              </div>
            )}
            <iframe
              id={`agent-iframe-${activeTab.port}`}
              key={activeTab.id}
              src={activeTab.url}
              className="h-full w-full border-0 bg-white"
              title={`Port ${activeTab.port}`}
              onError={() => setIframeError('Failed to load the page')}
            />
          </>
        ) : (
          <div
            id="iframe-empty-state"
            className="absolute inset-0 flex items-center justify-center bg-[radial-gradient(circle_at_top,_rgba(255,255,255,0.9),_rgba(241,245,249,0.92)_45%,_rgba(226,232,240,0.95))] p-6"
          >
            <div className="w-full max-w-lg rounded-[32px] border border-white/70 bg-white/82 px-10 py-12 text-center text-slate-500 shadow-[0_28px_80px_rgba(15,23,42,0.10)] backdrop-blur-sm">
              <div className="mx-auto mb-5 flex h-18 w-18 items-center justify-center rounded-[24px] border border-primary-100 bg-primary-50 text-primary-700 shadow-[0_16px_34px_rgba(13,148,136,0.10)]">
                <Monitor className="h-7 w-7 text-primary-600" />
              </div>
              <p className="mb-3 text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">Preview Surface</p>
              <p id="iframe-empty-title" className="text-3xl font-semibold tracking-tight text-slate-900">No Port Selected</p>
              <p id="iframe-empty-description" className="mt-3 text-sm leading-7 text-slate-500">
                Add a forwarded port from the toolbar to open a live preview in this workspace.
              </p>
            </div>
          </div>
        )}
      </div>

      {/* Fullscreen overlay */}
      {isFullscreen && activeTab && (
        <div id="fullscreen-overlay" className="fixed inset-0 z-50 flex flex-col bg-[linear-gradient(180deg,_rgba(250,252,251,0.98),_rgba(244,247,245,0.96))] backdrop-blur-sm">
          <div id="fullscreen-toolbar" className="flex items-center justify-between border-b border-slate-200 px-5 py-4 shrink-0 text-slate-900">
            <span id="fullscreen-tab-label" className="text-sm font-semibold tracking-[0.24em] text-slate-500">:{activeTab.port}</span>
            <button
              id="close-fullscreen-button"
              onClick={() => setIsFullscreen(false)}
              className="rounded-xl border border-slate-200 bg-white p-2 text-slate-500 transition-all hover:bg-slate-50 hover:text-slate-800"
              title="退出全屏 (Esc)"
            >
              <Minimize2 className="w-4 h-4" />
            </button>
          </div>
          <iframe
            id={`fullscreen-iframe-${activeTab.port}`}
            src={activeTab.url}
            className="flex-1 w-full border-0 bg-white"
            title={`Port ${activeTab.port} (fullscreen)`}
          />
        </div>
      )}
    </div>
  )
}
