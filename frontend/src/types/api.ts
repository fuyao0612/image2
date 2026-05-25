export interface APIEnvelope<T> {
  code: number
  message: string
  data: T
}

export interface SendEmailCodeRequest {
  email: string
  scene: 'register' | 'reset_password'
  captchaId?: string
  captchaCode?: string
}

export interface SendEmailCodeResponse {
  sent: boolean
  expiresIn: number
  debugCode?: string
}

export interface CaptchaResponse {
  captchaId: string
  image: string
  expiresIn: number
}

export interface PromptPlazaItem {
  id: string
  title: string
  prompt: string
  previewUrl: string
  previewThumbUrl?: string
  tags: string[]
  author: string
  status: 'approved' | 'pending' | 'rejected'
  favorites: number
  isFavorited: boolean
  createdAt: string
  updatedAt: string
  approvedAt?: string
  reviewedAt?: string
  rejectNote?: string
}

export interface ListPromptPlazaResponse {
  items: PromptPlazaItem[]
}

export interface SubmitPromptPlazaRequest {
  title: string
  prompt: string
  previewUrl?: string
  tags?: string[]
}

export interface AdminListPromptPlazaResponse {
  items: PromptPlazaItem[]
  page: number
  pageSize: number
  total: number
  pending: number
}

export interface AdminPromptPlazaPayload {
  title: string
  prompt: string
  previewUrl?: string
  tags?: string[]
  author?: string
  status?: 'approved' | 'pending' | 'rejected'
}

export interface RegisterRequest {
  email: string
  code: string
  username: string
  password: string
  inviteCode?: string
}

export interface LoginRequest {
  account: string
  password: string
  rememberMe?: boolean
}

export interface CheckResetPasswordEmailRequest {
  email: string
}

export interface CheckRegisterAvailabilityRequest {
  email: string
  username: string
}

export interface CheckRegisterAvailabilityResponse {
  emailAvailable: boolean
  usernameAvailable: boolean
  available: boolean
  message?: string
}

export interface CheckResetPasswordEmailResponse {
  available: boolean
  message?: string
  nextResetAt?: string | null
  resetAfterAt?: string | null
}

export interface ResetPasswordRequest {
  email: string
  code: string
  password: string
  confirmPassword: string
}

export interface UserInfo {
  id: string
  username: string
  email: string
  role: string
  pointsBalance: number
  freeCredits?: number
  paidCredits?: number
  creditsBalance?: number
  totalCredits: number
  consumedPoints?: number
  consumedCredits?: number
  checkinStreak: number
  lastCheckinAt?: string | null
  lastLoginAt?: string | null
  inviteCode?: string
}

export interface AuthResponse {
  accessToken: string
  tokenType: string
  expiresIn: number
  user: UserInfo
}

export interface ChangePasswordRequest {
  currentPassword: string
  newPassword: string
  confirmNewPassword: string
}

export interface ChangePasswordResponse {
  changedAt: string
}

export interface CheckinResponse {
  checkedIn: boolean
  gainedPoints: number
  gainedCredits: number
  streak: number
  pointsBalance: number
  freeCredits?: number
  paidCredits?: number
  creditsBalance?: number
  totalCredits: number
  lastCheckinAt: string
}

export interface PointsLogItem {
  id: string
  changeType: string
  delta: number
  beforeBalance: number
  afterBalance: number
  remark?: string
  createdAt: string
}

export interface CreditLogItem {
  id: string
  changeType: string
  deltaFree: number
  deltaPaid: number
  deltaCredits?: number
  beforeFree: number
  beforePaid: number
  beforeCredits?: number
  afterFree: number
  afterPaid: number
  afterCredits?: number
  remark?: string
  createdAt: string
}

export interface PointsResponse {
  balance: number
  freeCredits?: number
  paidCredits?: number
  creditsBalance?: number
  totalCredits: number
  streak: number
  lastCheckinAt?: string | null
  checkinPointsTotal: number
  checkinCreditsTotal: number
  recentLogs: PointsLogItem[]
  recentCreditLogs?: CreditLogItem[]
}

export interface ListPointsLogsResponse {
  items: PointsLogItem[]
  page: number
  pageSize: number
  total: number
}

export interface ListCreditLogsResponse {
  items: CreditLogItem[]
  page: number
  pageSize: number
  total: number
}

export interface UserActivityItem {
  id: string
  recordType: 'image_task' | 'points_log' | 'credit_log' | 'recharge_order'
  title: string
  description: string
  status?: string
  imageUrl?: string
  thumbnailUrl?: string
  deltaPoints?: number
  deltaFreeCredits?: number
  deltaPaidCredits?: number
  deltaCredits?: number
  creditsAmount?: number
  pointsAmount?: number
  amountCents?: number
  createdAt: string
}

export interface ListUserActivityResponse {
  items: UserActivityItem[]
  page: number
  pageSize: number
  total: number
}

export interface CreateImageTaskRequest {
  prompt: string
  size?: string
  style?: string
  count?: number
  quality?: string
  outputFormat?: string
  referenceImageUrl?: string
  referenceImageUrls?: string[]
  emailNotifyOnComplete?: boolean
}

export interface CreateImageTaskResponse {
  taskId: string
  status: string
  imageUrl?: string
  imageUrls?: string[]
  requestedCount: number
  costPoints: number
  costCredits: number
  freeCreditsCost?: number
  paidCreditsCost?: number
  creditSource?: string
  balance?: number
  pointsBalance?: number
  freeCreditsBalance?: number
  paidCreditsBalance?: number
  creditsBalance?: number
  totalCreditsBalance?: number
  emailNotifyEnabled?: boolean
  emailNotifyCostPoints?: number
  errorMessage?: string
}

export interface UploadImageResponse {
  imageUrl: string
  fileName?: string
  contentType?: string
  size?: number
}

export interface ImageTaskItem {
  id: string
  requestedCount: number
  prompt: string
  imageUrl?: string
  imageUrls?: string[]
  imageElapsedSeconds?: number[]
  thumbnailUrl?: string
  thumbnailUrls?: string[]
  costPoints: number
  costCredits: number
  freeCreditsCost?: number
  paidCreditsCost?: number
  creditSource?: string
  emailNotifyEnabled?: boolean
  emailNotifyCostPoints?: number
  status: string
  requestParams?: string
  errorMessage?: string
  createdAt: string
  updatedAt: string
}

export interface ImageTaskDetailResponse {
  taskId: string
  requestedCount: number
  prompt: string
  imageUrl?: string
  imageUrls?: string[]
  imageElapsedSeconds?: number[]
  thumbnailUrl?: string
  thumbnailUrls?: string[]
  provider?: string
  providerTaskId?: string
  providerResponse?: string
  costPoints: number
  costCredits: number
  freeCreditsCost?: number
  paidCreditsCost?: number
  creditSource?: string
  emailNotifyEnabled?: boolean
  emailNotifyCostPoints?: number
  status: string
  requestParams?: string
  errorMessage?: string
  createdAt: string
  updatedAt: string
}

export interface ListImageTasksResponse {
  items: ImageTaskItem[]
  page: number
  pageSize: number
  total: number
}

export interface PackageItem {
  code: string
  name: string
  packageType: 'recharge' | 'redeem'
  creditsAmount: number
  pointsCost: number
  amountCents: number
  bonusPoints: number
  productUrl?: string
  enabled: boolean
  sortOrder: number
  note?: string
}

export interface ListPackagesResponse {
  items: PackageItem[]
}

export interface ProductCategoryItem {
  id: string
  name: string
  description?: string
  sortOrder: number
  enabled: boolean
}

export interface ProductItem {
  id: string
  name: string
  shortDescription?: string
  description?: string
  previewUrl?: string
  previewThumbUrl?: string
  amountCents: number
  productUrl?: string
  sortOrder: number
  enabled: boolean
  categories: ProductCategoryItem[]
  createdAt: string
  updatedAt: string
}

export interface ListProductsResponse {
  items: ProductItem[]
}

export interface ListProductCategoriesResponse {
  items: ProductCategoryItem[]
}

export interface RedeemCodeRequest {
  code: string
}

export interface RedeemCodeResponse {
  code: string
  pointsAmount: number
  creditsAmount: number
  pointsBalance: number
  freeCreditsBalance?: number
  paidCreditsBalance?: number
  creditsBalance?: number
  totalCreditsBalance: number
  redeemedAt: string
}

export interface BillingSettingsResponse {
  imageGeneratePointCost: number
  imageGenerationEnabled: boolean
  imageFailureNotifyEnabled: boolean
  emailNotifyEnabled: boolean
  emailNotifyPointCost: number
  checkinPoints: number
  checkinCredits: number
  inviteRewardPoints: number
  inviteRewardCredits: number
  registrationInviteBonusPoints: number
  registrationGiftCredits: number
}

export interface UpdateBillingSettingsRequest {
  imageGeneratePointCost: number
  imageGenerationEnabled: boolean
  imageFailureNotifyEnabled: boolean
  emailNotifyEnabled: boolean
  emailNotifyPointCost: number
  checkinPoints: number
  checkinCredits: number
  inviteRewardPoints: number
  inviteRewardCredits: number
  registrationInviteBonusPoints: number
  registrationGiftCredits: number
}

export interface InviteInfoResponse {
  inviteCode: string
  invitedCount: number
  rewardPointsTotal: number
  rewardCreditsTotal: number
  inviteRewardPoints: number
  inviteRewardCredits: number
  registrationInviteBonusPoints: number
  registrationGiftCredits: number
}

export interface RedeemPointsRequest {
  packageCode: string
}

export interface RedeemPointsResponse {
  redemptionNo: string
  packageCode: string
  packageName: string
  pointsCost: number
  creditsAmount: number
  pointsBalance: number
  freeCreditsBalance?: number
  paidCreditsBalance?: number
  creditsBalance?: number
  totalCreditsBalance: number
  status: string
}

export interface AdminStatsResponse {
  usersTotal: number
  usersDisabled: number
  usersCreatedToday?: number
  tasksActiveUsersToday?: number
  ordersPaid?: number
  tasksTotal: number
  tasksGenerating: number
  tasksSuccess: number
  tasksFailed: number
  tasksPointsConsumedTotal?: number
  tasksCreditsConsumedTotal?: number
  tasksImagesGeneratedTotal?: number
  tasksPointsConsumedToday?: number
  tasksCreditsConsumedToday?: number
  tasksImagesGeneratedToday?: number
  tasksPointsConsumedYesterday?: number
  tasksCreditsConsumedYesterday?: number
  tasksImagesGeneratedYesterday?: number
  auditRejected: number
  pointsBalanceTotal: number
  pointsConvertedCreditsTotal?: number
  totalCredits?: number
  creditsTotal?: number
  creditsBalanceTotal?: number
  freeCreditsTotal?: number
  paidCreditsTotal?: number
  resourceCreditsTotal?: number
  pointToCreditRate?: number
  redeemCodesActive?: number
  redeemCodesUsed?: number
  redeemCodesDisabled?: number
  redeemCodeCreditsTotal?: number
  redeemCodePointsTotal?: number
  redeemCodeActivePointsTotal?: number
  redeemCodeUsedPointsTotal?: number
  redeemCodeConvertedCreditsTotal?: number
  taskTrend24h?: AdminTaskTrendPoint[]
  taskTrend3d?: AdminTaskTrendPoint[]
  taskTrend7d?: AdminTaskTrendPoint[]
}

export interface AdminTaskTrendPoint {
  label: string
  success: number
  failed: number
}

export interface AdminUserItem {
  id: string
  username: string
  email: string
  role: string
  status: number
  pointsBalance: number
  freeCredits?: number
  paidCredits?: number
  creditsBalance?: number
  totalCredits: number
  consumedPoints?: number
  consumedCredits?: number
  checkinStreak: number
  lastLoginAt?: string | null
  lastCheckinAt?: string | null
  createdAt: string
}

export interface AdminListUsersResponse {
  items: AdminUserItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminImageTaskItem {
  id: string
  userId: string
  username?: string
  prompt: string
  imageUrl?: string
  imageUrls?: string[]
  thumbnailUrl?: string
  thumbnailUrls?: string[]
  costPoints: number
  costCredits: number
  freeCreditsCost?: number
  paidCreditsCost?: number
  emailNotifyEnabled?: boolean
  emailNotifyCostPoints?: number
  creditSource?: string
  status: string
  provider?: string
  providerKeyId?: string
  providerKeyName?: string
  providerTaskId?: string
  providerResponse?: string
  requestParams?: string
  errorMessage?: string
  adminErrorMessage?: string
  createdAt: string
  updatedAt: string
}

export interface AdminListImageTasksResponse {
  items: AdminImageTaskItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminRedeemCodeItem {
  id: string
  code: string
  pointsAmount: number
  creditsAmount: number
  status: string
  batchNo?: string
  note?: string
  createdBy: string
  usedBy?: string
  usedAt?: string | null
  createdAt: string
}

export interface AdminCreateRedeemCodesRequest {
  count: number
  pointsAmount: number
  creditsAmount: number
  prefix?: string
  note?: string
}

export interface AdminCreateRedeemCodesResponse {
  batchNo: string
  items: AdminRedeemCodeItem[]
}

export interface AdminListRedeemCodesResponse {
  items: AdminRedeemCodeItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminContentAuditItem {
  id: string
  userId: string
  bizType: string
  bizId?: string
  contentType: string
  content: string
  status: string
  matchedKeywords?: string[]
  reason?: string
  action: string
  reviewerUserId?: string
  reviewedAt?: string | null
  createdAt: string
}

export interface AdminListContentAuditsResponse {
  items: AdminContentAuditItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminReviewContentAuditRequest {
  status: 'pass' | 'rejected'
  action?: 'allow' | 'reject'
  reason?: string
}

export interface AdminActionLogItem {
  id: string
  adminId: string
  userId?: string
  actorLabel?: string
  action: string
  recordType?: string
  targetType: string
  targetId: string
  title?: string
  description?: string
  status?: string
  beforeJson?: string
  afterJson?: string
  remark?: string
  createdAt: string
}

export interface AdminListActionLogsResponse {
  items: AdminActionLogItem[]
  page: number
  pageSize: number
  total: number
}

export interface DataCleanupRequest {
  type:
    | 'failed_image_tasks'
    | 'generated_images'
    | 'user_reference_images'
    | 'temporary_keys'
    | 'action_logs'
    | 'email_failure_notifications'
    | 'successful_image_tasks'
    | 'balance_logs'
    | 'user_activity'
    | 'announcement_reads'
    | 'rejected_prompt_plaza'
  mode?: 'retention' | 'date' | 'days_ago'
  keepDays?: number
  startDate?: string
  endDate?: string
  daysAgo?: number
}

export interface DataCleanupTableStat {
  table: string
  count: number
  bytes?: number
  note?: string
}

export interface DataCleanupResponse {
  mode: 'retention' | 'date' | 'days_ago'
  type: DataCleanupRequest['type']
  typeLabel: string
  targetDate: string
  keepDays: number
  startedAt: string
  finishedAt?: string | null
  dryRun: boolean
  tableStats: DataCleanupTableStat[]
  filesRemoved: number
  bytesRemoved: number
  recordsRemoved: number
}

export interface AnnouncementItem {
  id: string
  title: string
  content: string
  imageUrl?: string
  kind: 'pinned' | 'normal' | 'personal'
  targetUserId?: string
  enabled: boolean
  sortOrder: number
  isRead: boolean
  readAt?: string | null
  createdAt: string
  updatedAt: string
}

export interface ListAnnouncementsResponse {
  items: AnnouncementItem[]
}

export interface MarkAnnouncementReadResponse {
  id: string
  isRead: boolean
  readAt: string
}

export interface AdminListAnnouncementsResponse {
  items: AnnouncementItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminAnnouncementPayload {
  title: string
  content: string
  imageUrl?: string
  kind: 'pinned' | 'normal' | 'personal'
  targetUserId?: string
  enabled?: boolean
  sortOrder: number
}

export interface AdminPackageItem {
  id: string
  code: string
  name: string
  packageType: 'recharge' | 'redeem'
  creditsAmount: number
  pointsCost: number
  amountCents: number
  bonusPoints: number
  productUrl?: string
  enabled: boolean
  sortOrder: number
  note?: string
  createdAt: string
  updatedAt: string
}

export interface AdminListPackagesResponse {
  items: AdminPackageItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminPackagePayload {
  code?: string
  name: string
  packageType?: 'recharge' | 'redeem'
  creditsAmount: number
  pointsCost: number
  amountCents: number
  bonusPoints: number
  productUrl?: string
  enabled?: boolean
  sortOrder: number
  note?: string
}

export interface AdminProductPayload {
  name: string
  shortDescription?: string
  description?: string
  previewUrl?: string
  amountCents: number
  productUrl?: string
  categoryIds: string[]
  sortOrder: number
  enabled?: boolean
}

export interface AdminProductCategoryPayload {
  name: string
  description?: string
  sortOrder: number
  enabled?: boolean
}

export interface AdminListProductsResponse {
  items: ProductItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminListProductCategoriesResponse {
  items: ProductCategoryItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminProviderKeyItem {
  id: string
  name: string
  provider: string
  batchServiceUrl: string
  apiBaseUrl: string
  apiKeyMasked: string
  status: string
  isUsing: boolean
  priority: number
  autoSwitchEnabled: boolean
  callCount: number
  successCount: number
  failureCount: number
  lastUsedAt?: string | null
  lastError?: string
  createdAt: string
  updatedAt: string
}

export interface AdminListProviderKeysResponse {
  items: AdminProviderKeyItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminProviderKeyPayload {
  name: string
  provider?: string
  batchServiceUrl?: string
  apiBaseUrl?: string
  apiKey?: string
  status?: 'active' | 'disabled' | 'error' | 'low_balance'
  priority: number
  autoSwitchEnabled?: boolean
}
