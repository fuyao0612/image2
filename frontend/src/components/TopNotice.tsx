import { useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'

export function TopNotice({
  message,
  onDismiss,
}: {
  message: string
  onDismiss: () => void
}) {
  const onDismissRef = useRef(onDismiss)

  useEffect(() => {
    onDismissRef.current = onDismiss
  }, [onDismiss])

  useEffect(() => {
    if (!message.trim()) {
      return
    }

    const timer = window.setTimeout(() => {
      onDismissRef.current()
    }, 3000)

    return () => window.clearTimeout(timer)
  }, [message])

  if (!message.trim()) {
    return null
  }

  const notice = (
    <div className="top-notice-root pointer-events-none fixed inset-x-0 top-3 z-[100] flex justify-center px-4" role="status" aria-live="polite">
      <div className="w-full max-w-[22rem] rounded-lg border border-slate-700/90 bg-slate-950/95 px-3 py-2 text-xs leading-5 text-slate-100 shadow-xl shadow-slate-950/30 backdrop-blur sm:text-sm sm:leading-6">
        <div className="whitespace-pre-wrap break-words">{message}</div>
      </div>
    </div>
  )

  if (typeof document === 'undefined') {
    return notice
  }

  return createPortal(notice, document.body)
}
