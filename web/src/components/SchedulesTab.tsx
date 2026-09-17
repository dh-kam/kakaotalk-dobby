import React, { useEffect, useState } from 'react'
import { DobbyApiClient, ScheduleJob } from '../api/client'
import { Bell, Calendar, Check, Clock, Plus, RefreshCw, Trash2, X } from 'lucide-react'

interface Props {
  client: DobbyApiClient
}

export const SchedulesTab: React.FC<Props> = ({ client }) => {
  const [jobs, setJobs] = useState<ScheduleJob[]>([])
  const [loading, setLoading] = useState(true)
  const [modalOpen, setModalOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // New schedule form state
  const [title, setTitle] = useState('')
  const [message, setMessage] = useState('')
  const [type, setType] = useState<'once' | 'recurring'>('once')
  const [executeAt, setExecuteAt] = useState('1h')
  const [cronExpr, setCronExpr] = useState('0 8 * * 1-5')
  const [bot, setBot] = useState('dobby')

  const loadSchedules = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await client.getSchedules()
      setJobs(res.jobs || [])
    } catch (err: any) {
      setError(err.message || '알림 목록을 불러오지 못했습니다.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadSchedules()
  }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setCreating(true)
    setError(null)
    try {
      await client.createSchedule({
        title,
        message,
        type,
        execute_at: type === 'once' ? executeAt : undefined,
        cron_expr: type === 'recurring' ? cronExpr : undefined,
        bot: bot || undefined,
      })
      setModalOpen(false)
      setTitle('')
      setMessage('')
      loadSchedules()
    } catch (err: any) {
      setError(err.message || '스케줄 등록에 실패했습니다.')
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('정말 이 스케줄 알림을 삭제하시겠습니까?')) return
    try {
      await client.deleteSchedule(id)
      setJobs((prev) => prev.filter((j) => j.id !== id))
    } catch (err: any) {
      alert('삭제 실패: ' + err.message)
    }
  }

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-[#111827]">알림 & 스케줄 관리</h2>
          <p className="text-xs text-[#6B7280]">예약된 알람 및 반복 크론 작업을 관리하고 새 알람을 등록합니다.</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={loadSchedules}
            disabled={loading}
            className="btn"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${loading ? 'animate-spin' : ''}`} />
            새로고침
          </button>
          <button
            onClick={() => setModalOpen(true)}
            className="btn btn-primary"
          >
            <Plus className="w-4 h-4 mr-1.5" />
            새 알람 등록
          </button>
        </div>
      </div>

      {error && (
        <div className="p-4 text-xs font-medium text-[#C5221F] bg-[#FCE8E6] border border-[#FAD2CF] rounded-2xl flex items-center gap-3">
          <span>{error}</span>
        </div>
      )}

      {/* Jobs Table */}
      <div className="bg-white border border-[#EAECF0] rounded-[18px] shadow-[0_1px_3px_rgba(0,0,0,0.04)] overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-[#F8F9FA] text-[#6B7280] text-[11px] font-bold uppercase tracking-wider border-b border-[#EAECF0]">
              <tr>
                <th className="px-5 py-3.5">알림 제목</th>
                <th className="px-5 py-3.5">메시지 내용</th>
                <th className="px-5 py-3.5">유형</th>
                <th className="px-5 py-3.5">실행 일시 / 주기</th>
                <th className="px-5 py-3.5">상태</th>
                <th className="px-5 py-3.5 text-right">작업</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#F0F2F5]">
              {jobs.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-5 py-12 text-center text-[#9CA3AF]">
                    등록된 스케줄 알림이 없습니다.
                  </td>
                </tr>
              ) : (
                jobs.map((j) => (
                  <tr key={j.id} className="hover:bg-[#F8F9FA] transition-colors">
                    <td className="px-5 py-4 font-bold text-[#111827] flex items-center gap-2">
                      <Bell className="w-3.5 h-3.5 text-[#111827]" />
                      {j.title}
                    </td>
                    <td className="px-5 py-4 text-[#4B5563] max-w-xs truncate font-medium">
                      {j.message}
                    </td>
                    <td className="px-5 py-4">
                      {j.type === 'once' ? (
                        <span className="px-2 py-0.5 text-[11px] font-bold bg-[#F1F3F5] text-[#4B5563] border border-[#EAECF0] rounded-md">
                          일회성 (Once)
                        </span>
                      ) : (
                        <span className="px-2 py-0.5 text-[11px] font-bold bg-[#FEF3C7] text-[#92400E] border border-[#FDE68A] rounded-md">
                          반복 크론 (Cron)
                        </span>
                      )}
                    </td>
                    <td className="px-5 py-4 font-mono text-[11px] text-[#4B5563]">
                      {j.type === 'once' ? j.execute_at || '-' : j.cron_expr || '-'}
                    </td>
                    <td className="px-5 py-4">
                      <span className="inline-flex items-center gap-1.5 px-2 py-0.5 text-[11px] font-bold bg-[#E6F4EA] text-[#137333] border border-[#CEEAD6] rounded-md">
                        <Check className="w-3 h-3" /> 활성
                      </span>
                    </td>
                    <td className="px-5 py-4 text-right">
                      <button
                        onClick={() => handleDelete(j.id)}
                        className="p-1.5 text-[#9CA3AF] hover:text-[#DC2626] hover:bg-[#FCE8E6] rounded-lg transition-colors"
                        title="스케줄 삭제"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* New Schedule Modal */}
      {modalOpen && (
        <div className="fixed inset-0 bg-black/40 backdrop-blur-xs z-50 flex items-center justify-center p-4">
          <div className="bg-white border border-[#EAECF0] rounded-[24px] shadow-2xl max-w-md w-full p-6 space-y-4">
            <div className="flex items-center justify-between border-b border-[#EAECF0] pb-3">
              <h3 className="text-base font-bold text-[#111827]">새 스케줄 알림 등록</h3>
              <button
                onClick={() => setModalOpen(false)}
                className="p-1 text-[#9CA3AF] hover:text-[#111827] rounded-lg"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleCreate} className="space-y-3.5 text-xs">
              <div>
                <label className="block font-bold text-[#374151] mb-1">알림 제목</label>
                <input
                  type="text"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="예: 팟캐스트 녹음 알림"
                  className="w-full bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                  required
                />
              </div>

              <div>
                <label className="block font-bold text-[#374151] mb-1">메시지 본문</label>
                <textarea
                  rows={3}
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                  placeholder="카카오톡으로 발송할 내용을 입력하세요"
                  className="w-full bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                  required
                />
              </div>

              <div>
                <label className="block text-xs font-bold text-[#374151] mb-2">알림 유형</label>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => setType('once')}
                    className={`btn ${type === 'once' ? 'btn-primary' : ''} flex-1`}
                  >
                    일회성 (Once)
                  </button>
                  <button
                    type="button"
                    onClick={() => setType('recurring')}
                    className={`btn ${type === 'recurring' ? 'btn-primary' : ''} flex-1`}
                  >
                    반복 크론 (Cron)
                  </button>
                </div>
              </div>

              {type === 'once' ? (
                <div>
                  <label className="block font-bold text-[#374151] mb-1">실행 시점 (상대 시간 또는 ISO8601)</label>
                  <input
                    type="text"
                    value={executeAt}
                    onChange={(e) => setExecuteAt(e.target.value)}
                    placeholder="예: 1h, 30m, 2026-09-18T10:00:00"
                    className="w-full bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                    required
                  />
                  <p className="text-[11px] text-[#6B7280] mt-1">상대 시간(예: 10m, 2h) 또는 특정 일시 지원</p>
                </div>
              ) : (
                <div>
                  <label className="block font-bold text-[#374151] mb-1">크론 표현식 (5필드)</label>
                  <input
                    type="text"
                    value={cronExpr}
                    onChange={(e) => setCronExpr(e.target.value)}
                    placeholder="예: 0 8 * * 1-5 (평일 오전 8시)"
                    className="w-full bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 font-mono text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                    required
                  />
                  <p className="text-[11px] text-[#6B7280] mt-1">표준 5필드 크론: 분 시 일 월 요일</p>
                </div>
              )}

              <div className="pt-2 flex items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setModalOpen(false)}
                  className="btn"
                >
                  취소
                </button>
                <button
                  type="submit"
                  disabled={creating}
                  className="btn btn-primary"
                >
                  {creating ? '등록 중…' : '스케줄 등록'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
