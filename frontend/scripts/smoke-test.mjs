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

  const suffix = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 17)
  const email = `smoke-${suffix}@example.com`
  const username = `smoke_${suffix}`
  const password = 'Passw0rd!123'

  console.log(`1. 打开前端：${appUrl}`)
  await page.goto(appUrl, { waitUntil: 'networkidle' })

  const inputs = page.locator('input')
  await inputs.nth(0).fill(email)
  await inputs.nth(2).fill(username)
  await inputs.nth(3).fill(password)
  await page.getByRole('button', { name: '发送验证码' }).click()
  await page.waitForFunction(() => {
    const el = document.querySelector('input[placeholder="123456"]')
    return !!el && el.value.length === 6
  })
  const debugCode = await inputs.nth(1).inputValue()
  if (!debugCode) {
    throw new Error('开发环境没有返回 debugCode，无法自动完成注册')
  }
  console.log(`2. 验证码已生成：${debugCode}`)

  await page.getByRole('button', { name: '注册并登录' }).click()
  await page.waitForFunction(() => !!localStorage.getItem('ai-image-saas-token'))
  const token = await page.evaluate(() => localStorage.getItem('ai-image-saas-token'))
  if (!token) {
    throw new Error('注册后没有拿到 token')
  }
  console.log(`3. 注册登录成功：${username}`)

  await page.getByRole('button', { name: '每日签到' }).click()
  await page.waitForFunction(
    async (baseUrl) => {
      const token = localStorage.getItem('ai-image-saas-token')
      const resp = await fetch(`${baseUrl}/api/v1/user/points`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      const json = await resp.json()
      return json?.data?.balance === 3
    },
    apiBaseUrl,
  )
  let points = await apiGet('/api/v1/user/points', token)
  if (points?.data?.balance !== 3) {
    throw new Error(`签到后积分应为 3，实际返回：${JSON.stringify(points)}`)
  }
  console.log('4. 签到成功，积分 +3')

  const packages = await apiGet('/api/v1/packages', token)
  const redeemPackage = packages?.data?.items?.find((item) => item.packageType === 'redeem' && item.enabled)
  if (!redeemPackage) {
    throw new Error(`没有找到可用的积分兑换档位：${JSON.stringify(packages)}`)
  }

  const redeemResp = await page.evaluate(
    async ({ baseUrl, redeemPackageCode }) => {
      const token = localStorage.getItem('ai-image-saas-token')
      const resp = await fetch(`${baseUrl}/api/v1/user/points/redeem`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ packageCode: redeemPackageCode }),
      })
      return resp.json()
    },
    { baseUrl: apiBaseUrl, redeemPackageCode: redeemPackage.code },
  )
  if (redeemResp?.code !== 0) {
    throw new Error(`积分兑换失败：${JSON.stringify(redeemResp)}`)
  }
  points = await apiGet('/api/v1/user/points', token)
  if (points?.data?.balance !== 0 || points?.data?.totalCredits !== redeemPackage.creditsAmount) {
    throw new Error(`兑换后状态不正确：${JSON.stringify(points)}`)
  }
  console.log(`5. 积分兑换成功，获得 ${redeemPackage.creditsAmount} 次免费次数`)

  await page.locator('textarea').first().fill(prompt)
  await page.getByRole('button', { name: '生成图片' }).click()
  await page.getByText('生成进度').waitFor({ state: 'visible', timeout: 10000 })
  console.log('6. 生图任务已提交，前端进度面板可见')

  let latestTask = null
  let finalTask = null
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const list = await apiGet('/api/v1/image-tasks?page=1&pageSize=1', token)
    latestTask = list?.data?.items?.[0] ?? null
    if (latestTask) {
      finalTask = await apiGet(`/api/v1/image-tasks/${latestTask.id}`, token)
      if (finalTask?.data?.status !== 'generating') {
        break
      }
    }
    await sleep(2000)
  }
  if (!latestTask || !finalTask) {
    throw new Error('提交后没有查询到生图任务')
  }
  if (finalTask.data.status !== 'success') {
    throw new Error(`生图任务未成功：${JSON.stringify(finalTask)}`)
  }
  if (!finalTask.data.imageUrl) {
    throw new Error(`生图任务成功但缺少图片 URL：${JSON.stringify(finalTask)}`)
  }
  console.log(`7. 生图成功，任务 ${latestTask.id}，图片：${finalTask.data.imageUrl}`)

  await page.waitForFunction(() => document.querySelectorAll('img').length > 0, null, { timeout: 120000 })
  const imageSrcs = await page.locator('img').evaluateAll((els) => els.map((el) => el.getAttribute('src')).filter(Boolean))
  if (imageSrcs.length === 0) {
    throw new Error('前端没有渲染图片')
  }
  console.log(`8. 前端已渲染 ${imageSrcs.length} 张图片`)

  points = await apiGet('/api/v1/user/points', token)
  if (points?.data?.balance !== 0 || points?.data?.totalCredits !== 0) {
    throw new Error(`生图后状态应归零，实际返回：${JSON.stringify(points)}`)
  }
  console.log('9. 生图消耗次数成功，免费次数从 1 变为 0')

  const duplicateCheckin = await page.evaluate(async (baseUrl) => {
    const token = localStorage.getItem('ai-image-saas-token')
    const resp = await fetch(`${baseUrl}/api/v1/user/checkin`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: '{}',
    })
    return resp.json()
  }, apiBaseUrl)
  if (duplicateCheckin?.data?.checkedIn !== false) {
    throw new Error(`重复签到没有被拦住：${JSON.stringify(duplicateCheckin)}`)
  }
  console.log('10. 重复签到已被拦截')

  const rechargePackage =
    packages?.data?.items?.find((item) => item.packageType === 'recharge' && item.enabled && item.bonusPoints > 0) ??
    packages?.data?.items?.find((item) => item.packageType === 'recharge' && item.enabled)
  if (!rechargePackage) {
    throw new Error(`没有找到可用的充值档位：${JSON.stringify(packages)}`)
  }

  const createdOrder = await page.evaluate(
    async ({ baseUrl, packageCode }) => {
      const token = localStorage.getItem('ai-image-saas-token')
      const resp = await fetch(`${baseUrl}/api/v1/recharge-orders`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ packageCode }),
      })
      return resp.json()
    },
    { baseUrl: apiBaseUrl, packageCode: rechargePackage.code },
  )
  if (createdOrder?.code !== 0 || createdOrder?.data?.status !== 'pending') {
    throw new Error(`创建充值订单失败：${JSON.stringify(createdOrder)}`)
  }
  console.log(`11. 充值订单创建成功：${createdOrder.data.orderNo}`)

  const paidOrder = await page.evaluate(
    async ({ baseUrl, orderNo, amountCents }) => {
      const token = localStorage.getItem('ai-image-saas-token')
      const resp = await fetch(`${baseUrl}/api/v1/recharge-orders/${encodeURIComponent(orderNo)}/confirm`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          providerTradeNo: `smoke-${orderNo}`,
          paymentChannel: 'mock',
          amountCents,
          paidAt: new Date().toISOString(),
          remark: 'smoke test payment',
        }),
      })
      return resp.json()
    },
    { baseUrl: apiBaseUrl, orderNo: createdOrder.data.orderNo, amountCents: rechargePackage.amountCents },
  )
  if (paidOrder?.code !== 0 || paidOrder?.data?.status !== 'paid') {
    throw new Error(`确认充值订单失败：${JSON.stringify(paidOrder)}`)
  }

  points = await apiGet('/api/v1/user/points', token)
  if (points?.data?.paidCredits !== rechargePackage.creditsAmount || points?.data?.balance !== rechargePackage.bonusPoints) {
    throw new Error(`充值到账状态不正确：${JSON.stringify(points)}`)
  }
  console.log(
    `12. 充值订单确认成功，付费次数 +${rechargePackage.creditsAmount}，赠送积分 +${rechargePackage.bonusPoints}`,
  )

  await browser.close()
  console.log('SMOKE_OK')
}

main().catch((error) => {
  console.error('SMOKE_FAILED')
  console.error(error)
  process.exit(1)
})
