import React, { useEffect, useState } from 'react'
import { CatalogsData, DobbyApiClient } from '../api/client'
import { Bus, CheckCircle2, FileUp, GraduationCap, RefreshCw, UploadCloud } from 'lucide-react'

interface Props {
  client: DobbyApiClient
}

export const CatalogsTab: React.FC<Props> = ({ client }) => {
  const [catalogs, setCatalogs] = useState<CatalogsData>({})
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [statusMsg, setStatusMsg] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const loadCatalogs = async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await client.getCatalogs()
      setCatalogs(data)
    } catch (err: any) {
      setError(err.message || '카탈로그 데이터를 불러오지 못했습니다.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadCatalogs()
  }, [])

  const handleReload = async () => {
    setLoading(true)
    setStatusMsg(null)
    try {
      await client.reloadData()
      setStatusMsg('데이터 카탈로그를 성공적으로 다시 로드했습니다.')
      loadCatalogs()
    } catch (err: any) {
      setError(err.message || '데이터 리로드 실패')
    } finally {
      setLoading(false)
    }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>, dataType: string) => {
    const file = e.target.files?.[0]
    if (!file) return

    setUploading(true)
    setStatusMsg(null)
    setError(null)

    try {
      await client.uploadDataFile(file, dataType)
      setStatusMsg(`파일 '${file.name}' 업로드 및 반영 완료!`)
      loadCatalogs()
    } catch (err: any) {
      setError(err.message || '파일 업로드 실패')
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-[#111827]">데이터 카탈로그</h2>
          <p className="text-xs text-[#6B7280]">학원 셔틀버스 시간표 및 초등학교 시간표/수칙 데이터를 관리합니다.</p>
        </div>
        <button
          onClick={handleReload}
          disabled={loading || uploading}
          className="btn"
        >
          <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${loading ? 'animate-spin' : ''}`} />
          데이터 즉시 리로드
        </button>
      </div>

      {statusMsg && (
        <div className="p-4 text-xs font-medium text-[#137333] bg-[#E6F4EA] border border-[#CEEAD6] rounded-2xl flex items-center gap-2">
          <CheckCircle2 className="w-4 h-4 text-[#137333] flex-shrink-0" />
          <span>{statusMsg}</span>
        </div>
      )}

      {error && (
        <div className="p-4 text-xs font-medium text-[#C5221F] bg-[#FCE8E6] border border-[#FAD2CF] rounded-2xl flex items-center gap-2">
          <span>{error}</span>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Upload Bus Timetable */}
        <div className="bg-white border border-[#EAECF0] rounded-[18px] p-6 shadow-[0_1px_3px_rgba(0,0,0,0.04)] space-y-4">
          <div className="flex items-center gap-3">
            <div className="p-2.5 bg-[#F1F3F5] text-[#111827] rounded-xl">
              <Bus className="w-5 h-5" />
            </div>
            <div>
              <h3 className="font-bold text-sm text-[#111827]">학원 셔틀버스 시간표 업로드</h3>
              <p className="text-xs text-[#6B7280]">CSV 또는 TXT 파일 지원</p>
            </div>
          </div>

          <label className="border-2 border-dashed border-[#EAECF0] rounded-2xl p-6 flex flex-col items-center justify-center gap-2 hover:border-[#111827] hover:bg-[#F8F9FA] transition-all cursor-pointer">
            <UploadCloud className="w-8 h-8 text-[#9CA3AF]" />
            <span className="text-xs font-bold text-[#111827]">클릭하여 파일 선택</span>
            <span className="text-[11px] text-[#6B7280]">UTF-8 인코딩 텍스트 파일 권장</span>
            <input
              type="file"
              accept=".csv,.txt,.json"
              className="hidden"
              onChange={(e) => handleFileUpload(e, 'bus')}
              disabled={uploading}
            />
          </label>
        </div>

        {/* Upload School Timetable */}
        <div className="bg-white border border-[#EAECF0] rounded-[18px] p-6 shadow-[0_1px_3px_rgba(0,0,0,0.04)] space-y-4">
          <div className="flex items-center gap-3">
            <div className="p-2.5 bg-[#F1F3F5] text-[#111827] rounded-xl">
              <GraduationCap className="w-5 h-5" />
            </div>
            <div>
              <h3 className="font-bold text-sm text-[#111827]">초등학교 시간표/수칙 업로드</h3>
              <p className="text-xs text-[#6B7280]">시간표 텍스트 또는 생활 수칙 파일</p>
            </div>
          </div>

          <label className="border-2 border-dashed border-[#EAECF0] rounded-2xl p-6 flex flex-col items-center justify-center gap-2 hover:border-[#111827] hover:bg-[#F8F9FA] transition-all cursor-pointer">
            <FileUp className="w-8 h-8 text-[#9CA3AF]" />
            <span className="text-xs font-bold text-[#111827]">클릭하여 파일 선택</span>
            <span className="text-[11px] text-[#6B7280]">학사 일정 및 시간표 파일 권장</span>
            <input
              type="file"
              accept=".csv,.txt,.json"
              className="hidden"
              onChange={(e) => handleFileUpload(e, 'school')}
              disabled={uploading}
            />
          </label>
        </div>
      </div>

      {/* Catalog Status Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-white border border-[#EAECF0] rounded-[18px] p-6 shadow-[0_1px_3px_rgba(0,0,0,0.04)]">
          <div className="flex items-center justify-between mb-4">
            <div className="font-bold text-sm text-[#111827] flex items-center gap-2">
              <Bus className="w-4 h-4 text-[#111827]" />
              등록된 학원 셔틀 노선
            </div>
            <span className="text-[11px] font-bold px-2 py-0.5 bg-[#F1F3F5] text-[#4B5563] border border-[#EAECF0] rounded-md">
              총 {catalogs.academyRoutesCount || 0}개 학원
            </span>
          </div>

          {catalogs.academyRoutes && catalogs.academyRoutes.length > 0 ? (
            <div className="space-y-3">
              {catalogs.academyRoutes.map((acad: any, idx: number) => {
                const name = typeof acad === 'string' ? acad : acad.name
                const contact = typeof acad === 'object' ? acad.contact : ''
                const note = typeof acad === 'object' ? acad.note : ''
                return (
                  <div key={idx} className="p-3.5 bg-[#F8F9FA] border border-[#EAECF0] rounded-xl text-xs space-y-1">
                    <div className="font-bold text-[#111827]">{name}</div>
                    {contact && <div className="text-[#6B7280]">연락처: {contact}</div>}
                    {note && <div className="text-[11px] text-[#9CA3AF]">{note}</div>}
                  </div>
                )
              })}
            </div>
          ) : (
            <div className="text-center py-8 text-[#9CA3AF] text-xs">
              등록된 셔틀버스 노선이 없습니다.
            </div>
          )}
        </div>

        <div className="bg-white border border-[#EAECF0] rounded-[18px] p-6 shadow-[0_1px_3px_rgba(0,0,0,0.04)]">
          <div className="flex items-center justify-between mb-4">
            <div className="font-bold text-sm text-[#111827] flex items-center gap-2">
              <GraduationCap className="w-4 h-4 text-[#111827]" />
              초등학교 시간표 & 생활 수칙
            </div>
          </div>

          {catalogs.schoolTimetables ? (
            <div className="p-4 bg-[#F8F9FA] border border-[#EAECF0] rounded-xl font-mono text-xs text-[#111827] whitespace-pre-wrap">
              {catalogs.schoolTimetables}
            </div>
          ) : (
            <div className="text-center py-8 text-[#9CA3AF] text-xs">
              등록된 학교 시간표가 없습니다.
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
