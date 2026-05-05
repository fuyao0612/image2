import { useEffect, useRef } from 'react'

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
    }, 2000)

    return () => window.clearTimeout(timer)
  }, [message])

  if (!message.trim()) {
    return null
  }

  return (
    <div
      className="pointer-events-none fixed left-1/2 top-4 z-[80] w-[min(92vw,42rem)] -translate-x-1/2 px-4"
      role="status"
      aria-live="polite"
    >
      <div className="rounded-md border border-slate-700/90 bg-slate-950/95 px-4 py-2 text-sm leading-6 text-slate-100 shadow-2xl shadow-slate-950/40 backdrop-blur">
        <div className="whitespace-pre-wrap break-words">{message}</div>
      </div>
    </div>
  )
}
