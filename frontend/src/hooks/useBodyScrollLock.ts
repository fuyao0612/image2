import { useEffect } from 'react'

let lockCount = 0
let previousBodyOverflow = ''
let previousBodyWidth = ''
let previousHtmlOverflow = ''
let previousBodyTouchAction = ''
let previousHtmlOverscrollBehavior = ''
let previousBodyPosition = ''
let previousBodyTop = ''
let scrollY = 0

export function useBodyScrollLock(locked: boolean) {
  useEffect(() => {
    if (!locked || typeof window === 'undefined') {
      return
    }

    const body = document.body
    const html = document.documentElement

    if (lockCount === 0) {
      scrollY = window.scrollY
      previousBodyOverflow = body.style.overflow
      previousBodyWidth = body.style.width
      previousHtmlOverflow = html.style.overflow
      previousBodyTouchAction = body.style.touchAction
      previousHtmlOverscrollBehavior = html.style.overscrollBehavior
      previousBodyPosition = body.style.position
      previousBodyTop = body.style.top

      // To prevent jump to top on mobile browsers (like iOS Safari), 
      // we must use position: fixed with top: -scrollY.
      body.style.position = 'fixed'
      body.style.top = `-${scrollY}px`
      body.style.width = '100%'
      body.style.overflow = 'hidden'
      body.style.touchAction = 'none'
      html.style.overscrollBehavior = 'none'
    }

    lockCount += 1

    return () => {
      lockCount = Math.max(0, lockCount - 1)
      if (lockCount !== 0) {
        return
      }

      body.style.position = previousBodyPosition
      body.style.top = previousBodyTop
      body.style.overflow = previousBodyOverflow
      body.style.width = previousBodyWidth
      body.style.touchAction = previousBodyTouchAction
      html.style.overflow = previousHtmlOverflow
      html.style.overscrollBehavior = previousHtmlOverscrollBehavior

      // Restore scroll position
      window.scrollTo(0, scrollY)
    }
  }, [locked])
}
