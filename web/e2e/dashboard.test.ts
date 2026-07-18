import { test, expect } from '@playwright/test'

test.describe('stackyrd Dashboard', () => {
  test('renders main layout and title', async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', (err) => errors.push(err.message))

    await page.goto('/', { waitUntil: 'networkidle' })

    await expect(page).toHaveTitle('stackyrd Dashboard')
    await expect(page.locator('aside')).toBeVisible()
    await expect(page.getByText('stackyrd', { exact: true })).toBeVisible()
    await expect(page.getByText('Dashboard')).toBeVisible()
    await expect(page.getByRole('button', { name: /Refresh/i })).toBeVisible()
    expect(errors, `Page errors: ${errors.join(' | ')}`).toHaveLength(0)
  })

  test('Loads stat cards and panels', async ({ page }) => {
    await page.goto('/', { waitUntil: 'networkidle' })
    await page.waitForResponse((res) => res.url().includes('/health') && res.status() === 200, { timeout: 10000 })
    await page.waitForTimeout(1500)

    await expect(page.getByRole('heading', { name: /Infrastructure Components/ })).toBeVisible()
    await expect(page.getByRole('heading', { name: /Registered Services & Dependencies/ })).toBeVisible()
    await expect(page.getByRole('heading', { name: /Runtime Resources/ })).toBeVisible()
    await expect(page.getByRole('heading', { name: /API Quick Reference/ })).toBeVisible()
    await expect(page.getByRole('button', { name: /Refresh/i })).toBeVisible()
  })

  test('sidebar nav scrolls to section', async ({ page }) => {
    await page.goto('/', { waitUntil: 'networkidle' })
    await page.waitForTimeout(500)

    await page.getByRole('link', { name: 'Infrastructure' }).first().click()
    await expect(page.locator('section#infrastructure')).toBeInViewport()

    await page.getByRole('link', { name: 'Resources' }).click()
    await expect(page.locator('section#resources')).toBeInViewport()
  })

  test('responsive: cards stack on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 })
    await page.goto('/', { waitUntil: 'networkidle' })
    await page.waitForTimeout(500)

    const gridCols = await page.locator('section#overview .grid').evaluate(
      (el) => getComputedStyle(el).gridTemplateColumns
    )
    expect(gridCols.split(' ').length).toBe(1)
  })

  test('refresh updates sync timestamp', async ({ page }) => {
    await page.goto('/', { waitUntil: 'networkidle' })
    await page.waitForTimeout(500)

    const refresh = page.getByRole('button', { name: /Refresh/i })
    await expect(refresh).toBeVisible()
    await refresh.click()
    await expect(page.getByText(/Last sync:/)).toBeVisible()
  })
})
