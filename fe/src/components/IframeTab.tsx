import { X, ExternalLink } from 'lucide-react'

export interface TabInfo {
  id: string
  port: number
  url: string
}

interface IframeTabProps {
  tabs: TabInfo[]
  activeTabId: string | null
  onTabClick: (tabId: string) => void
  onTabClose: (tabId: string) => void
  onAddTab: () => void
}

export function IframeTab({ tabs, activeTabId, onTabClick, onTabClose, onAddTab }: IframeTabProps) {
  return (
    <div className="flex items-center gap-1 p-2 overflow-x-auto flex-1 min-w-0">
      {tabs.map((tab) => (
        <div
          key={tab.id}
          className={`group flex items-center gap-2 px-3 py-1.5 rounded-t-lg cursor-pointer transition-colors ${
            activeTabId === tab.id
              ? 'bg-white border-t border-l border-r border-slate-200 text-slate-900'
              : 'bg-slate-200 hover:bg-slate-300 text-slate-600'
          }`}
          onClick={() => onTabClick(tab.id)}
        >
          <span className="text-sm font-medium">:{tab.port}</span>
          <button
            onClick={(e) => {
              e.stopPropagation()
              onTabClose(tab.id)
            }}
            className="opacity-0 group-hover:opacity-100 hover:text-red-500 transition-opacity"
          >
            <X className="w-3 h-3" />
          </button>
          <a
            href={tab.url}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            className="opacity-0 group-hover:opacity-100 hover:text-primary-500 transition-opacity"
          >
            <ExternalLink className="w-3 h-3" />
          </a>
        </div>
      ))}
      <button
        onClick={onAddTab}
        className="flex items-center justify-center w-6 h-6 rounded hover:bg-slate-200 text-slate-500 hover:text-slate-700 transition-colors"
        title="Add new port"
      >
        <span className="text-lg leading-none">+</span>
      </button>
    </div>
  )
}