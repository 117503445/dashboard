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
    <div
      id="iframe-tab-bar"
      className="flex h-full min-w-0 flex-1 self-stretch items-stretch gap-2 overflow-x-auto rounded-[24px] border border-white/80 bg-white/86 px-2.5 py-2 shadow-[0_16px_40px_rgba(15,23,42,0.08),inset_0_1px_0_rgba(255,255,255,0.9)] backdrop-blur-sm"
    >
      {tabs.map((tab) => (
        <div
          id={`iframe-tab-${tab.port}`}
          key={tab.id}
          className={`group flex h-full min-w-fit items-center gap-2 px-4 rounded-[18px] cursor-pointer transition-all duration-200 ${
            activeTabId === tab.id
              ? 'border border-primary-100 bg-white text-slate-900 shadow-[0_12px_30px_rgba(13,148,136,0.10)]'
              : 'bg-transparent text-slate-500 hover:bg-slate-100 hover:text-slate-700'
          }`}
          onClick={() => onTabClick(tab.id)}
        >
          <span id={`iframe-tab-label-${tab.port}`} className="text-sm font-semibold tracking-tight">:{tab.port}</span>
          <button
            id={`iframe-tab-close-${tab.port}`}
            onClick={(e) => {
              e.stopPropagation()
              onTabClose(tab.id)
            }}
            className={`rounded-md p-1 transition-all ${
              activeTabId === tab.id
                ? 'text-slate-400 hover:bg-slate-100 hover:text-rose-500'
                : 'text-slate-400 opacity-0 group-hover:opacity-100 hover:bg-white hover:text-rose-500'
            }`}
          >
            <X className="w-3 h-3" />
          </button>
          <a
            id={`iframe-tab-open-${tab.port}`}
            href={tab.url}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            className={`rounded-md p-1 transition-all ${
              activeTabId === tab.id
                ? 'text-slate-400 hover:bg-slate-100 hover:text-primary-600'
                : 'text-slate-400 opacity-0 group-hover:opacity-100 hover:bg-white hover:text-primary-600'
            }`}
          >
            <ExternalLink className="w-3 h-3" />
          </a>
        </div>
      ))}
      <button
        id="iframe-tab-add"
        onClick={onAddTab}
        className="flex h-full min-w-12 items-center justify-center rounded-[18px] border border-dashed border-slate-200 bg-white/70 px-3 text-slate-500 transition-all hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700"
        title="Add new port"
      >
        <span className="text-lg leading-none">+</span>
      </button>
    </div>
  )
}
