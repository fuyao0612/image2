import { useEffect } from 'react'

let lockCount = 0
let previousBodyOverflow = ''
let previousBodyWidth = ''
let previousHtmlOverflow = ''
let previousBodyTouchAction = ''
let previousHtmlOverscrollBehavior = ''

export function useBodyScrollLock(locked: boolean) {
  useEffect(() => {
    if (!locked || typeof document === 'undefined') {
      return
    }

    const body = document.body
    const html = document.documentElement

    if (lockCount === 0) {
      previousBodyOverflow = body.style.overflow
      previousBodyWidth = body.style.width
      previousHtmlOverflow = html.style.overflow
      previousBodyTouchAction = body.style.touchAction
      previousHtmlOverscrollBehavior = html.style.overscrollBehavior

      html.style.overflow = 'hidden'
      html.style.overscrollBehavior = 'none'
      body.style.overflow = 'hidden'
      body.style.touchAction = 'none'
      body.style.width = '100%'
    }

    lockCount += 1

    return () => {
      lockCount = Math.max(0, lockCount - 1)
      if (lockCount !== 0) {
        return
      }

      html.style.overflow = previousHtmlOverflow
      html.style.overscrollBehavior = previousHtmlOverscrollBehavior
      body.style.overflow = previousBodyOverflow
      body.style.touchAction = previousBodyTouchAction
      body.style.width = previousBodyWidth
    }
  }, [locked])
}
