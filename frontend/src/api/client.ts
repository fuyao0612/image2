import type {
  AdminContentAuditItem,
  AdminListContentAuditsResponse,
  AdminListActionLogsResponse,
  AdminAnnouncementPayload,
  AdminListAnnouncementsResponse,
  AdminListImageTasksResponse,
  AdminListPackagesResponse,
  AdminListProductCategoriesResponse,
  AdminListProductsResponse,
  AdminListProviderKeysResponse,
  AdminCreateRedeemCodesRequest,
  AdminCreateRedeemCodesResponse,
  AdminListRedeemCodesResponse,
  AdminListUsersResponse,
  AdminPackageItem,
  AdminPackagePayload,
  AdminProductCategoryPayload,
  AdminProductPayload,
  AdminProviderKeyPayload,
  AdminStatsResponse,
  AdminReviewContentAuditRequest,
  DataCleanupRequest,
  DataCleanupResponse,
  APIEnvelope,
  AuthResponse,
  CaptchaResponse,
  AnnouncementItem,
  BillingSettingsResponse,
  ChangePasswordRequest,
  ChangePasswordResponse,
  CheckRegisterAvailabilityRequest,
  CheckRegisterAvailabilityResponse,
  CheckResetPasswordEmailRequest,
  CheckResetPasswordEmailResponse,
  CheckinResponse,
  CreateImageTaskRequest,
  CreateImageTaskResponse,
  RedeemCodeRequest,
  RedeemCodeResponse,
  ListAnnouncementsResponse,
  ListCreditLogsResponse,
  ListPointsLogsResponse,
  ListUserActivityResponse,
  ImageTaskDetailResponse,
  InviteInfoResponse,
  ListImageTasksResponse,
  ListPackagesResponse,
  ListProductCategoriesResponse,
  ListProductsResponse,
  LoginRequest,
  MarkAnnouncementReadResponse,
  PointsResponse,
  AdminListPromptPlazaResponse,
  AdminPromptPlazaPayload,
  ListPromptPlazaResponse,
  PromptPlazaItem,
  ProductCategoryItem,
  ProductItem,
  RegisterRequest,
  ResetPasswordRequest,
  SubmitPromptPlazaRequest,
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

  let response: Response
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      ...init,
      headers,
    })
  } catch (error) {
    throw new APIError(buildNetworkErrorMessage(error), 0, -1)
  }

  return parseResponse<T>(response)
}

async function requestFormData<T>(path: string, formData: FormData): Promise<T> {
  const headers = new Headers()
  const token = getStoredToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  let response: Response
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method: 'POST',
      headers,
      body: formData,
    })
  } catch (error) {
    throw new APIError(buildNetworkErrorMessage(error), 0, -1)
  }

  return parseResponse<T>(response)
}

async function parseResponse<T>(response: Response): Promise<T> {
  const raw = await response.text()
  const trimmed = raw.trim()
  if (!trimmed) {
    throw new APIError(`后端返回了空响应（HTTP ${response.status}）`, response.status, -1)
  }

  let payload: APIEnvelope<T>
  try {
    payload = JSON.parse(trimmed) as APIEnvelope<T>
  } catch {
    throw new APIError(buildNonJSONErrorMessage(response, trimmed), response.status, -1)
  }

  if (!response.ok || payload.code !== 0) {
    throw new APIError(payload.message || '请求失败', response.status, payload.code)
  }

  return payload.data
}

function buildNetworkErrorMessage(error: unknown): string {
  const detail = error instanceof Error ? error.message : ''
  if (!API_BASE_URL) {
    return `无法连接本地后端。请确认 backend 已在 127.0.0.1:8080 启动，Vite 代理才可以转发 /api 请求。${detail ? `（${detail}）` : ''}`
  }
  return `无法连接后端接口 ${API_BASE_URL}。${detail ? `（${detail}）` : ''}`
}

function buildNonJSONErrorMessage(response: Response, body: string): string {
  const contentType = response.headers.get('content-type') || 'unknown'
  const preview = body.replace(/\s+/g, ' ').slice(0, 160)
  const isHTML = /^<!doctype html/i.test(body) || /^<html/i.test(body)
  const isViteProxyError = body.includes('http proxy error') || body.includes('ECONNREFUSED')

  if (!API_BASE_URL && (isViteProxyError || response.status >= 500)) {
    return `本地后端没有正常响应。请先启动 backend（监听 127.0.0.1:8080），再刷新 5173 页面重试。HTTP ${response.status}，Content-Type: ${contentType}。`
  }
  if (isHTML) {
    return `接口返回了 HTML 页面，不是 JSON。请检查请求是否被转到了前端页面或 Nginx 错误页。HTTP ${response.status}，Content-Type: ${contentType}。`
  }
  return `后端返回了非 JSON 响应。HTTP ${response.status}，Content-Type: ${contentType}，响应片段：${preview || '空'}`
}

async function requestBlob(path: string, fallbackFileName: string): Promise<DownloadedFile> {
  const headers = new Headers()
  const token = getStoredToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  let response: Response
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method: 'GET',
      headers,
    })
  } catch (error) {
    throw new APIError(buildNetworkErrorMessage(error), 0, -1)
  }

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

export async function getCaptcha(): Promise<CaptchaResponse> {
  return request<CaptchaResponse>('/api/v1/auth/captcha')
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

export async function checkResetPasswordEmail(
  payload: CheckResetPasswordEmailRequest,
): Promise<CheckResetPasswordEmailResponse> {
  return request<CheckResetPasswordEmailResponse>('/api/v1/auth/reset-password/check', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function checkRegisterAvailability(
  payload: CheckRegisterAvailabilityRequest,
): Promise<CheckRegisterAvailabilityResponse> {
  return request<CheckRegisterAvailabilityResponse>('/api/v1/auth/register/check', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function resetPassword(payload: ResetPasswordRequest): Promise<ChangePasswordResponse> {
  return request<ChangePasswordResponse>('/api/v1/auth/reset-password', {
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

export async function getInviteInfo(): Promise<InviteInfoResponse> {
  return request<InviteInfoResponse>('/api/v1/user/invite')
}

export async function listAnnouncements(): Promise<ListAnnouncementsResponse> {
  return request<ListAnnouncementsResponse>('/api/v1/user/announcements')
}

export async function markAnnouncementRead(announcementId: string): Promise<MarkAnnouncementReadResponse> {
  return request<MarkAnnouncementReadResponse>(`/api/v1/user/announcements/${encodeURIComponent(announcementId)}/read`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export async function changePassword(
  payload: ChangePasswordRequest,
): Promise<ChangePasswordResponse> {
  return request<ChangePasswordResponse>('/api/v1/user/password', {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function uploadReferenceImage(file: File, purpose = 'reference'): Promise<UploadImageResponse> {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('purpose', purpose)
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

export async function listPromptPlaza(): Promise<ListPromptPlazaResponse> {
  return request<ListPromptPlazaResponse>('/api/v1/prompt-plaza')
}

export async function listProducts(keyword = '', categoryId = ''): Promise<ListProductsResponse> {
  const params = new URLSearchParams()
  if (keyword.trim()) params.set('keyword', keyword.trim())
  if (categoryId.trim()) params.set('categoryId', categoryId.trim())
  const query = params.toString()
  return request<ListProductsResponse>(`/api/v1/products${query ? `?${query}` : ''}`)
}

export async function listProductCategories(): Promise<ListProductCategoriesResponse> {
  return request<ListProductCategoriesResponse>('/api/v1/product-categories')
}

export async function listPromptPlazaForUser(): Promise<ListPromptPlazaResponse> {
  return request<ListPromptPlazaResponse>('/api/v1/user/prompt-plaza-gallery')
}

export async function listMyPromptPlaza(): Promise<ListPromptPlazaResponse> {
  return request<ListPromptPlazaResponse>('/api/v1/user/prompt-plaza')
}

export async function submitPromptPlaza(payload: SubmitPromptPlazaRequest): Promise<PromptPlazaItem> {
  return request<PromptPlazaItem>('/api/v1/user/prompt-plaza', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function togglePromptPlazaFavorite(id: string): Promise<PromptPlazaItem> {
  return request<PromptPlazaItem>(`/api/v1/user/prompt-plaza/${encodeURIComponent(id)}/favorite`, {
    method: 'POST',
  })
}

export async function redeemCode(payload: RedeemCodeRequest): Promise<RedeemCodeResponse> {
  return request<RedeemCodeResponse>('/api/v1/redeem-code', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function adminStats(): Promise<AdminStatsResponse> {
  return request<AdminStatsResponse>('/api/v1/admin/stats')
}

export async function adminListUsers(page = 1, pageSize = 10, keyword = '', status = ''): Promise<AdminListUsersResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (keyword.trim()) {
    params.set('keyword', keyword.trim())
  }
  if (status.trim()) {
    params.set('status', status.trim())
  }
  return request<AdminListUsersResponse>(`/api/v1/admin/users?${params.toString()}`)
}

export async function adminListImageTasks(page = 1, pageSize = 10, status = '', keyword = ''): Promise<AdminListImageTasksResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (status.trim()) {
    params.set('status', status.trim())
  }
  if (keyword.trim()) {
    params.set('keyword', keyword.trim())
  }
  return request<AdminListImageTasksResponse>(`/api/v1/admin/image-tasks?${params.toString()}`)
}

export async function adminListRedeemCodes(
  page = 1,
  pageSize = 10,
  status = '',
  keyword = '',
): Promise<AdminListRedeemCodesResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (status.trim()) {
    params.set('status', status.trim())
  }
  if (keyword.trim()) {
    params.set('keyword', keyword.trim())
  }
  return request<AdminListRedeemCodesResponse>(`/api/v1/admin/redeem-codes?${params.toString()}`)
}

export async function adminCreateRedeemCodes(
  payload: AdminCreateRedeemCodesRequest,
): Promise<AdminCreateRedeemCodesResponse> {
  return request<AdminCreateRedeemCodesResponse>('/api/v1/admin/redeem-codes', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function adminDeleteRedeemCode(codeId: string): Promise<unknown> {
  return request<unknown>(`/api/v1/admin/redeem-codes/${encodeURIComponent(codeId)}`, {
    method: 'DELETE',
  })
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
  action = '',
): Promise<AdminListActionLogsResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (targetType) {
    params.set('targetType', targetType)
  }
  if (targetId) {
    params.set('targetId', targetId)
  }
  if (action) {
    params.set('action', action)
  }
  return request<AdminListActionLogsResponse>(`/api/v1/admin/action-logs?${params.toString()}`)
}

export async function adminPreviewDataCleanup(payload: DataCleanupRequest): Promise<DataCleanupResponse> {
  return request<DataCleanupResponse>('/api/v1/admin/data-cleanup/preview', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function adminExecuteDataCleanup(payload: DataCleanupRequest): Promise<DataCleanupResponse> {
  return request<DataCleanupResponse>('/api/v1/admin/data-cleanup', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function adminListAnnouncements(page = 1, pageSize = 10): Promise<AdminListAnnouncementsResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  return request<AdminListAnnouncementsResponse>(`/api/v1/admin/announcements?${params.toString()}`)
}

export async function adminCreateAnnouncement(payload: AdminAnnouncementPayload): Promise<AnnouncementItem> {
  return request<AnnouncementItem>('/api/v1/admin/announcements', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function adminUploadAnnouncementImage(file: File, purpose = 'announcement'): Promise<UploadImageResponse> {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('purpose', purpose)
  return requestFormData<UploadImageResponse>('/api/v1/admin/announcements/upload-image', formData)
}

export async function adminUploadPromptPlazaImage(file: File): Promise<UploadImageResponse> {
  return adminUploadAnnouncementImage(file, 'prompt-plaza-preview')
}

export async function adminUploadProductImage(file: File): Promise<UploadImageResponse> {
  return adminUploadAnnouncementImage(file, 'product-image')
}

export async function adminUpdateAnnouncement(
  announcementId: string,
  payload: AdminAnnouncementPayload,
): Promise<AnnouncementItem> {
  return request<AnnouncementItem>(`/api/v1/admin/announcements/${encodeURIComponent(announcementId)}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function adminDeleteAnnouncement(announcementId: string): Promise<unknown> {
  return request<unknown>(`/api/v1/admin/announcements/${encodeURIComponent(announcementId)}`, {
    method: 'DELETE',
  })
}

export async function adminListPromptPlaza(page = 1, pageSize = 50, status = ''): Promise<AdminListPromptPlazaResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (status.trim()) params.set('status', status.trim())
  return request<AdminListPromptPlazaResponse>(`/api/v1/admin/prompt-plaza?${params.toString()}`)
}

export async function adminCreatePromptPlaza(payload: AdminPromptPlazaPayload): Promise<PromptPlazaItem> {
  return request<PromptPlazaItem>('/api/v1/admin/prompt-plaza', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function adminUpdatePromptPlaza(id: string, payload: AdminPromptPlazaPayload): Promise<PromptPlazaItem> {
  return request<PromptPlazaItem>(`/api/v1/admin/prompt-plaza/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function adminReviewPromptPlaza(id: string, status: 'approved' | 'rejected', rejectNote = ''): Promise<PromptPlazaItem> {
  return request<PromptPlazaItem>(`/api/v1/admin/prompt-plaza/${encodeURIComponent(id)}/review`, {
    method: 'PATCH',
    body: JSON.stringify({ status, rejectNote }),
  })
}

export async function adminDeletePromptPlaza(id: string, reason = ''): Promise<unknown> {
  return request<unknown>(`/api/v1/admin/prompt-plaza/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    body: JSON.stringify({ reason }),
  })
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

export async function adminListProducts(
  page = 1,
  pageSize = 20,
  keyword = '',
  categoryId = '',
  enabled = '',
): Promise<AdminListProductsResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  if (keyword.trim()) params.set('keyword', keyword.trim())
  if (categoryId.trim()) params.set('categoryId', categoryId.trim())
  if (enabled.trim()) params.set('enabled', enabled.trim())
  return request<AdminListProductsResponse>(`/api/v1/admin/products?${params.toString()}`)
}

export async function adminCreateProduct(payload: AdminProductPayload): Promise<ProductItem> {
  return request<ProductItem>('/api/v1/admin/products', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function adminUpdateProduct(id: string, payload: AdminProductPayload): Promise<ProductItem> {
  return request<ProductItem>(`/api/v1/admin/products/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function adminDeleteProduct(id: string): Promise<{ deleted: boolean }> {
  return request<{ deleted: boolean }>(`/api/v1/admin/products/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export async function adminListProductCategories(page = 1, pageSize = 50): Promise<AdminListProductCategoriesResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  return request<AdminListProductCategoriesResponse>(`/api/v1/admin/product-categories?${params.toString()}`)
}

export async function adminCreateProductCategory(payload: AdminProductCategoryPayload): Promise<ProductCategoryItem> {
  return request<ProductCategoryItem>('/api/v1/admin/product-categories', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function adminUpdateProductCategory(
  id: string,
  payload: AdminProductCategoryPayload,
): Promise<ProductCategoryItem> {
  return request<ProductCategoryItem>(`/api/v1/admin/product-categories/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function adminDeleteProductCategory(id: string): Promise<{ deleted: boolean }> {
  return request<{ deleted: boolean }>(`/api/v1/admin/product-categories/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
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

export async function adminListProviderKeys(page = 1, pageSize = 20): Promise<AdminListProviderKeysResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  return request<AdminListProviderKeysResponse>(`/api/v1/admin/provider-keys?${params.toString()}`)
}

export async function adminCreateProviderKey(payload: AdminProviderKeyPayload): Promise<unknown> {
  return request<unknown>('/api/v1/admin/provider-keys', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function adminUpdateProviderKey(providerKeyId: string, payload: AdminProviderKeyPayload): Promise<unknown> {
  return request<unknown>(`/api/v1/admin/provider-keys/${encodeURIComponent(providerKeyId)}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function adminUseProviderKey(providerKeyId: string): Promise<unknown> {
  return request<unknown>(`/api/v1/admin/provider-keys/${encodeURIComponent(providerKeyId)}/use`, {
    method: 'POST',
  })
}

export async function adminDeleteProviderKey(providerKeyId: string): Promise<unknown> {
  return request<unknown>(`/api/v1/admin/provider-keys/${encodeURIComponent(providerKeyId)}`, {
    method: 'DELETE',
  })
}

export const api = {
  API_BASE_URL,
  getCaptcha,
  sendEmailCode,
  register,
  login,
  checkRegisterAvailability,
  checkResetPasswordEmail,
  resetPassword,
  checkin,
  getMe,
  getInviteInfo,
  listAnnouncements,
  markAnnouncementRead,
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
  listProducts,
  listProductCategories,
  getBillingSettings,
  listPromptPlaza,
  listPromptPlazaForUser,
  listMyPromptPlaza,
  submitPromptPlaza,
  togglePromptPlazaFavorite,
  redeemCode,
  adminStats,
  adminListUsers,
  adminListImageTasks,
  adminListRedeemCodes,
  adminCreateRedeemCodes,
  adminDeleteRedeemCode,
  adminListContentAudits,
  adminReviewContentAudit,
  adminListActionLogs,
  adminPreviewDataCleanup,
  adminExecuteDataCleanup,
  adminListAnnouncements,
  adminCreateAnnouncement,
  adminUploadAnnouncementImage,
  adminUploadPromptPlazaImage,
  adminUploadProductImage,
  adminUpdateAnnouncement,
  adminDeleteAnnouncement,
  adminListPromptPlaza,
  adminCreatePromptPlaza,
  adminUpdatePromptPlaza,
  adminReviewPromptPlaza,
  adminDeletePromptPlaza,
  adminListPackages,
  adminListProducts,
  adminCreateProduct,
  adminUpdateProduct,
  adminDeleteProduct,
  adminListProductCategories,
  adminCreateProductCategory,
  adminUpdateProductCategory,
  adminDeleteProductCategory,
  adminGetBillingSettings,
  adminUpdateBillingSettings,
  adminUpdateUserStatus,
  adminAdjustPoints,
  adminListUserActivity,
  adminCreatePackage,
  adminDeletePackage,
  adminUpdatePackage,
  adminListProviderKeys,
  adminCreateProviderKey,
  adminUpdateProviderKey,
  adminUseProviderKey,
  adminDeleteProviderKey,
}
