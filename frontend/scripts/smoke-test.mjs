import { chromium } from 'playwright'
import fs from 'node:fs'

const appUrl = process.env.SMOKE_APP_URL ?? 'http://localhost:5173'
const apiBaseUrl = process.env.SMOKE_API_BASE_URL ?? 'http://localhost:8080'
const browserExecutable = process.env.SMOKE_BROWSER_EXECUTABLE ?? detectChromeExecutable()
const headless = process.env.SMOKE_HEADLESS !== 'false'
const timeoutMs = Number(process.env.SMOKE_TIMEOUT_MS ?? 8 * 60 * 1000)
const prompt = process.env.SMOKE_PROMPT ?? '一只站在城市屋顶的机械狐狸，电影感光影'

function detectChromeExecutable() {
  const candidates = [
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge',
    '/usr/bin/google-chrome',
    '/usr/bin/google-chrome-stable',
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser',
  ]
  return candidates.find((item) => fs.existsSync(item)) ?? ''
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function apiGet(path, token) {
  const resp = await fetch(`${apiBaseUrl}${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })
  return resp.json()
}

async function apiPost(path, token, payload) {
  const resp = await fetch(`${apiBaseUrl}${path}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(payload ?? {}),
  })
  return resp.json()
}

async function main() {
  const launchOptions = {
    headless,
    args: ['--no-sandbox'],
  }
  if (browserExecutable) {
    launchOptions.executablePath = browserExecutable
  } else {
    launchOptions.channel = process.env.SMOKE_BROWSER_CHANNEL ?? 'chrome'
  }

  const browser = await chromium.launch(launchOptions)
  const context = await browser.newContext({ viewport: { width: 1440, height: 1400 } })
  const page = await context.newPage()

  const username = 'smoke_admin'
  const password = 'SmokePass123!'

  console.log(`1. 打开前端：${appUrl}`)
  await page.goto(appUrl, { waitUntil: 'networkidle' })

  await page.getByPlaceholder('请输入用户名或邮箱').fill(username)
  await page.getByPlaceholder('请输入密码').fill(password)
  await page.getByRole('button', { name: '登录' }).click()
  await page.waitForFunction(() => !!localStorage.getItem('ai-image-saas-token'))
  const token = await page.evaluate(() => localStorage.getItem('ai-image-saas-token'))
  if (!token) {
    throw new Error('登录后没有拿到 token')
  }
  console.log(`2. 登录成功：${username}`)

  await page.getByRole('heading', { name: '概览' }).waitFor({ state: 'visible', timeout: 10000 })
  console.log('3. 管理后台页面已加载')

  const stats = await apiGet('/api/v1/admin/stats', token)
  if (stats?.code !== 0 || typeof stats?.data?.usersTotal !== 'number') {
    throw new Error(`后台概览接口异常：${JSON.stringify(stats)}`)
  }
  console.log(`4. 后台概览接口可用，用户数：${stats.data.usersTotal}`)

  const pointsBefore = await apiGet('/api/v1/user/points', token)
  if (pointsBefore?.code !== 0) {
    throw new Error(`用户资产读取失败：${JSON.stringify(pointsBefore)}`)
  }
  const codeResp = await apiPost('/api/v1/admin/redeem-codes', token, {
    count: 1,
    pointsAmount: 10,
    creditsAmount: 1,
    prefix: 'SMOKE',
    note: 'smoke test',
  })
  const code = codeResp?.data?.items?.[0]?.code
  if (codeResp?.code !== 0 || !code) {
    throw new Error(`兑换码生成失败：${JSON.stringify(codeResp)}`)
  }

  const redeemResp = await apiPost('/api/v1/redeem-code', token, { code })
  if (redeemResp?.code !== 0) {
    throw new Error(`兑换码兑换失败：${JSON.stringify(redeemResp)}`)
  }
  const pointsAfter = await apiGet('/api/v1/user/points', token)
  const expectedPoints = Number(pointsBefore.data.balance ?? 0) + 10
  const expectedCredits = Number(pointsBefore.data.totalCredits ?? 0) + 1
  if (Number(pointsAfter?.data?.balance ?? -1) !== expectedPoints || Number(pointsAfter?.data?.totalCredits ?? -1) !== expectedCredits) {
    throw new Error(`兑换后资产状态不正确：${JSON.stringify({ before: pointsBefore, after: pointsAfter })}`)
  }
  console.log('5. 兑换码生成与兑换链路通过')

  const createdTask = await apiPost('/api/v1/image-tasks', token, { prompt, count: 1 })
  const taskId = createdTask?.data?.taskId
  if (createdTask?.code !== 0 || !taskId) {
    throw new Error(`生图任务提交失败：${JSON.stringify(createdTask)}`)
  }
  console.log(`6. 生图任务提交成功，任务：${taskId}`)

  let finalTask = null
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    finalTask = await apiGet(`/api/v1/image-tasks/${taskId}`, token)
    if (finalTask?.data?.status !== 'generating') {
      break
    }
    await sleep(2000)
  }
  if (!finalTask) {
    throw new Error('提交后没有查询到生图任务')
  }
  if (!['success', 'failed'].includes(finalTask.data.status)) {
    throw new Error(`生图任务未进入终态：${JSON.stringify(finalTask)}`)
  }
  if (finalTask.data.status === 'success') {
    if (!finalTask.data.imageUrl) {
      throw new Error(`生图任务成功但缺少图片 URL：${JSON.stringify(finalTask)}`)
    }
    console.log(`7. 生图成功，图片：${finalTask.data.imageUrl}`)
  } else {
    console.log(`7. 生图进入失败态，错误：${finalTask.data.errorMessage || '无错误信息'}`)
  }

  const profile = await apiGet('/api/v1/user/me', token)
  if (!profile?.data?.email || !profile?.data?.username) {
    throw new Error(`用户资料加载异常：${JSON.stringify(profile)}`)
  }
  console.log(`8. 用户资料可用：${profile.data.username}`)

  const history = await apiGet('/api/v1/user/logs?page=1&pageSize=5', token)
  if (!Array.isArray(history?.data?.items)) {
    throw new Error(`活动记录加载异常：${JSON.stringify(history)}`)
  }
  console.log(`9. 用户日志可用，首屏 ${history.data.items.length} 条`)

  const taskDetail = await apiGet(`/api/v1/image-tasks/${taskId}`, token)
  if (!taskDetail?.data?.id && !taskDetail?.data?.taskId) {
    throw new Error(`任务详情加载异常：${JSON.stringify(taskDetail)}`)
  }
  console.log(`10. 任务详情可用：${taskId}`)

  await browser.close()
  console.log('SMOKE_OK')
}

main().catch((error) => {
  console.error('SMOKE_FAILED')
  console.error(error)
  process.exit(1)
})
