import React, { useState } from 'react'
import { ChatSimulateResponse, DobbyApiClient } from '../api/client'
import { Bot, Code, CornerDownLeft, Send, Sparkles, User, X } from 'lucide-react'

interface Props {
  client: DobbyApiClient
}

interface MessageItem {
  id: string
  role: 'user' | 'bot'
  text: string
  latency?: number
  raw?: any
}

export const SimulatorTab: React.FC<Props> = ({ client }) => {
  const [messages, setMessages] = useState<MessageItem[]>([
    {
      id: 'welcome',
      role: 'bot',
      text: '안녕하세요! 도비(Dobby) 카카오 AI 에이전트 시뮬레이터입니다. 슬래시 커맨드(/status, /bus 등)나 자연어로 질문해 보세요.',
    },
  ])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [selectedRaw, setSelectedRaw] = useState<any | null>(null)

  const quickCommands = [
    '/status',
    '/time',
    '/bus',
    '/school',
    '/alarm',
    '내일 태권도 버스 시간 알려줘',
    '오늘 급식 및 시간표 알려줘',
  ]

  const handleSend = async (textToSend?: string) => {
    const query = (textToSend || input).trim()
    if (!query || loading) return

    const userMsg: MessageItem = {
      id: `user-${Date.now()}`,
      role: 'user',
      text: query,
    }

    setMessages((prev) => [...prev, userMsg])
    if (!textToSend) setInput('')
    setLoading(true)

    try {
      const res = await client.simulateChat(query)
      const botMsg: MessageItem = {
        id: `bot-${Date.now()}`,
        role: 'bot',
        text: res.reply_text || '응답이 비어있습니다.',
        latency: res.latency_ms,
        raw: res.response,
      }
      setMessages((prev) => [...prev, botMsg])
    } catch (err: any) {
      const errMsg: MessageItem = {
        id: `err-${Date.now()}`,
        role: 'bot',
        text: `⚠️ 오류 발생: ${err.message || '시뮬레이션 요청에 실패했습니다.'}`,
      }
      setMessages((prev) => [...prev, errMsg])
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-4">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-xl bg-[#111827] text-white flex items-center justify-center text-xs font-black shadow-xs">
            D
          </div>
          <div>
            <h2 className="text-base font-bold text-[#111827]">도비 AI ReAct 시뮬레이터</h2>
            <p className="text-[11px] text-[#6B7280]">OpenBuilder Webhook Mock Runtime</p>
          </div>
        </div>

        <button
          onClick={() => setMessages([messages[0]])}
          className="btn btn-ghost"
        >
          대화 지우기
        </button>
      </div>

      {/* Quick Suggestion Pills */}
      <div className="flex items-center gap-1.5 overflow-x-auto pb-1">
        {quickCommands.map((cmd) => (
          <button
            key={cmd}
            onClick={() => handleSend(cmd)}
            disabled={loading}
            className="filter-pill"
          >
            {cmd}
          </button>
        ))}
      </div>

      {/* Chat Area */}
      <div className="bg-white border border-[#EAECF0] rounded-[18px] shadow-[0_1px_3px_rgba(0,0,0,0.04)] overflow-hidden flex flex-col h-[520px]">
        {/* Messages List */}
        <div className="flex-1 p-5 overflow-y-auto space-y-4">
          {messages.map((m) => {
            const isUser = m.role === 'user'
            return (
              <div
                key={m.id}
                className={`flex items-start gap-2.5 ${isUser ? 'flex-row-reverse' : 'flex-row'}`}
              >
                {/* Avatar */}
                <div
                  className={`w-7 h-7 rounded-xl flex items-center justify-center text-xs font-extrabold flex-shrink-0 shadow-xs ${
                    isUser ? 'bg-[#F1F3F5] text-[#111827]' : 'bg-[#111827] text-white'
                  }`}
                >
                  {isUser ? <User className="w-3.5 h-3.5" /> : 'D'}
                </div>

                {/* Bubble Container */}
                <div className={`space-y-1.5 max-w-[80%] ${isUser ? 'items-end' : 'items-start'}`}>
                  <div
                    className={`p-3.5 text-xs whitespace-pre-wrap leading-relaxed ${
                      isUser
                        ? 'bg-[#111827] text-white rounded-2xl rounded-tr-xs shadow-xs font-medium'
                        : 'bg-[#F8F9FA] border border-[#EAECF0] text-[#111827] rounded-2xl rounded-tl-xs shadow-xs'
                    }`}
                  >
                    {m.text}
                  </div>

                  {/* Metadata Row for Bot Messages */}
                  {!isUser && m.latency !== undefined && (
                    <div className="flex items-center gap-2 text-[10px] text-[#9CA3AF] px-1 font-medium">
                      <span>응답 시간: {m.latency}ms</span>
                      {m.raw && (
                        <button
                          onClick={() => setSelectedRaw(m.raw)}
                          className="text-[#111827] hover:underline flex items-center gap-1 font-bold"
                        >
                          <Code className="w-3 h-3" /> JSON 보기
                        </button>
                      )}
                    </div>
                  )}
                </div>
              </div>
            )
          })}

          {loading && (
            <div className="flex items-start gap-2.5">
              <div className="w-7 h-7 rounded-xl bg-[#111827] text-white flex items-center justify-center text-xs font-extrabold flex-shrink-0 shadow-xs">
                D
              </div>
              <div className="p-3.5 bg-[#F8F9FA] border border-[#EAECF0] rounded-2xl rounded-tl-xs text-xs text-[#6B7280] flex items-center gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-[#111827] animate-bounce"></div>
                <div className="w-1.5 h-1.5 rounded-full bg-[#111827] animate-bounce [animation-delay:0.2s]"></div>
                <div className="w-1.5 h-1.5 rounded-full bg-[#111827] animate-bounce [animation-delay:0.4s]"></div>
                <span className="font-semibold text-[11px] text-[#111827]">도비 에이전트가 생각 중…</span>
              </div>
            </div>
          )}
        </div>

        {/* Input Bar */}
        <div className="p-3 bg-[#F8F9FA] border-t border-[#EAECF0]">
          <form
            onSubmit={(e) => {
              e.preventDefault()
              handleSend()
            }}
            className="flex items-center gap-2"
          >
            <input
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="도비에게 명령하거나 물어보세요 (예: /status, 내일 학원 버스 몇 시야?)"
              className="flex-1 bg-white border border-[#EAECF0] rounded-xl px-4 py-2.5 text-xs text-[#111827] focus:outline-none focus:border-[#111827] focus:ring-1 focus:ring-[#111827] transition-all"
              disabled={loading}
            />
            <button
              type="submit"
              disabled={loading || !input.trim()}
              className="btn btn-primary"
            >
              <Send className="w-4 h-4 mr-1.5" />
              전송
            </button>
          </form>
        </div>
      </div>

      {/* Raw JSON Inspector Modal */}
      {selectedRaw && (
        <div className="fixed inset-0 bg-black/40 backdrop-blur-xs z-50 flex items-center justify-center p-4">
          <div className="bg-white border border-[#EAECF0] rounded-[24px] shadow-2xl max-w-2xl w-full p-6 space-y-4 max-h-[85vh] flex flex-col">
            <div className="flex items-center justify-between border-b border-[#EAECF0] pb-3">
              <h3 className="text-base font-bold text-[#111827]">OpenBuilder 응답 인스펙터</h3>
              <button
                onClick={() => setSelectedRaw(null)}
                className="p-1 text-[#9CA3AF] hover:text-[#111827] rounded-lg"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="flex-1 overflow-auto bg-[#F8F9FA] border border-[#EAECF0] p-4 rounded-xl">
              <pre className="font-mono text-xs text-[#111827] whitespace-pre overflow-x-auto">
                {JSON.stringify(selectedRaw, null, 2)}
              </pre>
            </div>

            <div className="pt-2 flex justify-end">
              <button
                onClick={() => setSelectedRaw(null)}
                className="btn btn-primary"
              >
                닫기
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
