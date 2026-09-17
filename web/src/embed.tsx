import React from 'react'
import ReactDOM from 'react-dom/client'
import './styles/index.css'
import { App, AppProps } from './App'

interface ErrorBoundaryProps {
  children: React.ReactNode
}

interface ErrorBoundaryState {
  hasError: boolean
  error: Error | null
}

class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('[Dobby] Uncaught error in embedded React tree:', error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="dobby-shell p-6 bg-red-50 border border-red-200 rounded-2xl text-red-900">
          <h2 className="text-base font-bold mb-2">도비 플러그인 로드 중 오류가 발생했습니다.</h2>
          <p className="text-xs font-mono bg-white p-3 rounded-lg border border-red-100 overflow-x-auto">
            {this.state.error?.message || '알 수 없는 에러'}
          </p>
        </div>
      )
    }
    return this.props.children
  }
}

export function mount(container: HTMLElement, props: AppProps = {}): () => void {
  if (!container) {
    throw new Error('[Dobby] Mount container element is required')
  }

  const root = ReactDOM.createRoot(container)
  root.render(
    <React.StrictMode>
      <ErrorBoundary>
        <App embedded={true} {...props} />
      </ErrorBoundary>
    </React.StrictMode>
  )

  return () => {
    try {
      root.unmount()
    } catch (err) {
      console.warn('[Dobby] Error during unmount:', err)
    }
  }
}

export default App
