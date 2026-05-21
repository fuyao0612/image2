import type { PointsLogItem, CreditLogItem } from './api'

export type AuthMode = 'login' | 'register' | 'reset'
export type AppPage = 'home' | 'orders' | 'profile' | 'flows' | 'recharge' | 'prompts' | 'products'
export type UserTheme = 'light' | 'dark'
export type FlowLogType = 'points' | 'credits'
export type FlowLogDetail =
  | { type: 'points'; item: PointsLogItem }
  | { type: 'credits'; item: CreditLogItem }
  | null
export type BusyAction =
  | 'send-code'
  | 'check-register-availability'
  | 'check-reset-email'
  | 'auth'
  | 'checkin'
  | 'upload-reference'
  | 'create-task'
  | 'refresh'
  | `retry:${string}`
  | `purchase:${string}`
  | 'copy-invite-link'
  | 'redeem-code'
  | 'change-password'
  | null

export type ReferenceImageSource = 'none' | 'local' | 'network'

export interface ReferenceImageItem {
  url: string
  name: string
  source: Exclude<ReferenceImageSource, 'none'>
}

export type GenerationStatus = 'idle' | 'running' | 'success' | 'failed'

export interface GenerationProgress {
  taskId: string | null
  prompt: string
  status: GenerationStatus
  startedAt: number | null
  elapsedSeconds: number
  imageElapsedSeconds: number[]
  total: number
  completed: number
  success: number
  running: number
  attempt: number
  backendUrl: string
  message: string
}

export interface TaskRequestSnapshot {
  prompt?: string
  size?: string
  style?: string
  count?: number
  quality?: string
  outputFormat?: string
  referenceImageUrl?: string
  referenceImageUrls?: string[]
  referenceImages?: ReferenceImageItem[]
  emailNotifyOnComplete?: boolean
}

export interface PromptGalleryItem {
  id: string
  title: string
  prompt: string
  previewUrl: string
  previewThumbUrl?: string
  tags: string[]
  author: string
  status: 'approved' | 'pending' | 'rejected'
  favorites: number
  createdAt: string
  approvedAt?: string
  serialNo?: string
  isFavorited?: boolean
}

export type PromptSortMode = 'recommended' | 'favorites' | 'latest-approved' | 'serial'

export interface PasswordChangeForm {
  currentPassword: string
  newPassword: string
  confirmNewPassword: string
}

export interface WechatSaveGuideState {
  open: boolean
  imageUrl: string
  title: string
}

export type RechargeConfirmDialogState =
  | {
      open: false
    }
  | {
      open: true
      kind: 'package'
      title: string
      lines: string[]
      confirmLabel: string
      href?: string
    }
