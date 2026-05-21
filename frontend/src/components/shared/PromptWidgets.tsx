import { Heart, Copy, XCircle, Image } from 'lucide-react'
import type { PromptGalleryItem } from '@/types/app'

export function PromptCard({
  item,
  priority,
  onFavorite,
  onOpenDetail,
}: {
  item: PromptGalleryItem & { serialNo?: string; favorite?: boolean; displayFavorites?: number }
  priority?: boolean
  onFavorite: () => void
  onOpenDetail: () => void
}) {
  const displayImageUrl = item.previewThumbUrl || item.previewUrl
  return (
    <article
      onClick={onOpenDetail}
      className="prompt-card group cursor-pointer overflow-hidden rounded-lg border shadow-panel transition"
    >
      <div className="prompt-card-media relative aspect-[5/4]">
        {displayImageUrl ? (
          <img
            src={displayImageUrl}
            alt={item.title}
            loading={priority ? 'eager' : 'lazy'}
            decoding="async"
            className="h-full w-full bg-slate-800 object-cover"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-slate-500">
            <Image className="h-8 w-8" />
          </div>
        )}
      </div>
      <div className="prompt-card-body flex min-h-[92px] flex-col p-2">
        <div className="min-w-0 flex-1">
          <h3 className="line-clamp-2 min-h-9 text-[13px] font-semibold leading-[18px]">{item.title}</h3>
          {item.tags?.length ? (
            <div className="mt-1 flex flex-wrap gap-1">
              {item.tags.slice(0, 4).map((tag) => (
                <span key={tag} className="prompt-card-tag rounded-full bg-slate-800/80 px-1.5 py-0.5 text-[10px] leading-4 text-slate-300">
                  #{tag}
                </span>
              ))}
            </div>
          ) : null}
        </div>
        <div className="prompt-card-footer mt-1.5 grid grid-cols-[auto_auto] items-center justify-between gap-2">
          <span className="prompt-card-serial text-xs font-medium">{item.serialNo ? `#${item.serialNo}` : '-'}</span>
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation()
              onFavorite()
            }}
            className={`prompt-card-like inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs transition ${
              item.favorite ? 'is-active' : ''
            }`}
          >
            <Heart className="h-3.5 w-3.5" />
            {item.displayFavorites ?? item.favorites}
          </button>
        </div>
      </div>
    </article>
  )
}

export function PromptDetailDialog({
  item,
  onClose,
  onFavorite,
  onCopy,
}: {
  item: (PromptGalleryItem & { serialNo?: string; favorite?: boolean; displayFavorites?: number }) | null
  onClose: () => void
  onFavorite: () => void
  onCopy: () => void
}) {
  if (!item) return null
  const displayImageUrl = item.previewThumbUrl || item.previewUrl
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 p-4">
      <button type="button" className="absolute inset-0" aria-label="关闭详情" onClick={onClose} />
      <div className="relative w-full max-w-lg overflow-hidden rounded-xl border border-slate-700 bg-slate-900 shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-800 px-4 py-3">
          <div className="min-w-0">
            <p className="text-xs text-slate-400">上传人：{item.author || '-'}</p>
            <h3 className="truncate text-base font-semibold text-slate-100">{item.title}</h3>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-slate-700 text-slate-300 transition hover:border-slate-500 hover:text-white"
          >
            <XCircle className="h-4 w-4" />
          </button>
        </div>
        <div className="max-h-[72vh] overflow-y-auto p-4">
          <p className="mb-2 text-xs text-slate-500">为保证性能体验，展示图质量会进行压缩</p>
          <div className="overflow-hidden rounded-lg border border-slate-800 bg-slate-950">
            {displayImageUrl ? (
              <img src={displayImageUrl} alt={item.title} loading="lazy" decoding="async" className="h-auto w-full object-cover" />
            ) : (
              <div className="flex h-40 items-center justify-center text-slate-500">
                <Image className="h-8 w-8" />
              </div>
            )}
          </div>
          <div className="mt-4 whitespace-pre-wrap rounded-lg border border-slate-800 bg-slate-950/70 p-3 text-sm leading-6 text-slate-200">
            {item.prompt}
          </div>
          {item.tags?.length ? (
            <div className="mt-3 flex flex-wrap gap-2">
              {item.tags.slice(0, 4).map((tag) => (
                <span key={tag} className="rounded-full border border-slate-700 px-2 py-1 text-xs text-slate-300">
                  #{tag}
                </span>
              ))}
            </div>
          ) : null}
          <div className="mt-4 flex items-center justify-between gap-2">
            <button
              type="button"
              onClick={onFavorite}
              className={`inline-flex h-10 items-center gap-2 rounded-md border px-3 text-sm transition ${
                item.favorite
                  ? 'border-rose-400/50 bg-rose-500/10 text-rose-100'
                  : 'border-slate-700 text-slate-200 hover:border-slate-500'
              }`}
            >
              <Heart className="h-4 w-4" />
              {item.displayFavorites ?? item.favorites}
            </button>
            <button
              type="button"
              onClick={onCopy}
              className="inline-flex h-10 items-center gap-2 rounded-md bg-violet-500 px-4 text-sm font-medium text-slate-950 transition hover:bg-violet-400"
            >
              <Copy className="h-4 w-4" />
              复制提示词
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
