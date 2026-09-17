import React, { useEffect, useState } from 'react'
import { BotStatus, DobbyApiClient, SendMessageRequest, SendMessageResponse } from '../api/client'
import { AlertCircle, CheckCircle2, ExternalLink, Send, Smartphone } from 'lucide-react'

interface Props {
  client: DobbyApiClient
}

export const MessageTab: React.FC<Props> = ({ client }) => {
  const [bots, setBots] = useState<BotStatus[]>([])
  const [selectedBot, setSelectedBot] = useState('')
  const [target, setTarget] = useState<'memo' | 'friends' | 'channel'>('memo')
  const [msgType, setMsgType] = useState<'text' | 'feed' | 'custom'>('text')

  const [text, setText] = useState('')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [imageUrl, setImageUrl] = useState('')
  const [webUrl, setWebUrl] = useState('')
  const [buttonTitle, setButtonTitle] = useState('자세히 보기')
  const [receiverUuids, setReceiverUuids] = useState('')
  const [channelId, setChannelId] = useState('0xc0de1ab')
  const [templateId, setTemplateId] = useState<number>(0)
  const [templateArgs, setTemplateArgs] = useState('')

  const [sending, setSending] = useState(false)
  const [result, setResult] = useState<SendMessageResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    client.getBotsStatus().then((res) => {
      setBots(res.bots || [])
      if (res.bots && res.bots.length > 0) {
        const def = res.bots.find((b) => b.is_default) || res.bots[0]
        setSelectedBot(def.name)
      }
    }).catch(() => {})
  }, [client])

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault()
    setSending(true)
    setError(null)
    setResult(null)

    try {
      const payload: SendMessageRequest = {
        bot: selectedBot,
        target,
        type: msgType,
      }

      if (msgType === 'text') {
        payload.text = text
        payload.web_url = webUrl || undefined
        payload.button_title = buttonTitle || undefined
      } else if (msgType === 'feed') {
        payload.title = title
        payload.description = description || text
        payload.image_url = imageUrl || undefined
        payload.web_url = webUrl || undefined
        payload.button_title = buttonTitle || undefined
      } else if (msgType === 'custom') {
        payload.template_id = templateId
        if (templateArgs.trim()) {
          try {
            payload.template_args = JSON.parse(templateArgs)
          } catch {
            throw new Error('템플릿 파라미터는 올바른 JSON 형식이어야 합니다.')
          }
        }
      }

      if (target === 'friends') {
        const uuids = receiverUuids.split(',').map((s) => s.trim()).filter(Boolean)
        if (uuids.length === 0) {
          throw new Error('친구에게 보내기는 1개 이상의 수신자 UUID가 필요합니다.')
        }
        payload.receiver_uuids = uuids
      } else if (target === 'channel') {
        payload.channel_id = channelId
      }

      const res = await client.sendMessage(payload)
      setResult(res)
    } catch (err: any) {
      setError(err.message || '메시지 발송에 실패했습니다.')
    } finally {
      setSending(false)
    }
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
      {/* Form Section */}
      <div className="lg:col-span-7 space-y-5">
        <div>
          <h2 className="text-lg font-bold text-[#111827]">메시지 발송 콘솔</h2>
          <p className="text-xs text-[#6B7280]">봇을 선택하여 나에게, 친구에게, 또는 채널에 카카오톡 메시지를 즉시 발송합니다.</p>
        </div>

        {error && (
          <div className="p-4 text-xs font-medium text-[#C5221F] bg-[#FCE8E6] border border-[#FAD2CF] rounded-2xl flex items-center gap-3">
            <AlertCircle className="w-4 h-4 flex-shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {result && (
          <div className="p-4 text-xs font-medium text-[#137333] bg-[#E6F4EA] border border-[#CEEAD6] rounded-2xl flex items-start gap-3">
            <CheckCircle2 className="w-4 h-4 text-[#137333] flex-shrink-0 mt-0.5" />
            <div className="space-y-1">
              <div className="font-bold">메시지 발송 요청이 완료되었습니다!</div>
              <div className="text-[11px] text-[#2D3748]">
                전송 성공: {result.delivered_count}건 / 실패: {result.failed_count}건 (Bot: {result.bot})
              </div>
            </div>
          </div>
        )}

        <form onSubmit={handleSend} className="p-6 bg-white border border-[#EAECF0] rounded-[18px] shadow-[0_1px_3px_rgba(0,0,0,0.04)] space-y-4">
          {/* Bot Selector */}
          <div>
            <label className="block text-xs font-bold text-[#374151] mb-1.5">발송 봇 (카카오 앱)</label>
            <select
              value={selectedBot}
              onChange={(e) => setSelectedBot(e.target.value)}
              className="w-full text-xs bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2.5 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
            >
              {bots.map((b) => (
                <option key={b.name} value={b.name}>
                  {b.name} {b.is_default ? '(기본 봇)' : ''}
                </option>
              ))}
            </select>
          </div>

          {/* Target Selector */}
          <div>
            <label className="block text-xs font-bold text-[#374151] mb-2">수신 대상 (Target)</label>
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() => setTarget('memo')}
                className={`btn ${target === 'memo' ? 'btn-primary' : ''}`}
              >
                나에게 보내기 (Talk Memo)
              </button>
              <button
                type="button"
                onClick={() => setTarget('friends')}
                className={`btn ${target === 'friends' ? 'btn-primary' : ''}`}
              >
                친구에게 보내기
              </button>
              <button
                type="button"
                onClick={() => setTarget('channel')}
                className={`btn ${target === 'channel' ? 'btn-primary' : ''}`}
              >
                채널 알림톡
              </button>
            </div>
          </div>

          {/* Conditional Target Inputs */}
          {target === 'friends' && (
            <div>
              <label className="block text-xs font-bold text-[#374151] mb-1">수신자 UUID 목록 (쉼표 구분)</label>
              <input
                type="text"
                value={receiverUuids}
                onChange={(e) => setReceiverUuids(e.target.value)}
                placeholder="uuid1, uuid2"
                className="w-full text-xs bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                required
              />
            </div>
          )}

          {target === 'channel' && (
            <div>
              <label className="block text-xs font-bold text-[#374151] mb-1">카카오톡 채널 Search ID</label>
              <input
                type="text"
                value={channelId}
                onChange={(e) => setChannelId(e.target.value)}
                placeholder="0xc0de1ab"
                className="w-full text-xs bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                required
              />
            </div>
          )}

          {/* Message Type Selector */}
          <div>
            <label className="block text-xs font-bold text-[#374151] mb-2">메시지 템플릿 포맷</label>
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() => setMsgType('text')}
                className={`btn ${msgType === 'text' ? 'btn-primary' : ''}`}
              >
                텍스트 (Simple Text)
              </button>
              <button
                type="button"
                onClick={() => setMsgType('feed')}
                className={`btn ${msgType === 'feed' ? 'btn-primary' : ''}`}
              >
                피드 카드 (Feed Card)
              </button>
              <button
                type="button"
                onClick={() => setMsgType('custom')}
                className={`btn ${msgType === 'custom' ? 'btn-primary' : ''}`}
              >
                커스텀 템플릿 (Template ID)
              </button>
            </div>
          </div>

          {/* Content Inputs */}
          {msgType === 'custom' ? (
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-bold text-[#374151] mb-1">카카오 개발자 템플릿 ID</label>
                <input
                  type="number"
                  value={templateId || ''}
                  onChange={(e) => setTemplateId(parseInt(e.target.value) || 0)}
                  placeholder="예: 123456"
                  className="w-full text-xs bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                  required
                />
              </div>
              <div>
                <label className="block text-xs font-bold text-[#374151] mb-1">템플릿 인자 (JSON)</label>
                <textarea
                  rows={3}
                  value={templateArgs}
                  onChange={(e) => setTemplateArgs(e.target.value)}
                  placeholder='{"title": "알림", "description": "내용"}'
                  className="w-full font-mono text-xs bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                />
              </div>
            </div>
          ) : (
            <div className="space-y-3">
              {msgType === 'feed' && (
                <>
                  <div>
                    <label className="block text-xs font-bold text-[#374151] mb-1">카드 제목 (Title)</label>
                    <input
                      type="text"
                      value={title}
                      onChange={(e) => setTitle(e.target.value)}
                      placeholder="🚨 [Alert] 클러스터 노드 경고"
                      className="w-full text-xs bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                      required
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-bold text-[#374151] mb-1">이미지 URL (선택)</label>
                    <input
                      type="url"
                      value={imageUrl}
                      onChange={(e) => setImageUrl(e.target.value)}
                      placeholder="https://example.com/image.png"
                      className="w-full text-xs bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                    />
                  </div>
                </>
              )}

              <div>
                <label className="block text-xs font-bold text-[#374151] mb-1">
                  {msgType === 'feed' ? '카드 상세 설명 (Description)' : '메시지 본문 (Text)'}
                </label>
                <textarea
                  rows={4}
                  value={text}
                  onChange={(e) => setText(e.target.value)}
                  placeholder="전송할 메시지 내용을 입력하세요..."
                  className="w-full text-xs bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                  required
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-bold text-[#374151] mb-1">클릭 이동 URL (Web URL)</label>
                  <input
                    type="url"
                    value={webUrl}
                    onChange={(e) => setWebUrl(e.target.value)}
                    placeholder="https://0xc0de1ab.dev"
                    className="w-full text-xs bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                  />
                </div>
                <div>
                  <label className="block text-xs font-bold text-[#374151] mb-1">버튼 문구 (Button Title)</label>
                  <input
                    type="text"
                    value={buttonTitle}
                    onChange={(e) => setButtonTitle(e.target.value)}
                    placeholder="자세히 보기"
                    className="w-full text-xs bg-white border border-[#EAECF0] rounded-xl px-3.5 py-2 text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827]"
                  />
                </div>
              </div>
            </div>
          )}

          <div className="pt-2">
            <button
              type="submit"
              disabled={sending}
              className="btn btn-primary w-full"
            >
              <Send className="w-4 h-4 mr-2" />
              {sending ? '카카오톡 전송 중…' : '카카오톡 메시지 즉시 발송'}
            </button>
          </div>
        </form>
      </div>

      {/* Mobile Preview Section */}
      <div className="lg:col-span-5 flex flex-col items-center">
        <div className="w-full flex items-center justify-between mb-3 px-1">
          <div className="text-[11px] font-bold text-[#6B7280] uppercase tracking-wider flex items-center gap-1.5">
            <Smartphone className="w-3.5 h-3.5" /> 모바일 카카오톡 렌더링 미리보기
          </div>
          <span className="text-[11px] font-semibold text-[#6B7280]">앱: {selectedBot || 'dobby'}</span>
        </div>

        {/* Mobile Device Mockup */}
        <div className="w-full max-w-[340px] bg-[#B2C7D9] p-4 rounded-[28px] border-4 border-[#CBD5E1] shadow-md min-h-[460px] flex flex-col justify-between">
          <div className="space-y-3">
            {/* Header / Date */}
            <div className="text-center">
              <span className="text-[10px] text-white/90 bg-black/15 px-2.5 py-0.5 rounded-full font-medium">
                오늘
              </span>
            </div>

            {/* Bubble Container */}
            <div className="flex items-start gap-2">
              <div className="w-8 h-8 rounded-full bg-[#FEE500] flex items-center justify-center text-[#111827] text-xs font-black flex-shrink-0 shadow-xs">
                {selectedBot ? selectedBot[0].toUpperCase() : 'D'}
              </div>

              <div className="flex-1 max-w-[240px]">
                <div className="text-[11px] font-bold text-slate-800 mb-1">
                  {selectedBot || '도비'}
                </div>

                {msgType === 'feed' ? (
                  <div className="bg-white rounded-2xl overflow-hidden shadow-xs border border-slate-100">
                    {imageUrl ? (
                      <img src={imageUrl} alt="preview" className="w-full h-28 object-cover" />
                    ) : (
                      <div className="w-full h-24 bg-slate-100 flex items-center justify-center text-slate-400 text-xs font-medium">
                        이미지 미리보기
                      </div>
                    )}
                    <div className="p-3">
                      <div className="font-bold text-xs text-slate-900 line-clamp-2">
                        {title || '카드 제목이 여기에 표시됩니다'}
                      </div>
                      <div className="text-[11px] text-slate-600 mt-1 line-clamp-3 whitespace-pre-wrap">
                        {text || '상세 설명 텍스트가 여기에 표시됩니다.'}
                      </div>
                      {webUrl && (
                        <div className="mt-3 pt-2 border-t border-slate-100 flex items-center justify-center gap-1 text-[11px] font-bold text-slate-800">
                          {buttonTitle || '자세히 보기'} <ExternalLink className="w-3 h-3" />
                        </div>
                      )}
                    </div>
                  </div>
                ) : (
                  <div className="bg-[#FEE500] p-3 rounded-2xl rounded-tl-xs shadow-xs text-xs text-[#111827] font-medium whitespace-pre-wrap break-words">
                    {text || '메시지 본문 텍스트가 여기에 표시됩니다.'}
                    {webUrl && (
                      <div className="mt-2.5 pt-2 border-t border-amber-300/60 text-[11px] text-slate-800 font-bold flex items-center gap-1">
                        {buttonTitle || '링크 열기'} <ExternalLink className="w-3 h-3" />
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
          </div>

          <div className="text-center text-[10px] text-white/70 py-1 font-medium">
            실제 카카오톡 앱 뷰와 100% 동일하게 전달됩니다
          </div>
        </div>
      </div>
    </div>
  )
}
