import React, { useMemo, useState } from 'react'
import { DobbyApiClient } from './api/client'
import { OverviewTab } from './components/OverviewTab'
import { MessageTab } from './components/MessageTab'
import { SchedulesTab } from './components/SchedulesTab'
import { CatalogsTab } from './components/CatalogsTab'
import { SimulatorTab } from './components/SimulatorTab'
import { Bell, Database, LayoutDashboard, MessageSquare, Sparkles } from 'lucide-react'

export interface AppProps {
  embedded?: boolean
  apiBase?: string
}

export const App: React.FC<AppProps> = ({ embedded = false, apiBase = '/dobby/api' }) => {
  const [activeTab, setActiveTab] = useState<'overview' | 'message' | 'schedules' | 'catalogs' | 'simulator'>('overview')
  const client = useMemo(() => new DobbyApiClient(apiBase), [apiBase])

  const navItems = [
    { id: 'overview', label: '대시보드', icon: LayoutDashboard },
    { id: 'message', label: '메시지 발송', icon: MessageSquare },
    { id: 'schedules', label: '알림 & 스케줄', icon: Bell },
    { id: 'catalogs', label: '데이터 카탈로그', icon: Database },
    { id: 'simulator', label: 'AI 시뮬레이터', icon: Sparkles },
  ] as const

  return (
    <div className={`dobby-shell w-full ${embedded ? 'p-0 bg-transparent' : 'min-h-screen bg-[#EFF0F3] p-6'}`}>
      <div className="max-w-6xl mx-auto space-y-6">
        {/* Top Header Card in Dashboard Style */}
        <div className="bg-white border border-[#EAECF0] rounded-[20px] p-5 shadow-[0_1px_3px_rgba(0,0,0,0.04)] flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
          <div className="flex items-center gap-3.5">
            <div className="w-10 h-10 rounded-xl bg-[#111827] text-white font-extrabold flex items-center justify-center text-sm tracking-tight shadow-xs flex-shrink-0">
              0x
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-[1.15rem] font-extrabold text-[#111827] tracking-tight">도비 (카카오봇)</h1>
                <span className="text-[11px] font-bold px-2 py-0.5 bg-[#FEF3C7] text-[#92400E] border border-[#FDE68A] rounded-md">
                  @0xc0de1ab
                </span>
                <span className="text-[10px] font-bold px-1.5 py-0.5 bg-[#F1F3F5] text-[#4B5563] border border-[#EAECF0] rounded-md uppercase tracking-wider">
                  v2.0
                </span>
              </div>
              <p className="text-xs text-[#6B7280] mt-0.5 font-medium">
                카카오 멀티봇 오퍼레이션 & AI 에이전트 콘솔
              </p>
            </div>
          </div>

          {/* Sub-navigation Tabs matching Dashboard button style */}
          <div className="flex items-center gap-2 overflow-x-auto pb-1 sm:pb-0">
            {navItems.map((item) => {
              const Icon = item.icon
              const isActive = activeTab === item.id
              return (
                <button
                  key={item.id}
                  onClick={() => setActiveTab(item.id)}
                  className={`btn ${isActive ? 'btn-primary' : ''}`}
                >
                  <Icon className="w-4 h-4 mr-1.5" />
                  {item.label}
                </button>
              )
            })}
          </div>
        </div>

        {/* Tab Content View */}
        <div className="transition-all duration-150">
          {activeTab === 'overview' && <OverviewTab client={client} />}
          {activeTab === 'message' && <MessageTab client={client} />}
          {activeTab === 'schedules' && <SchedulesTab client={client} />}
          {activeTab === 'catalogs' && <CatalogsTab client={client} />}
          {activeTab === 'simulator' && <SimulatorTab client={client} />}
        </div>
      </div>
    </div>
  )
}

export default App
