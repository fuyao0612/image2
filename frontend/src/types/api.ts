export interface APIEnvelope<T> {
  code: number
  message: string
  data: T
}

export interface SendEmailCodeRequest {
  email: string
  scene: 'register' | 'reset_password'
}

export interface SendEmailCodeResponse {
  sent: boolean
  expiresIn: number
  debugCode?: string
}

export interface RegisterRequest {
  email: string
  code: string
  username: string
  password: string
}

export interface LoginRequest {
  account: string
  password: string
}

export interface UserInfo {
  id: string
  username: string
  email: string
  role: string
  pointsBalance: number
  freeCredits: number
  paidCredits: number
  totalCredits: number
  checkinStreak: number
  lastCheckinAt?: string | null
  lastLoginAt?: string | null
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
  freeCredits: number
  paidCredits: number
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
  beforeFree: number
  beforePaid: number
  afterFree: number
  afterPaid: number
  remark?: string
  createdAt: string
}

export interface PointsResponse {
  balance: number
  freeCredits: number
  paidCredits: number
  totalCredits: number
  streak: number
  lastCheckinAt?: string | null
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
  recordType: 'image_task' | 'recharge_order' | 'points_log' | 'credit_log'
  title: string
  description: string
  status?: string
  imageUrl?: string
  deltaPoints?: number
  deltaFreeCredits?: number
  deltaPaidCredits?: number
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
}

export interface CreateImageTaskResponse {
  taskId: string
  status: string
  imageUrl?: string
  imageUrls?: string[]
  balance?: number
  pointsBalance?: number
  freeCreditsBalance?: number
  paidCreditsBalance?: number
  totalCreditsBalance?: number
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
  prompt: string
  imageUrl?: string
  imageUrls?: string[]
  costPoints: number
  costCredits: number
  freeCreditsCost: number
  paidCreditsCost: number
  creditSource: string
  status: string
  requestParams?: string
  errorMessage?: string
  createdAt: string
}

export interface ImageTaskDetailResponse {
  taskId: string
  prompt: string
  imageUrl?: string
  imageUrls?: string[]
  costPoints: number
  costCredits: number
  freeCreditsCost: number
  paidCreditsCost: number
  creditSource: string
  status: string
  requestParams?: string
  errorMessage?: string
  createdAt: string
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
  enabled: boolean
  sortOrder: number
  note?: string
}

export interface ListPackagesResponse {
  items: PackageItem[]
}

export interface CreateRechargeOrderRequest {
  rechargeType: 'custom' | 'package'
  count?: number
  packageCode?: string
}

export interface ConfirmRechargeOrderRequest {
  providerTradeNo?: string
  paymentChannel?: string
  amountCents?: number
  paidAt?: string | null
  remark?: string
}

export interface RechargePaymentCallbackRequest extends ConfirmRechargeOrderRequest {
  orderNo: string
}

export interface RechargeOrderItem {
  id: string
  orderNo: string
  rechargeType: 'custom' | 'package' | string
  packageCode: string
  packageName: string
  amountCents: number
  creditsAmount: number
  bonusPoints: number
  pointsAmount: number
  status: string
  paymentChannel?: string
  providerTradeNo?: string
  paidAt?: string | null
  createdAt: string
}

export interface RechargeOrderResponse extends RechargeOrderItem {
  pointsBalance: number
  freeCreditsBalance: number
  paidCreditsBalance: number
  totalCreditsBalance: number
  balance: number
}

export interface ListRechargeOrdersResponse {
  items: RechargeOrderItem[]
  page: number
  pageSize: number
  total: number
}

export type MockPurchaseRequest = CreateRechargeOrderRequest
export type MockPurchaseResponse = RechargeOrderResponse

export interface BillingSettingsResponse {
  customRechargeUnitPriceCents: number
  imageGeneratePointCost: number
  checkinPoints: number
  checkinCredits: number
}

export interface UpdateBillingSettingsRequest {
  customRechargeUnitPriceCents: number
  imageGeneratePointCost: number
  checkinPoints: number
  checkinCredits: number
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
  freeCreditsBalance: number
  paidCreditsBalance: number
  totalCreditsBalance: number
  status: string
}

export interface AdminStatsResponse {
  usersTotal: number
  usersDisabled: number
  tasksTotal: number
  tasksGenerating: number
  tasksSuccess: number
  tasksFailed: number
  ordersPaid: number
  auditRejected: number
  pointsBalanceTotal: number
  freeCreditsTotal: number
  paidCreditsTotal: number
}

export interface AdminUserItem {
  id: string
  username: string
  email: string
  role: string
  status: number
  pointsBalance: number
  freeCredits: number
  paidCredits: number
  totalCredits: number
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
  prompt: string
  imageUrl?: string
  imageUrls?: string[]
  costPoints: number
  costCredits: number
  freeCreditsCost: number
  paidCreditsCost: number
  creditSource: string
  status: string
  provider?: string
  requestParams?: string
  errorMessage?: string
  createdAt: string
}

export interface AdminListImageTasksResponse {
  items: AdminImageTaskItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminRechargeOrderItem {
  id: string
  userId: string
  orderNo: string
  packageCode: string
  packageName: string
  amountCents: number
  creditsAmount: number
  bonusPoints: number
  pointsAmount: number
  status: string
  paymentChannel?: string
  providerTradeNo?: string
  paidAt?: string | null
  createdAt: string
}

export interface AdminListRechargeOrdersResponse {
  items: AdminRechargeOrderItem[]
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
  action: string
  targetType: string
  targetId: string
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

export interface AdminPackageItem {
  id: string
  code: string
  name: string
  packageType: 'recharge' | 'redeem'
  creditsAmount: number
  pointsCost: number
  amountCents: number
  bonusPoints: number
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
  enabled?: boolean
  sortOrder: number
  note?: string
}
