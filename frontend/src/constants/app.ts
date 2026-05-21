export const GENERATION_WAITING_NOTICE = '请耐心等待，图片生成时间会随着尺寸、张数和质量档位提高而增长。高质量或 4K 图片通常会更慢，也更容易偶发失败；如生成失败，本次消耗会自动退回。'
export const sizeOptions = [
  { value: '1024x1024', label: '1024x1024 方图' },
  { value: '1024x1536', label: '1024x1536 竖图' },
  { value: '1536x1024', label: '1536x1024 横图' },
  { value: '2048x2048', label: '2048x2048 2K 方图' },
  { value: '2048x1152', label: '2048x1152 2K 横图' },
  { value: '1152x2048', label: '1152x2048 2K 竖图' },
  { value: '3840x2160', label: '3840x2160 4K 横图' },
  { value: '2160x3840', label: '2160x3840 4K 竖图' },
  { value: 'custom', label: '自定义尺寸' },
  { value: 'auto', label: '自动' },
]

export const CUSTOM_SIZE_MAX_SIDE = 3840

export const styleOptions = [
  { value: '', label: '无' },
  { value: 'realistic', label: '写实' },
  { value: 'anime', label: '动漫' },
  { value: 'oil painting', label: '油画' },
  { value: 'watercolor', label: '水彩' },
  { value: 'pixel art', label: '像素' },
  { value: '3d', label: '3D' },
  { value: 'cinematic', label: '电影感' },
]

export const qualityOptions = [
  { value: 'auto', label: '自动' },
  { value: 'low', label: '低质量，通常更快' },
  { value: 'medium', label: '中' },
  { value: 'high', label: '高质量，通常更慢' },
]

export const outputFormatOptions = [
  { value: 'default', label: '默认' },
  { value: 'jpeg', label: 'JPEG' },
  { value: 'webp', label: 'WebP' },
  { value: 'png', label: 'PNG' },
]

export const HISTORY_PAGE_SIZE = 5

export const FREE_REFERENCE_IMAGE_COUNT = 3

export const MAX_REFERENCE_IMAGES = 12

//_WAITING_NOTICE =
  '请耐心等待，图片生成时间会随着尺寸、张数和质量档位提高而增长。高质量或 4K 图片通常会更慢，也更容易偶发失败；如生成失败，本次消耗会自动退回。'