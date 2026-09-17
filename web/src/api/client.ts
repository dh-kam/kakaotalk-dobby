export interface BotStatus {
  name: string
  is_default: boolean
  has_token: boolean
  is_expired: boolean
  refresh_token_expired: boolean
  expires_in: number
  created_at?: string
  scope?: string
}

export interface ScheduleJob {
  id: string
  user_id?: string
  title: string
  message: string
  type: string
  execute_at?: string
  cron_expr?: string
  created_at: string
  status: string
  payload?: Record<string, string>
}

export interface SendMessageRequest {
  bot?: string
  target: 'memo' | 'friends' | 'channel'
  type?: 'text' | 'feed' | 'custom'
  text?: string
  title?: string
  description?: string
  image_url?: string
  web_url?: string
  button_title?: string
  receiver_uuids?: string[]
  channel_id?: string
  template_id?: number
  template_args?: Record<string, string>
}

export interface SendMessageResponse {
  success: boolean
  bot: string
  target: string
  type: string
  message?: string
  delivered_count: number
  failed_count: number
  details?: {
    successful_receivers?: string[]
    failures?: Array<{ code: number; msg: string; receiver_uuids: string[] }>
    channel_info?: Record<string, any>
  }
  error?: string
}

export interface CatalogsData {
  academyRoutesCount?: number
  academyRoutes?: Array<{
    name: string
    contact: string
    note: string
    routesCount: number
    stopsCount: number
  }>
  schoolTimetables?: string
}

export interface ChatSimulateResponse {
  success: boolean
  utterance: string
  reply_text: string
  latency_ms: number
  response?: any
  error?: string
}

export class DobbyApiClient {
  private apiBase: string

  constructor(apiBase: string = '/dobby/api') {
    this.apiBase = apiBase.replace(/\/+$/, '')
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const url = `${this.apiBase}${path}`
    const headers = new Headers(options.headers || {})
    if (!headers.has('Content-Type') && !(options.body instanceof FormData)) {
      headers.set('Content-Type', 'application/json')
    }

    const res = await fetch(url, {
      ...options,
      headers,
    })

    if (!res.ok) {
      let errMsg = `Request failed: ${res.status} ${res.statusText}`
      try {
        const errJson = await res.json()
        if (errJson.error) errMsg = errJson.error
        else if (errJson.message) errMsg = errJson.message
      } catch {}
      throw new Error(errMsg)
    }

    return res.json()
  }

  async getBotsStatus(): Promise<{ bots: BotStatus[]; count: number }> {
    return this.request('/bots/status')
  }

  async sendMessage(req: SendMessageRequest): Promise<SendMessageResponse> {
    return this.request('/message/send', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  }

  async getSchedules(): Promise<{ jobs: ScheduleJob[]; count: number }> {
    return this.request('/schedules')
  }

  async createSchedule(data: {
    title: string
    message: string
    type?: string
    execute_at?: string
    cron_expr?: string
    bot?: string
  }): Promise<{ success: boolean; job: ScheduleJob }> {
    return this.request('/schedules', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  async deleteSchedule(id: string): Promise<{ success: boolean; deleted: string }> {
    return this.request(`/schedules?id=${encodeURIComponent(id)}`, {
      method: 'DELETE',
    })
  }

  async getCatalogs(): Promise<CatalogsData> {
    const res = await this.request<{ catalogs: CatalogsData }>('/data/catalogs')
    return res.catalogs || {}
  }

  async uploadDataFile(file: File, dataType: string): Promise<any> {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('data_type', dataType)

    return this.request('/data/upload', {
      method: 'POST',
      body: formData,
    })
  }

  async reloadData(): Promise<any> {
    return this.request('/data/reload', {
      method: 'POST',
    })
  }

  async simulateChat(utterance: string): Promise<ChatSimulateResponse> {
    return this.request('/agent/chat', {
      method: 'POST',
      body: JSON.stringify({ utterance }),
    })
  }
}
