import React from 'react';
import {
  ArrowUpDown, BadgeCheck, CheckCircle2, ChevronDown, ChevronLeft, ChevronRight,
  Copy, Download, Eye, EyeOff, Gift, Heart, History, Home, Image as ImageIcon,
  ImagePlus, KeyRound, Link2, LoaderCircle, LogIn, LogOut, Mail, Megaphone,
  Menu, Moon, PanelLeftClose, PanelLeftOpen, RefreshCcw, ReceiptText, Send,
  Search, Sparkles, ShoppingBag, Sun, Timer, Upload, UserRound, UserRoundPlus,
  ShieldCheck, Wallet, XCircle
} from 'lucide-react';
import { sizeOptions, CUSTOM_SIZE_MAX_SIDE, styleOptions, qualityOptions, outputFormatOptions, HISTORY_PAGE_SIZE, FREE_REFERENCE_IMAGE_COUNT, MAX_REFERENCE_IMAGES, GENERATION_WAITING_NOTICE } from '../constants/app';
import { getImageTaskStatusLabel, limitImageURLs, normalizeImageURLs, getPreviewRequestedCount, isPartialImageTaskStatus, getTaskStatusBadgeClass, formatTaskCost, getTaskCreditCost, getPreviewElapsedSeconds, formatElapsedTriplet, buildPreviewImageCandidates, createCenteredThumbnailSlots, getPreviewEmptyMessage, highlightSearchText, getGenerationStatusLabel, buildPreviewRequestOptions, parseTaskRequestParams } from '../utils/imageTasks';
import { WechatSaveGuide, ReferenceImagesDialog, PreviewDetailDialog, TaskHistoryPanel, TaskDetailPanel, PreviewImageStage, PromptSearchOverlay, GenerationProgressPanel, GenerationProgressCard, ProgressStat } from '../components/home';

export interface HomePageProps {
  activePage: string;
  points: any;
  billingSettings: any;
  emailNotifyAvailable: boolean;
  emailNotifyOnComplete: boolean;
  setEmailNotifyOnComplete: (val: boolean) => void;
  emailNotifyPointCost: number;
  prompt: string;
  setPrompt: (val: string) => void;
  handlePromptDrag: (e: any) => void;
  handlePromptDragLeave: (e: any) => void;
  handlePromptDrop: (e: any) => void;
  handlePromptPaste: (e: any) => void;
  promptDragActive: boolean;
  referenceImageInputRef: any;
  referenceImageUploading: any;
  busy: any;
  setShowNetworkImageInput: (val: any) => void;
  showNetworkImageInput: boolean;
  handleNetworkReferenceImageInput: (e: any) => void;
  referenceImages: any[];
  handleDeleteReferenceImage: (index: number) => void;
  setReferenceImagesDialogOpen: (val: boolean) => void;
  referenceImagesDialogOpen: boolean;
  getReferenceImageExtraPointCost: (count: number) => number;
  size: string;
  setSize: (val: string) => void;
  customWidth: string;
  setCustomWidth: (val: string) => void;
  customHeight: string;
  setCustomHeight: (val: string) => void;
  style: string;
  setStyle: (val: string) => void;
  count: number;
  setCount: (val: number) => void;
  clampCount: (val: number) => number;
  quality: string;
  setQuality: (val: string) => void;
  outputFormat: string;
  setOutputFormat: (val: string) => void;
  estimatedGenerationPointCost: number;
  handleOpenCreateTaskConfirm: () => void;
  hasRunningGeneration: boolean;
  handleClearPromptOptions: (e?: any) => void;
  preview: any;
  setPreview: (val: any) => void;
  previewImageIndex: number;
  setPreviewImageIndex: (val: number) => void;
  previewDetailOpen: boolean;
  setPreviewDetailOpen: (val: boolean) => void;
  handleSaveCurrentImage: () => void;
  handlePreviousPreviewImage: () => void;
  handleNextPreviewImage: () => void;
  loadPreviewTaskDetail: (taskId: string) => Promise<any>;
  generationProgresses: any[];
  history: any[];
  historyPage: number;
  historyTotal: number;
  loadHistoryPage: (page: number) => void;
  handleSelectTask: (task: any) => void;
  wechatSaveGuide: any;
  setWechatSaveGuide: (val: any) => void;
  generateConfirmOpen: boolean;
  setGenerateConfirmOpen: (val: boolean) => void;
  handleLocalReferenceImageChange: (e: any) => void;
  referenceImageURL: string;
  setReferenceImageURL: (val: string) => void;
  handleAddNetworkReferenceImage: () => void;
  isAuthed: boolean;
  imageGenerationEnabled: boolean;
  handleOpenPreviewDetail: (item: any) => void;
  previewImageURLs: string[];
  previewThumbnailURLs: string[];
  handleCancelCreateTaskConfirm: () => void;
  handleConfirmCreateTask: () => void;
}

export function HomePage(props: HomePageProps) {
  const {
    activePage, points, billingSettings, emailNotifyAvailable, emailNotifyOnComplete,
    setEmailNotifyOnComplete, emailNotifyPointCost, prompt, setPrompt, handlePromptDrag,
    handlePromptDragLeave, handlePromptDrop, handlePromptPaste, promptDragActive,
    referenceImageInputRef,
    referenceImageUploading, busy, setShowNetworkImageInput, showNetworkImageInput,
    handleNetworkReferenceImageInput, referenceImages, handleDeleteReferenceImage,
    setReferenceImagesDialogOpen, referenceImagesDialogOpen,
    getReferenceImageExtraPointCost, size, setSize, customWidth, setCustomWidth,
    customHeight, setCustomHeight, style, setStyle, count, setCount, clampCount,
    quality, setQuality, outputFormat, setOutputFormat, estimatedGenerationPointCost,
    handleOpenCreateTaskConfirm, hasRunningGeneration, handleClearPromptOptions,
    preview, setPreview, previewImageIndex, setPreviewImageIndex, previewDetailOpen,
    setPreviewDetailOpen, handleSaveCurrentImage, handlePreviousPreviewImage,
    handleNextPreviewImage, loadPreviewTaskDetail, generationProgresses, history,
    historyPage, historyTotal, loadHistoryPage, handleSelectTask, wechatSaveGuide,
    setWechatSaveGuide, generateConfirmOpen, setGenerateConfirmOpen,
    handleLocalReferenceImageChange, referenceImageURL, setReferenceImageURL,
    handleAddNetworkReferenceImage, isAuthed, imageGenerationEnabled,
    handleOpenPreviewDetail, previewImageURLs, previewThumbnailURLs,
    handleCancelCreateTaskConfirm, handleConfirmCreateTask
  } = props;

  return (
    <div className={activePage === 'home' ? 'block' : 'hidden'}>
              <div className="space-y-4">
                <div className="grid min-w-0 grid-cols-1 gap-4 xl:grid-cols-[380px_minmax(0,1fr)_360px]">
                  <section className="min-w-0 overflow-hidden rounded-lg border border-slate-800 bg-slate-900/60 p-3 shadow-panel sm:p-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <h2 className="text-lg font-medium text-slate-50">提示词</h2>
                        <p className="mt-1 text-sm text-slate-400">在这里输入提示词，再选择生成参数。</p>
                      </div>
                      <button
                        type="button"
                        onClick={handleClearPromptOptions}
                        className="inline-flex items-center gap-1.5 rounded-md border border-slate-700 bg-slate-950 px-2.5 py-1.5 text-xs text-slate-300 transition hover:border-rose-400 hover:text-rose-200"
                        title="清除当前提示词和生成选项"
                      >
                        <XCircle className="h-3.5 w-3.5" />
                        清除
                      </button>
                    </div>

                    <div className="mt-4 grid grid-cols-2 gap-2 text-sm">
                      <div className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-slate-300">
                        <div className="text-xs text-slate-500">当前积分</div>
                        <div className="mt-1 text-base font-semibold text-slate-100">{points ? points.balance : 0}</div>
                      </div>
                      <div className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-slate-300">
                        <div className="text-xs text-slate-500">可用福利次数</div>
                        <div className="mt-1 text-base font-semibold text-slate-100">
                          {points ? points.totalCredits : 0}
                        </div>
                      </div>
                      <div className="col-span-2 rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-slate-300">
                        <div className="text-sm font-medium text-slate-100">
                          今日生图消耗 {billingSettings?.imageGeneratePointCost ?? 20} 积分
                        </div>
                      </div>
                    </div>

                    {emailNotifyAvailable ? (
                      <label className="mt-3 flex items-start gap-3 rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-sm text-slate-300">
                        <input
                          type="checkbox"
                          checked={emailNotifyOnComplete}
                          onChange={(event) => setEmailNotifyOnComplete(event.target.checked as boolean)}
                          className="mt-1 h-4 w-4 rounded border-slate-600 bg-slate-900 text-violet-500 focus:ring-violet-400"
                        />
                        <span className="min-w-0 leading-6">
                          额外消耗 {emailNotifyPointCost} 积分，任务完成后发送邮件至账号邮箱，失败将退还积分。
                        </span>
                      </label>
                    ) : null}

                    <textarea
                      value={prompt}
                      onChange={(event) => setPrompt(event.target.value)}
                      onDragEnter={handlePromptDrag}
                      onDragOver={handlePromptDrag}
                      onDragLeave={handlePromptDragLeave}
                      onDrop={handlePromptDrop}
                      onPaste={handlePromptPaste}
                      className={`mt-4 min-h-32 w-full resize-y rounded-md border px-3 py-3 text-sm leading-6 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400 ${
                        promptDragActive
                          ? 'border-violet-400 bg-violet-950/40 ring-2 ring-violet-400/40'
                          : 'border-slate-700 bg-slate-900'
                      }`}
                      placeholder={`请输入提示词，或将本地图片拖入 / 粘贴到此框中\n可免费使用 ${FREE_REFERENCE_IMAGE_COUNT} 张参考图，后续每多 1 张额外消耗 1 积分，最多可添加 ${MAX_REFERENCE_IMAGES} 张。`}
                    />

                    <div className="mt-3 grid grid-cols-2 gap-2">
                      <button
                        type="button"
                        onClick={() => referenceImageInputRef.current?.click()}
                        disabled={referenceImageUploading || busy === 'upload-reference'}
                        className="inline-flex items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {referenceImageUploading || busy === 'upload-reference' ? (
                          <LoaderCircle className="h-4 w-4 animate-spin" />
                        ) : (
                          <ImagePlus className="h-4 w-4" />
                        )}
                        添加本地图片
                      </button>
                      <button
                        type="button"
                        onClick={() => setShowNetworkImageInput((current: any) => !current)}
                        className="inline-flex items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white"
                      >
                        <Link2 className="h-4 w-4" />
                        添加网络图片
                      </button>
                    </div>

                    <input
                      ref={referenceImageInputRef}
                      type="file"
                      accept="image/jpeg,image/png,image/webp,image/gif"
                      multiple
                      className="hidden"
                      onChange={handleLocalReferenceImageChange}
                    />

                    {showNetworkImageInput ? (
                      <label className="mt-3 block">
                        <span className="mb-2 block text-sm text-slate-400">网络图片链接</span>
                        <div className="flex overflow-hidden rounded-md border border-slate-700 bg-slate-900 focus-within:border-violet-400">
                          <input
                            value={referenceImageURL}
                            onChange={(event) => handleNetworkReferenceImageInput(event.target.value)}
                            onKeyDown={(event) => {
                              if (event.key === 'Enter') {
                                event.preventDefault()
                                handleAddNetworkReferenceImage()
                              }
                            }}
                            className="h-11 min-w-0 flex-1 bg-transparent px-3 text-sm text-slate-100 outline-none placeholder:text-slate-500"
                            placeholder="请输入公网图片链接"
                          />
                          <button
                            type="button"
                            onClick={handleAddNetworkReferenceImage}
                            className="h-11 shrink-0 border-l border-slate-700 px-4 text-sm font-medium text-violet-200 transition hover:bg-violet-500/10 hover:text-violet-100"
                          >
                            确定
                          </button>
                        </div>
                      </label>
                    ) : null}

                    <div className="mt-3 space-y-2">
                      {referenceImages.length ? (
                        <>
                          <button
                            type="button"
                            onClick={() => setReferenceImagesDialogOpen(true)}
                            className="inline-flex w-full items-center justify-center gap-2 rounded-lg border border-slate-700 bg-slate-900 px-3 py-2.5 text-sm font-medium text-slate-100 transition hover:border-violet-400 hover:bg-slate-800"
                          >
                            查看已添加的 {referenceImages.length} 张图片（最多 {MAX_REFERENCE_IMAGES} 张）
                          </button>
                          <div className="rounded-lg border border-slate-800 bg-slate-950 px-3 py-2 text-xs text-slate-500">
                            参考图额外消耗 {getReferenceImageExtraPointCost(referenceImages.length)} 积分
                          </div>
                        </>
                      ) : (
                        <div className="rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm text-slate-500">未添加参考图片。</div>
                      )}
                    </div>

                    <div className="mt-4 grid grid-cols-2 gap-2">
                      <label className="block">
                        <span className="mb-2 block text-sm text-slate-400">尺寸</span>
                        <select
                          value={size}
                          onChange={(event) => setSize(event.target.value)}
                          className="h-11 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                        >
                          {sizeOptions.map((option: any) => (
                            <option key={option.value} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </select>
                      </label>

                      <label className="block">
                        <span className="mb-2 block text-sm text-slate-400">张数</span>
                        <select
                          value={count}
                          onChange={(event) => setCount(Number(event.target.value))}
                          className="h-11 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                        >
                          {[1, 2, 3, 4, 5, 6].map((value) => (
                            <option key={value} value={value}>
                              {value} 张
                            </option>
                          ))}
                        </select>
                      </label>
                    </div>

                    {size === 'custom' ? (
                      <div className="mt-3 grid grid-cols-2 gap-2">
                        <label className="block">
                          <span className="mb-2 block text-sm text-slate-400">自定义宽度</span>
                          <input
                            type="number"
                            min={1}
                            max={CUSTOM_SIZE_MAX_SIDE}
                            step={64}
                            value={customWidth}
                            onChange={(event) => setCustomWidth(event.target.value)}
                            placeholder="1024"
                            className="h-11 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                          />
                        </label>
                        <label className="block">
                          <span className="mb-2 block text-sm text-slate-400">自定义高度</span>
                          <input
                            type="number"
                            min={1}
                            max={CUSTOM_SIZE_MAX_SIDE}
                            step={64}
                            value={customHeight}
                            onChange={(event) => setCustomHeight(event.target.value)}
                            placeholder="1024"
                            className="h-11 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                          />
                        </label>
                      </div>
                    ) : null}

                    <div className="mt-3 grid grid-cols-3 gap-2">
                      <label className="block min-w-0">
                        <span className="mb-2 block text-sm text-slate-400">风格</span>
                        <select
                          value={style}
                          onChange={(event) => setStyle(event.target.value)}
                          className="h-11 w-full min-w-0 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                        >
                          {styleOptions.map((option: any) => (
                            <option key={option.value || 'none'} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </select>
                      </label>

                      <label className="block min-w-0">
                        <span className="mb-2 block text-sm text-slate-400">质量</span>
                        <select
                          value={quality}
                          onChange={(event) => setQuality(event.target.value)}
                          className="h-11 w-full min-w-0 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                        >
                          {qualityOptions.map((option: any) => (
                            <option key={option.value} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </select>
                      </label>

                      <label className="block min-w-0">
                        <span className="mb-2 block text-sm text-slate-400">输出格式</span>
                        <select
                          value={outputFormat}
                          onChange={(event) => setOutputFormat(event.target.value)}
                          className="h-11 w-full min-w-0 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                        >
                          {outputFormatOptions.map((option: any) => (
                            <option key={option.value} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </select>
                      </label>
                    </div>

                    <div className="mt-3">
                      <button
                        type="button"
                        onClick={handleOpenCreateTaskConfirm}
                        disabled={!isAuthed || busy === 'create-task' || busy === 'upload-reference' || referenceImageUploading || !prompt.trim() || !imageGenerationEnabled}
                        className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-md bg-violet-500 px-4 text-sm font-medium text-slate-950 transition hover:bg-violet-400 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {busy === 'create-task' ? (
                          <LoaderCircle className="h-4 w-4 animate-spin" />
                        ) : referenceImageUploading || busy === 'upload-reference' ? (
                          <LoaderCircle className="h-4 w-4 animate-spin" />
                        ) : (
                          <Sparkles className="h-4 w-4" />
                        )}
                        {imageGenerationEnabled
                          ? referenceImageUploading || busy === 'upload-reference'
                            ? '参考图上传中...'
                            : `生成图片，本次预计消耗 ${estimatedGenerationPointCost} 积分`
                          : '暂时关闭，详情见公告'}
                      </button>
                    </div>
                  </section>

                  <section className="min-w-0 overflow-hidden rounded-lg border border-slate-800 bg-slate-900/60 p-3 shadow-panel sm:p-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <h2 className="text-lg font-medium text-slate-50">展示区</h2>
                        <p className="mt-1 text-sm text-slate-400">长按图片可选择放大预览</p>
                      </div>
                      <button
                        type="button"
                        onClick={() => void handleOpenPreviewDetail(preview)}
                        className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white"
                      >
                        <Eye className="h-4 w-4" />
                        提示词详情
                      </button>
                    </div>

                <PreviewImageStage
                  preview={preview}
                  imageURLs={previewImageURLs}
                  thumbnailURLs={previewThumbnailURLs}
                  activeIndex={previewImageIndex}
                  onPrevious={handlePreviousPreviewImage}
                  onNext={handleNextPreviewImage}
                  onSaveCurrent={handleSaveCurrentImage}
                  onSelectThumbnail={setPreviewImageIndex}
                  onImageLoadError={loadPreviewTaskDetail}
                />

                <TaskDetailPanel
                  preview={preview}
                  progress={generationProgresses.find((item: any) => item.taskId === preview.taskId) ?? null}
                />
              </section>

                  <aside className="min-w-0 space-y-4 overflow-hidden">
                    <TaskHistoryPanel
                      items={history}
                      page={historyPage}
                      pageSize={HISTORY_PAGE_SIZE}
                      total={historyTotal}
                      onSelect={handleSelectTask}
                      onPageChange={(page: any) => void loadHistoryPage(page)}
                    />
                  </aside>
                </div>
              </div>
    </div>
  );
}
