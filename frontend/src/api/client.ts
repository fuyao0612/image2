import type {
  AdminContentAuditItem,
  AdminListContentAuditsResponse,
  AdminListActionLogsResponse,
  AdminListImageTasksResponse,
  AdminListPackagesResponse,
  AdminListRechargeOrdersResponse,
  AdminListUsersResponse,
  AdminPackageItem,
  AdminPackagePayload,
  AdminStatsResponse,
  AdminReviewContentAuditRequest,
  APIEnvelope,
  AuthResponse,
  BillingSettingsResponse,
  ChangePasswordRequest,
  ChangePasswordResponse,
  CheckinResponse,
  CreateImageTaskRequest,
  CreateImageTaskResponse,
  CreateRechargeOrderRequest,
  ConfirmRechargeOrderRequest,
  RechargePaymentCallbackRequest,
  ListRechargeOrdersResponse,
  ListCreditLogsResponse,
  ListPointsLogsResponse,
  ListUserActivityResponse,
  ImageTaskDetailResponse,
  ListImageTasksResponse,
  ListPackagesResponse,
  LoginRequest,
  MockPurchaseRequest,
  MockPurchaseResponse,
  PointsResponse,
  RechargeOrderResponse,
  RegisterRequest,
  SendEmailCodeRequest,
  SendEmailCodeResponse,
  UpdateBillingSettingsRequest,
  UploadImageResponse,
  UserInfo,
} from '@/types/api'

const ENV_API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '').trim().replace(/\/+$/, '')
const API_BASE_URL = ENV_API_BASE_URL
const TOKEN_KEY = 'ai-image-saas-token'

export class APIError extends Error {
  status: number
  code: number

  constructor(message: string, status: number, code: number) {
    super(message)
    this.name = 'APIError'
    this.status = status
    this.code = code
  }
}

export interface DownloadedFile {
  blob: Blob
  fileName: string
}

export function getStoredToken(): string {
  return window.localStorage.getItem(TOKEN_KEY) ?? ''
}

export function saveStoredToken(token: string): void {
  window.localStorage.setItem(TOKEN_KEY, token)
}

export function clearStoredToken(): void {
  window.localStorage.removeItem(TOKEN_KEY)
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  const hasFormDataBody = init.body instanceof FormData
  if (!hasFormDataBody && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const token = getStoredToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers,
  })

  return parseResponse<T>(response)
}

async function requestFormData<T>(path: string, formData: FormData): Promise<T> {
  const headers = new Headers()
  const token = getStoredToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    headers,
    body: formData,
  })

  return parseResponse<T>(response)
}

async function parseResponse<T>(response: Response): Promise<T> {
  let payload: APIEnvelope<T> | undefined
  try {
    payload = (await response.json()) as APIEnvelope<T>
  } catch {
    if (!response.ok) {
      throw new APIError('后端返回了非 JSON 响应', response.status, -1)
    }
    throw new Error('后端返回了非 JSON 响应')
  }

  if (!response.ok || payload.code !== 0) {
    throw new APIError(payload.message || '请求失败', response.status, payload.code)
  }

  return payload.data
}

async function requestBlob(path: string, fallbackFileName: string): Promise<DownloadedFile> {
  const headers = new Headers()
  const token = getStoredToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: 'GET',
    headers,
  })

  if (!response.ok) {
    const message = await parseBlobErrorMessage(response)
    throw new APIError(message, response.status, -1)
  }

  const blob = await response.blob()
  const fileName = parseDownloadFileName(response.headers.get('Content-Disposition')) || fallbackFileName
  return { blob, fileName }
}

async function parseBlobErrorMessage(response: Response): Promise<string> {
  const raw = await response.text()
  if (!raw.trim()) {
    return '图片下载失败'
  }

  try {
    const payload = JSON.parse(raw) as APIEnvelope<unknown>
    return payload.message || '图片下载失败'
  } catch {
    return raw.slice(0, 120)
  }
}

function parseDownloadFileName(contentDisposition: string | null): string {
  if (!contentDisposition) {
    return ''
  }

  const encoded = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)?.[1]
  if (encoded) {
    try {
      return decodeURIComponent(encoded)
    } catch {
      return encoded
    }
  }

  const plain = contentDisposition.match(/filename="?([^";]+)"?/i)?.[1]
  return plain ?? ''
}

export async function sendEmailCode(payload: SendEmailCodeRequest): Promise<SendEmailCodeResponse> {
  return request<SendEmailCodeResponse>('/api/v1/auth/email-code', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function register(payload: RegisterRequest): Promise<AuthResponse> {
  return request<AuthResponse>('/api/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function login(payload: LoginRequest): Promise<AuthResponse> {
  return request<AuthResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function checkin(): Promise<CheckinResponse> {
  return request<CheckinResponse>('/api/v1/user/checkin', {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export async function getPoints(): Promise<PointsResponse> {
  return request<PointsResponse>('/api/v1/user/points')
}

export async function listPointsLogs(page = 1, pageSize = 7): Promise<ListPointsLogsResponse> {
  const params = new URLSearchParams({
    page: String(page),
    pageSize: String(pageSize),
  })
  return request<ListPointsLogsResponse>(`/api/v1/user/points-logs?${params.toString()}`)
}

export async function listCreditLogs(page = 1, pageSize = 7): Promise<ListCreditLogsResponse> {
  const params = new URLSearchParams({
    page: String(page),
    pageSize: String(pageSize),
  })
  return request<ListCreditLogsResponse>(`/api/v1/user/credit-logs?${params.toString()}`)
}

export async function listUserActivity(page = 1, pageSize = 10): Promise<ListUserActivityResponse> {
  const params = new URLSearchParams({
    page: String(page),
    pageSize: String(pageSize),
  })
  return request<ListUserActivityResponse>(`/api/v1/user/logs?${params.toString()}`)
}

export async function getMe(): Promise<UserInfo> {
  return request<UserInfo>('/api/v1/user/me')
}

export async function changePassword(
  payload: ChangePasswordRequest,
): Promise<ChangePasswordResponse> {
  return request<ChangePasswordResponse>('/api/v1/user/password', {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function uploadReferenceImage(file: File): Promise<UploadImageResponse> {
  const formData = new FormData()
  formData.append('file', file)
  return requestFormData<UploadImageResponse>('/api/v1/uploads/reference-image', formData)
}

export async function createImageTask(payload: CreateImageTaskRequest): Promise<CreateImageTaskResponse> {
  return request<CreateImageTaskResponse>('/api/v1/image-tasks', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function getImageTask(taskId: string): Promise<ImageTaskDetailResponse> {
  return request<ImageTaskDetailResponse>(`/api/v1/image-tasks/${encodeURIComponent(taskId)}`)
}

export async function downloadImageTaskImage(taskId: string, index: number): Promise<DownloadedFile> {
  const safeIndex = Math.max(0, Math.floor(index || 0))
  const fallbackFileName = `image2-task-${taskId}-${safeIndex + 1}.png`
  return requestBlob(
    `/api/v1/image-tasks/${encodeURIComponent(taskId)}/images/download?index=${safeIndex}`,
    fallbackFileName,
  )
}

export async function retryImageTask(taskId: string): Promise<CreateImageTaskResponse> {
  return request<CreateImageTaskResponse>(`/api/v1/image-tasks/${encodeURIComponent(taskId)}/retry`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export async function listImageTasks(page = 1, pageSize = 10): Promise<ListImageTasksResponse> {
  const params = new URLSearchParams({
    page: String(page),
    pageSize: String(pageSize),
  })
  return request<ListImageTasksResponse>(`/api/v1/image-tasks?${params.toString()}`)
}

export async function listPackages(packageType?: 'recharge'): Promise<ListPackagesResponse> {
  const params = packageType ? `?type=${encodeURIComponent(packageType)}` : ''
  return request<ListPackagesResponse>(`/api/v1/packages${params}`)
}

export async function getBillingSettings(): Promise<BillingSettingsResponse> {
  return request<BillingSettingsResponse>('/api/v1/billing-settings')
}

export async function createRechargeOrder(payload: CreateRechargeOrderRequest): Promise<RechargeOrderResponse> {
  return request<RechargeOrderResponse>('/api/v1/recharge-orders', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function confirmRechargeOrder(
  orderNo: string,
  payload: ConfirmRechargeOrderRequest = {},
): Promise<RechargeOrderResponse> {
  return request<RechargeOrderResponse>(`/api/v1/recharge-orders/${encodeURIComponent(orderNo)}/confirm`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function getRechargeOrder(orderNo: string): Promise<RechargeOrderResponse> {
  return request<RechargeOrderResponse>(`/api/v1/recharge-orders/${encodeURIComponent(orderNo)}`)
}

export async function listRechargeOrders(page = 1, pageSize = 10): Promise<ListRechargeOrdersResponse> {
  const params = new URLSearchParams({
    page: String(page),
    pageSize: String(pageSize),
  })
  return request<ListRechargeOrdersResponse>(`/api/v1/recharge-orders?${params.toString()}`)
}

export async function rechargePaymentCallback(payload: RechargePaymentCallbackRequest): Promise<RechargeOrderResponse> {
  return request<RechargeOrderResponse>('/api/v1/payments/recharge/callback', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function mockPurchase(payload: MockPurchaseRequest): Promise<MockPurchaseResponse> {
  const order = await createRechargeOrder(payload)
  return confirmRechargeOrder(order.orderNo, {
    providerTradeNo: `mock-${order.orderNo}`,
    paymentChannel: 'mock',
    amountCents: order.amountCents,
    paidAt: new Date().toISOString(),
    remark: '兼容旧版模拟支付',
  })
}

export async function adminStats(): Promise<AdminStatsResponse> {
  return request<AdminStatsResponse>('/api/v1/admin/stats')
}

export async function adminListUsers(page = 1, pageSize = 10, keyword = ''): Promise<AdminListUsersResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (keyword.trim()) {
    params.set('keyword', keyword.trim())
  }
  return request<AdminListUsersResponse>(`/api/v1/admin/users?${params.toString()}`)
}

export async function adminListImageTasks(page = 1, pageSize = 10, status = ''): Promise<AdminListImageTasksResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (status.trim()) {
    params.set('status', status.trim())
  }
  return request<AdminListImageTasksResponse>(`/api/v1/admin/image-tasks?${params.toString()}`)
}

export async function adminListRechargeOrders(
  page = 1,
  pageSize = 10,
  status = '',
): Promise<AdminListRechargeOrdersResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (status.trim()) {
    params.set('status', status.trim())
  }
  return request<AdminListRechargeOrdersResponse>(`/api/v1/admin/recharge-orders?${params.toString()}`)
}

export async function adminListContentAudits(
  page = 1,
  pageSize = 10,
  status = '',
): Promise<AdminListContentAuditsResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (status.trim()) {
    params.set('status', status.trim())
  }
  return request<AdminListContentAuditsResponse>(`/api/v1/admin/content-audits?${params.toString()}`)
}

export async function adminReviewContentAudit(
  auditId: string,
  payload: AdminReviewContentAuditRequest,
): Promise<AdminContentAuditItem> {
  return request<AdminContentAuditItem>(`/api/v1/admin/content-audits/${encodeURIComponent(auditId)}/review`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function adminListActionLogs(
  page = 1,
  pageSize = 10,
  targetType = '',
  targetId = '',
): Promise<AdminListActionLogsResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (targetType) {
    params.set('targetType', targetType)
  }
  if (targetId) {
    params.set('targetId', targetId)
  }
  return request<AdminListActionLogsResponse>(`/api/v1/admin/action-logs?${params.toString()}`)
}

export async function adminListPackages(
  page = 1,
  pageSize = 20,
  packageType = '',
): Promise<AdminListPackagesResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (packageType.trim()) {
    params.set('type', packageType.trim())
  }
  return request<AdminListPackagesResponse>(`/api/v1/admin/packages?${params.toString()}`)
}

export async function adminGetBillingSettings(): Promise<BillingSettingsResponse> {
  return request<BillingSettingsResponse>('/api/v1/admin/billing-settings')
}

export async function adminUpdateBillingSettings(
  payload: UpdateBillingSettingsRequest,
): Promise<BillingSettingsResponse> {
  return request<BillingSettingsResponse>('/api/v1/admin/billing-settings', {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function adminUpdateUserStatus(userId: string, status: number): Promise<unknown> {
  return request<unknown>(`/api/v1/admin/users/${encodeURIComponent(userId)}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  })
}

export async function adminAdjustPoints(userId: string, delta: number, remark: string): Promise<unknown> {
  return request<unknown>(`/api/v1/admin/users/${encodeURIComponent(userId)}/points-adjustments`, {
    method: 'POST',
    body: JSON.stringify({ delta, remark }),
  })
}

export async function adminListUserActivity(
  userId: string,
  page = 1,
  pageSize = 10,
): Promise<ListUserActivityResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  return request<ListUserActivityResponse>(`/api/v1/admin/users/${encodeURIComponent(userId)}/logs?${params.toString()}`)
}

export async function adminCreatePackage(payload: AdminPackagePayload): Promise<AdminPackageItem> {
  return request<AdminPackageItem>('/api/v1/admin/packages', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function adminUpdatePackage(packageId: string, payload: AdminPackagePayload): Promise<AdminPackageItem> {
  return request<AdminPackageItem>(`/api/v1/admin/packages/${encodeURIComponent(packageId)}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function adminDeletePackage(packageId: string): Promise<unknown> {
  return request<unknown>(`/api/v1/admin/packages/${encodeURIComponent(packageId)}`, {
    method: 'DELETE',
  })
}

export const api = {
  API_BASE_URL,
  sendEmailCode,
  register,
  login,
  checkin,
  getMe,
  changePassword,
  getPoints,
  listPointsLogs,
  listCreditLogs,
  listUserActivity,
  uploadReferenceImage,
  createImageTask,
  getImageTask,
  downloadImageTaskImage,
  retryImageTask,
  listImageTasks,
  listPackages,
  getBillingSettings,
  createRechargeOrder,
  confirmRechargeOrder,
  getRechargeOrder,
  listRechargeOrders,
  rechargePaymentCallback,
  mockPurchase,
  adminStats,
  adminListUsers,
  adminListImageTasks,
  adminListRechargeOrders,
  adminListContentAudits,
  adminReviewContentAudit,
  adminListActionLogs,
  adminListPackages,
  adminGetBillingSettings,
  adminUpdateBillingSettings,
  adminUpdateUserStatus,
  adminAdjustPoints,
  adminListUserActivity,
  adminCreatePackage,
  adminDeletePackage,
  adminUpdatePackage,
}
