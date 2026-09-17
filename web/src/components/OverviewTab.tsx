import React, { useEffect, useState } from 'react'
import { BotStatus, CatalogsData, DobbyApiClient } from '../api/client'
import { Bot, CheckCircle, Clock, Database, RefreshCw, ShieldAlert } from 'lucide-react'

interface Props {
  client: DobbyApiClient
}

export const OverviewTab: React.FC<Props> = ({ client }) => {
  const [bots, setBots] = useState<BotStatus[]>([])
  const [schedulesCount, setSchedulesCount] = useState<number>(0)
  const [catalogs, setCatalogs] = useState<CatalogsData>({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadData = async () => {
    setLoading(true)
    setError(null)
    try {
      const [botRes, schedRes, catRes] = await Promise.all([
        client.getBotsStatus(),
        client.getSchedules(),
        client.getCatalogs(),
      ])
      setBots(botRes.bots || [])
      setSchedulesCount(schedRes.count || 0)
      setCatalogs(catRes || {})
    } catch (err: any) {
      setError(err.message || '데이터 로드에 실패했습니다.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-[#111827]">봇 시스템 개요</h2>
          <p className="text-xs text-[#6B7280]">카카오톡 애플리케이션 및 연결된 AI 봇의 실시간 가동 상태입니다.</p>
        </div>
        <button
          onClick={loadData}
          disabled={loading}
          className="btn"
        >
          <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${loading ? 'animate-spin' : ''}`} />
          새로고침
        </button>
      </div>

      {error && (
        <div className="p-4 text-xs font-medium text-[#C5221F] bg-[#FCE8E6] border border-[#FAD2CF] rounded-2xl flex items-center gap-3">
          <ShieldAlert className="w-4 h-4 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Metric Cards matching dashboard */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="p-5 bg-white border border-[#EAECF0] rounded-[18px] shadow-[0_1px_3px_rgba(0,0,0,0.04)] flex items-center gap-4">
          <div className="p-3 bg-[#F1F3F5] text-[#111827] rounded-xl">
            <Bot className="w-5 h-5" />
          </div>
          <div>
            <div className="text-[11px] font-bold text-[#6B7280] uppercase tracking-wider">등록된 카카오 봇</div>
            <div className="text-2xl font-extrabold text-[#111827] mt-0.5 tracking-tight">{bots.length}개 앱</div>
          </div>
        </div>

        <div className="p-5 bg-white border border-[#EAECF0] rounded-[18px] shadow-[0_1px_3px_rgba(0,0,0,0.04)] flex items-center gap-4">
          <div className="p-3 bg-[#F1F3F5] text-[#111827] rounded-xl">
            <Clock className="w-5 h-5" />
          </div>
          <div>
            <div className="text-[11px] font-bold text-[#6B7280] uppercase tracking-wider">활성 알람/스케줄</div>
            <div className="text-2xl font-extrabold text-[#111827] mt-0.5 tracking-tight">{schedulesCount}건</div>
          </div>
        </div>

        <div className="p-5 bg-white border border-[#EAECF0] rounded-[18px] shadow-[0_1px_3px_rgba(0,0,0,0.04)] flex items-center gap-4">
          <div className="p-3 bg-[#F1F3F5] text-[#111827] rounded-xl">
            <Database className="w-5 h-5" />
          </div>
          <div>
            <div className="text-[11px] font-bold text-[#6B7280] uppercase tracking-wider">데이터 카탈로그</div>
            <div className="text-2xl font-extrabold text-[#111827] mt-0.5 tracking-tight">{catalogs.academyRoutesCount || 0}개 노선</div>
          </div>
        </div>
      </div>

      {/* Bot App Status Table */}
      <div className="bg-white border border-[#EAECF0] rounded-[18px] shadow-[0_1px_3px_rgba(0,0,0,0.04)] overflow-hidden">
        <div className="px-5 py-4 border-b border-[#EAECF0] flex items-center justify-between">
          <div className="font-bold text-sm text-[#111827]">카카오 애플리케이션 (봇) 토큰 상태</div>
          <span className="text-[11px] px-2.5 py-0.5 bg-[#F1F3F5] text-[#4B5563] border border-[#EAECF0] rounded-md font-bold">
            Multi-Bot Registry
          </span>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-[#F8F9FA] text-[#6B7280] text-[11px] font-bold uppercase tracking-wider border-b border-[#EAECF0]">
              <tr>
                <th className="px-5 py-3.5">봇 이름</th>
                <th className="px-5 py-3.5">기본(Default)</th>
                <th className="px-5 py-3.5">토큰 상태</th>
                <th className="px-5 py-3.5">만료 잔여 시간</th>
                <th className="px-5 py-3.5">토큰 스코프</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#F0F2F5]">
              {bots.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-5 py-8 text-center text-[#9CA3AF]">
                    등록된 봇이 없습니다.
                  </td>
                </tr>
              ) : (
                bots.map((b) => (
                  <tr key={b.name} className="hover:bg-[#F8F9FA] transition-colors">
                    <td className="px-5 py-4 font-bold text-[#111827] flex items-center gap-2">
                      <span className="w-2 h-2 rounded-full bg-[#111827]"></span>
                      {b.name}
                    </td>
                    <td className="px-5 py-4">
                      {b.is_default ? (
                        <span className="px-2 py-0.5 text-[11px] font-bold bg-[#111827] text-white rounded-md">
                          기본 봇
                        </span>
                      ) : (
                        <span className="text-[#9CA3AF] text-xs">-</span>
                      )}
                    </td>
                    <td className="px-5 py-4">
                      {!b.has_token ? (
                        <span className="inline-flex items-center gap-1.5 text-[11px] font-bold px-2 py-0.5 bg-[#FCE8E6] text-[#C5221F] border border-[#FAD2CF] rounded-md">
                          <ShieldAlert className="w-3.5 h-3.5" /> 미인증
                        </span>
                      ) : b.is_expired ? (
                        <span className="inline-flex items-center gap-1.5 text-[11px] font-bold px-2 py-0.5 bg-[#FEF3C7] text-[#92400E] border border-[#FDE68A] rounded-md">
                          <Clock className="w-3.5 h-3.5" /> 만료됨 (자동갱신 예정)
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1.5 text-[11px] font-bold px-2 py-0.5 bg-[#E6F4EA] text-[#137333] border border-[#CEEAD6] rounded-md">
                          <CheckCircle className="w-3.5 h-3.5" /> 인증 유효
                        </span>
                      )}
                    </td>
                    <td className="px-5 py-4 text-[#4B5563] font-medium">
                      {b.has_token && !b.is_expired
                        ? `${Math.floor(b.expires_in / 3600)}시간 ${Math.floor((b.expires_in % 3600) / 60)}분`
                        : '-'}
                    </td>
                    <td className="px-5 py-4 text-[#6B7280] font-mono">
                      {b.scope || 'talk_message'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
