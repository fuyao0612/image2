import { useState, useEffect, useMemo, useRef, ChangeEvent, DragEvent, ClipboardEvent } from 'react';
import { api } from '@/api/client';
import type { PromptGalleryItem } from '@/types/app';
import { useAppContext } from '@/contexts/AppContext';
import { Heart, Upload, ArrowUpDown, ChevronDown, Search, XCircle, LoaderCircle, ImagePlus, Send, Image } from "lucide-react";
import { PromptCard, Field, PromptDetailDialog, parsePromptTagString, copyTextToClipboard } from "../App";
import { PromptSortMode } from "../App";

const featuredPromptItems: PromptGalleryItem[] = [];

export function PromptsPage() {
  const { token, setMessage, handleApiError } = useAppContext();
  
  const [serverPromptItems, setServerPromptItems] = useState<PromptGalleryItem[]>([]);
  const [myPromptItems, setMyPromptItems] = useState<PromptGalleryItem[]>([]);
  const [promptSubmission, setPromptSubmission] = useState<{
    open: boolean;
    title: string;
    prompt: string;
    previewUrl: string;
    tags: string[];
  }>({ open: false, title: '', prompt: '', previewUrl: '', tags: [] });

  const promptGalleryItems = useMemo(
    () =>
      [...serverPromptItems, ...featuredPromptItems].map((item) => ({
        ...item,
        tags: normalizePromptTags(item.tags ?? []),
        approvedAt: item.approvedAt ?? (item.status === 'approved' ? item.createdAt : undefined),
      })),
    [serverPromptItems],
  );

  const favoritePromptIds = useMemo(
    () => promptGalleryItems.filter((item) => item.isFavorited).map((item) => item.id),
    [promptGalleryItems]
  );

  async function loadPromptsPage() {
    try {
      const [promptPlazaResult, myPromptPlazaResult] = await Promise.all([
        api.listPromptPlazaForUser().catch(() => null),
        api.listMyPromptPlaza().catch(() => null),
      ]);
      if (promptPlazaResult) setServerPromptItems(promptPlazaResult.items ?? []);
      if (myPromptPlazaResult) setMyPromptItems(myPromptPlazaResult.items ?? []);
    } catch (error) {
      console.error(error);
    }
  }

  useEffect(() => {
    void loadPromptsPage();
  }, [token]);

  async function handleTogglePromptFavorite(promptId: string) {
    if (!token) {
      setMessage('请先登录。');
      return;
    }
    const currentItem = serverPromptItems.find((item) => item.id === promptId);
    if (currentItem) {
      const nextFavorited = !currentItem.isFavorited;
      const nextFavorites = Math.max(0, Number(currentItem.favorites || 0) + (nextFavorited ? 1 : -1));
      setServerPromptItems((current) =>
        current.map((item) =>
          item.id === promptId ? { ...item, isFavorited: nextFavorited, favorites: nextFavorites } : item,
        ),
      );
    }
    try {
      const updated = await api.togglePromptPlazaFavorite(promptId);
      setServerPromptItems((current) =>
        current.map((item) =>
          item.id === updated.id ? { ...item, favorites: updated.favorites, isFavorited: updated.isFavorited } : item,
        ),
      );
      setMyPromptItems((current) =>
        current.map((item) =>
          item.id === updated.id ? { ...item, favorites: updated.favorites, isFavorited: updated.isFavorited } : item,
        ),
      );
    } catch (error) {
      if (currentItem) {
        setServerPromptItems((current) => current.map((item) => (item.id === currentItem.id ? currentItem : item)));
      }
      handleApiError(error, '收藏操作失败');
    }
  }

  async function handleSubmitPromptDraft() {
    if (!token) {
      setMessage('请先登录。');
      return;
    }
    const title = promptSubmission.title.trim();
    const promptText = promptSubmission.prompt.trim();
    if (!title || !promptText) {
      setMessage('请先填写提示词标题和正文。');
      return;
    }

    try {
      await api.submitPromptPlaza({
        title,
        prompt: promptText,
        previewUrl: promptSubmission.previewUrl.trim(),
        tags: normalizePromptTags(promptSubmission.tags),
      });
      setPromptSubmission({ open: false, title: '', prompt: '', previewUrl: '', tags: [] });
      setMessage('提示词已提交，等待审核。');
      void loadPromptsPage();
    } catch (error) {
      handleApiError(error, '提示词提交失败');
    }
  }

  return (
    <PromptPlaza
      items={promptGalleryItems}
      myItems={myPromptItems}
      favoriteIds={favoritePromptIds}
      submission={promptSubmission}
      onSubmissionChange={(value) => setPromptSubmission(prev => ({ ...prev, ...value }))}
      onSubmit={handleSubmitPromptDraft}
      onFavorite={handleTogglePromptFavorite}
      onNotify={setMessage}
    />
  );
}
export function PromptPlaza({
  items,
  myItems,
  favoriteIds,
  submission,
  onSubmissionChange,
  onSubmit,
  onFavorite,
  onNotify,
}: {
  items: PromptGalleryItem[]
  myItems: PromptGalleryItem[]
  favoriteIds: string[]
  submission: { title: string; prompt: string; previewUrl: string; tags: string[] }
  onSubmissionChange: (value: { title: string; prompt: string; previewUrl: string; tags: string[] }) => void
  onSubmit: () => void
  onFavorite: (id: string) => void
  onNotify: (value: string) => void
}) {
  const [favoritesOnly, setFavoritesOnly] = useState(false)
  const [myUploadsOnly, setMyUploadsOnly] = useState(false)
  const [showSubmissionModal, setShowSubmissionModal] = useState(false)
  const [sortMode, setSortMode] = useState<PromptSortMode>('recommended')
  const [sortOpen, setSortOpen] = useState(false)
  const [searchKeyword, setSearchKeyword] = useState('')
  const [activeDetailId, setActiveDetailId] = useState<string | null>(null)
  const [submissionImageUploading, setSubmissionImageUploading] = useState(false)
  const [submissionDragActive, setSubmissionDragActive] = useState(false)
  const [submissionImageName, setSubmissionImageName] = useState('')
  const submissionImageInputRef = useRef<HTMLInputElement | null>(null)
  const approvedItems = useMemo(() => items.filter((item) => item.status === 'approved'), [items])

  const serialMap = useMemo(() => {
    const ordered = [...approvedItems].sort((a, b) => {
      const aTime = new Date(a.approvedAt ?? a.createdAt).getTime()
      const bTime = new Date(b.approvedAt ?? b.createdAt).getTime()
      if (aTime !== bTime) return aTime - bTime
      return a.id.localeCompare(b.id)
    })
    const map = new Map<string, string>()
    ordered.forEach((item, index) => {
      map.set(item.id, String(index + 1))
    })
    return map
  }, [approvedItems])

  const enrichedItems = useMemo(
    () =>
      approvedItems.map((item, index) => {
        const serialNo = serialMap.get(item.id) ?? ''
        const favorite = Boolean(item.isFavorited)
        return {
          ...item,
          serialNo,
          favorite,
          displayFavorites: item.favorites,
          recommendedOrder: index,
        }
      }),
    [approvedItems, serialMap],
  )

  const visibleItems = useMemo(() => {
    const keyword = searchKeyword.trim().toLowerCase()
    const keywordSerial = keyword.replace('#', '')

    let next = myUploadsOnly
      ? myItems.map((item, index) => ({ ...item, serialNo: item.status === 'approved' ? serialMap.get(item.id) ?? '' : '', favorite: Boolean(item.isFavorited), displayFavorites: item.favorites, recommendedOrder: index }))
      : enrichedItems
    if (favoritesOnly) {
      next = next.filter((item) => item.favorite)
    }
    if (keyword) {
      next = next.filter((item) => {
        const serialText = (item.serialNo ?? '').toLowerCase()
        const serialMatch = serialText.includes(keyword) || serialText.replace('#', '').includes(keywordSerial)
        const tagMatch = (item.tags ?? []).some((tag) => {
          const normalizedTag = tag.toLowerCase()
          return normalizedTag.includes(keyword.replace('#', '')) || `#${normalizedTag}`.includes(keyword)
        })
        const authorMatch = String(item.author ?? '').toLowerCase().includes(keyword)
        return serialMatch || tagMatch || authorMatch
      })
    }

    const sorted = [...next]
    switch (sortMode) {
      case 'favorites':
        sorted.sort((a, b) => {
          if (b.displayFavorites !== a.displayFavorites) return b.displayFavorites - a.displayFavorites
          return a.recommendedOrder - b.recommendedOrder
        })
        break
      case 'latest-approved':
        sorted.sort((a, b) => {
          const aTime = new Date(a.approvedAt ?? a.createdAt).getTime()
          const bTime = new Date(b.approvedAt ?? b.createdAt).getTime()
          if (bTime !== aTime) return bTime - aTime
          return a.recommendedOrder - b.recommendedOrder
        })
        break
      case 'serial':
        sorted.sort((a, b) => {
          const aSerial = Number((a.serialNo ?? '#0').replace('#', ''))
          const bSerial = Number((b.serialNo ?? '#0').replace('#', ''))
          return aSerial - bSerial
        })
        break
      default:
        sorted.sort((a, b) => a.recommendedOrder - b.recommendedOrder)
        break
    }
    return sorted
  }, [enrichedItems, favoritesOnly, myItems, myUploadsOnly, searchKeyword, serialMap, sortMode])

  const activeDetail = useMemo(
    () => enrichedItems.find((item) => item.id === activeDetailId) ?? null,
    [activeDetailId, enrichedItems],
  )

  const sortOptions: Array<{ value: PromptSortMode; label: string }> = [
    { value: 'recommended', label: '默认推荐' },
    { value: 'favorites', label: '收藏最多' },
    { value: 'latest-approved', label: '最新通过' },
    { value: 'serial', label: '编号顺序' },
  ]

  const sortLabel = sortOptions.find((item) => item.value === sortMode)?.label ?? '默认推荐'
  async function handleCopyPromptText(value: string) {
    const copied = await copyTextToClipboard(value)
    if (copied) {
      onNotify('提示词已复制。')
    } else {
      onNotify('复制失败，请长按提示词手动复制。')
    }
  }

  function handleSubmitShare() {
    if (!submission.title.trim() || !submission.prompt.trim()) {
      onNotify('请先填写提示词标题和正文。')
      return
    }
    void onSubmit()
    setSubmissionImageName('')
    setShowSubmissionModal(false)
  }

  async function uploadSubmissionPreviewImage(file?: File) {
    if (!file) return
    if (!file.type.startsWith('image/')) {
      onNotify('请上传图片文件。')
      return
    }

    try {
      setSubmissionImageUploading(true)
      const result = await api.uploadReferenceImage(file, 'prompt-plaza-preview')
      onSubmissionChange({ ...submission, previewUrl: result.imageUrl })
      setSubmissionImageName(file.name)
      onNotify('缩略图已上传。')
    } catch (error) {
      onNotify(error instanceof Error ? error.message : '图片上传失败。')
    } finally {
      setSubmissionImageUploading(false)
    }
  }

  async function handleSubmissionImageChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    await uploadSubmissionPreviewImage(file)
  }

  function handleSubmissionPromptDrag(event: DragEvent<HTMLTextAreaElement>) {
    if (!Array.from(event.dataTransfer.types).includes('Files')) {
      return
    }
    event.preventDefault()
    setSubmissionDragActive(event.type === 'dragenter' || event.type === 'dragover')
  }

  async function handleSubmissionPromptDrop(event: DragEvent<HTMLTextAreaElement>) {
    if (!Array.from(event.dataTransfer.types).includes('Files')) {
      return
    }
    event.preventDefault()
    setSubmissionDragActive(false)
    const file = Array.from(event.dataTransfer.files).find((item) => item.type.startsWith('image/'))
    await uploadSubmissionPreviewImage(file)
  }

  async function handleSubmissionPromptPaste(event: ClipboardEvent<HTMLTextAreaElement>) {
    const file = Array.from(event.clipboardData.files).find((item) => item.type.startsWith('image/'))
    if (!file) {
      return
    }
    event.preventDefault()
    await uploadSubmissionPreviewImage(file)
  }

  return (
    <div className="prompt-plaza space-y-2.5">
      <div className="space-y-3">
        <div className="grid grid-cols-3 gap-2">
          <button
            type="button"
            onClick={() => setFavoritesOnly((current) => !current)}
            className={`prompt-plaza-action inline-flex h-10 items-center justify-center gap-2 rounded-xl border text-sm font-medium transition ${
              favoritesOnly ? 'is-active' : ''
            }`}
          >
            <Heart className="h-4 w-4" />
            我的收藏
          </button>
          <button
            type="button"
            onClick={() => {
              setMyUploadsOnly((current) => !current)
              setFavoritesOnly(false)
            }}
            className={`prompt-plaza-action inline-flex h-10 items-center justify-center gap-2 rounded-xl border text-sm font-medium transition ${
              myUploadsOnly ? 'is-active' : ''
            }`}
          >
            <Upload className="h-4 w-4" />
            我的上传
          </button>
          <button
            type="button"
            onClick={() => setShowSubmissionModal(true)}
            className="prompt-plaza-action inline-flex h-10 items-center justify-center gap-2 rounded-xl border text-sm font-medium transition"
          >
            <Upload className="h-4 w-4" />
            上传分享
          </button>
        </div>

        <div className="grid grid-cols-[auto_minmax(0,1fr)] gap-2">
          <div className="relative">
            <button
              type="button"
              onClick={() => setSortOpen((current) => !current)}
              className="prompt-plaza-sort inline-flex h-10 items-center gap-2 rounded-xl border px-3 text-sm transition"
            >
              <ArrowUpDown className="h-4 w-4" />
              {sortLabel}
              <ChevronDown className={`h-4 w-4 transition ${sortOpen ? 'rotate-180' : ''}`} />
            </button>
            {sortOpen ? (
              <div className="prompt-plaza-sort-menu absolute left-0 top-11 z-20 min-w-[144px] overflow-hidden rounded-lg border shadow-xl">
                {sortOptions.map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => {
                      setSortMode(option.value)
                      setSortOpen(false)
                    }}
                    className={`block w-full px-3 py-2 text-left text-sm transition ${sortMode === option.value ? 'is-active' : ''}`}
                  >
                    {option.label}
                  </button>
                ))}
              </div>
            ) : null}
          </div>

          <div className="prompt-plaza-search flex h-10 min-w-0 items-center overflow-hidden rounded-xl border text-left">
            <Search className="ml-3 h-4 w-4 text-slate-400" />
            <input
              value={searchKeyword}
              onChange={(event) => setSearchKeyword(event.target.value)}
              className="min-w-0 flex-1 bg-transparent px-2 text-sm outline-none"
              placeholder="搜索编号、标签或上传人"
            />
            {searchKeyword ? (
              <button
                type="button"
                onClick={() => {
                  setSearchKeyword('')
                }}
                className="mr-1 inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-400 transition hover:bg-slate-800 hover:text-slate-200"
                aria-label="清空搜索"
              >
                <XCircle className="h-4 w-4" />
              </button>
            ) : null}
            <span className="prompt-plaza-search-button flex h-full items-center px-3 text-sm font-medium">
              搜索
            </span>
          </div>
        </div>

        <section className="grid grid-cols-2 gap-1.5 sm:gap-2 lg:grid-cols-3 xl:grid-cols-5">
          {visibleItems.map((item, index) => (
            <PromptCard
              key={item.id}
              item={item}
              priority={index < 6}
              onOpenDetail={() => setActiveDetailId(item.id)}
              onFavorite={() => onFavorite(item.id)}
            />
          ))}
        </section>

        {!visibleItems.length ? (
          <div className="rounded-lg border border-dashed border-slate-700 bg-slate-900/60 px-4 py-6 text-center text-sm text-slate-400">
            {favoritesOnly
              ? '当前还没有已收藏的提示词。'
              : myUploadsOnly
                ? '当前还没有上传过提示词。'
              : searchKeyword.trim()
                ? '没有匹配到编号、标签或上传人，试试别的关键词。'
                : '暂时没有可展示的提示词。'}
          </div>
        ) : null}
      </div>

      {showSubmissionModal ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 p-4">
          <button
            type="button"
            className="absolute inset-0"
            aria-label="关闭上传分享"
            onClick={() => setShowSubmissionModal(false)}
          />
          <div className="relative flex max-h-[86dvh] w-full max-w-md flex-col overflow-hidden rounded-xl border border-slate-700 bg-slate-900 shadow-2xl">
            <div className="shrink-0 border-b border-slate-800 px-4 py-3">
              <div className="flex items-center justify-between">
              <h3 className="text-base font-semibold text-slate-100">上传分享</h3>
              <button
                type="button"
                onClick={() => setShowSubmissionModal(false)}
                className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-slate-700 text-slate-300 transition hover:border-slate-500 hover:text-white"
              >
                <XCircle className="h-4 w-4" />
              </button>
              </div>
            </div>
            <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-4">
              <Field label="标题">
                <input
                  value={submission.title}
                  onChange={(event) => onSubmissionChange({ ...submission, title: event.target.value })}
                  className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none focus:border-violet-400"
                  placeholder="给这个灵感起个名字"
                />
              </Field>
              <Field label="提示词">
                <textarea
                  value={submission.prompt}
                  onChange={(event) => onSubmissionChange({ ...submission, prompt: event.target.value })}
                  onDragEnter={handleSubmissionPromptDrag}
                  onDragOver={handleSubmissionPromptDrag}
                  onDragLeave={handleSubmissionPromptDrag}
                  onDrop={(event) => void handleSubmissionPromptDrop(event)}
                  onPaste={(event) => void handleSubmissionPromptPaste(event)}
                  className={`min-h-28 w-full resize-y rounded-md border bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none focus:border-violet-400 ${
                    submissionDragActive ? 'border-violet-400 ring-2 ring-violet-400/20' : 'border-slate-700'
                  }`}
                  placeholder="写下完整提示词，也可以拖入或粘贴本地图片作为缩略图"
                />
              </Field>
              <Field label="标签（可选）">
                <div className="space-y-2">
                  {submission.tags.map((tag, index) => (
                    <div key={index} className="flex gap-2">
                      <input
                        value={tag}
                        onChange={(event) => {
                          const nextTags = [...submission.tags]
                          nextTags[index] = event.target.value
                          onSubmissionChange({ ...submission, tags: nextTags })
                        }}
                        className="min-w-0 flex-1 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none focus:border-violet-400"
                        placeholder={`标签 ${index + 1}`}
                      />
                      {index === 0 ? (
                        <button
                          type="button"
                          onClick={() => {
                            if (submission.tags.length >= 4) return
                            onSubmissionChange({ ...submission, tags: [...submission.tags, ''] })
                          }}
                          disabled={submission.tags.length >= 4}
                          className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-slate-700 text-lg text-slate-200 transition hover:border-violet-400 disabled:cursor-not-allowed disabled:opacity-40"
                          aria-label="增加标签"
                        >
                          +
                        </button>
                      ) : (
                        <button
                          type="button"
                          onClick={() => onSubmissionChange({ ...submission, tags: submission.tags.filter((_, tagIndex) => tagIndex !== index) })}
                          className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-slate-700 text-slate-300 transition hover:border-rose-400 hover:text-rose-200"
                          aria-label="删除标签"
                        >
                          ×
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              </Field>
              <Field label="缩略图链接">
                <input
                  value={submission.previewUrl}
                  onChange={(event) => {
                    setSubmissionImageName('')
                    onSubmissionChange({ ...submission, previewUrl: event.target.value })
                  }}
                  className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none focus:border-violet-400"
                  placeholder="https://..."
                />
              </Field>
              <div className="rounded-lg border border-slate-800 bg-slate-950/80 p-3">
                <div className="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    onClick={() => submissionImageInputRef.current?.click()}
                    disabled={submissionImageUploading}
                    className="inline-flex h-9 items-center justify-center gap-2 rounded-md border border-slate-700 px-3 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {submissionImageUploading ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ImagePlus className="h-4 w-4" />}
                    {submissionImageUploading ? '上传中...' : '选择本地照片'}
                  </button>
                  <span className="text-xs text-slate-500">也可以拖拽或复制粘贴图片到提示词区域。</span>
                </div>
                <input
                  ref={submissionImageInputRef}
                  type="file"
                  accept="image/jpeg,image/png,image/webp,image/gif"
                  className="hidden"
                  onChange={(event) => void handleSubmissionImageChange(event)}
                />
                {submission.previewUrl ? (
                  <div className="mt-3 flex items-center gap-2 rounded-md border border-slate-800 bg-slate-900/70 px-3 py-2 text-sm text-slate-300">
                    <Image className="h-4 w-4 shrink-0 text-slate-500" />
                    <span className="min-w-0 flex-1 truncate">{submissionImageName || '已填写缩略图链接'}</span>
                    <button
                      type="button"
                      onClick={() => {
                        setSubmissionImageName('')
                        onSubmissionChange({ ...submission, previewUrl: '' })
                      }}
                      className="shrink-0 rounded-md px-2 py-1 text-xs text-slate-400 transition hover:bg-slate-800 hover:text-slate-100"
                    >
                      移除
                    </button>
                  </div>
                ) : null}
              </div>
            </div>
            <div className="shrink-0 border-t border-slate-800 p-4">
              <button
                type="button"
                onClick={handleSubmitShare}
                className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-violet-500 px-4 text-sm font-medium text-slate-950 transition hover:bg-violet-400"
              >
                <Send className="h-4 w-4" />
                提交审核
              </button>
            </div>
          </div>
        </div>
      ) : null}

      <PromptDetailDialog
        item={activeDetail}
        onClose={() => setActiveDetailId(null)}
        onFavorite={() => (activeDetail ? onFavorite(activeDetail.id) : undefined)}
        onCopy={() => (activeDetail ? void handleCopyPromptText(activeDetail.prompt) : undefined)}
      />
    </div>
  )
}
export function normalizePromptTags(tags: unknown): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  const source = Array.isArray(tags)
    ? tags
    : typeof tags === 'string'
      ? parsePromptTagString(tags)
      : []
  for (const tag of source) {
    const normalized = String(tag ?? '').trim().replace(/^#+/, '').replace(/\s+/g, '')
    if (!normalized) continue
    const key = normalized.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    result.push(Array.from(normalized).slice(0, 24).join(''))
    if (result.length >= 4) break
  }
  return result
}
