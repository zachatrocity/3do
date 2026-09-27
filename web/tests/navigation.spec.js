import { test, expect } from '@playwright/test';

async function fixture(page, role = 'admin') {
  const items = Array.from({ length: 9 }, (_, i) => ({ id: i + 1, title: `Print ${i + 1}`, status: 'queued', priority: 'normal', quantity: 1, links: [], files: [], notes: [], status_events: [] }));
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.route('**/api/**', async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const id = Number(path.split('/')[3]);
    let payload = {};
    if (path === '/api/session') payload = { user: { id: 1, display_name: 'Tester', role } };
    else if (path === '/api/printers' || path === '/api/users') payload = [];
    else if (path === '/api/queue-items' && request.method() === 'POST') {
      payload = { ...items[0], id: 10, title: 'New print' }; items.push(payload);
    } else if (path === '/api/queue-items') payload = items;
    else if (path.endsWith('/notes')) {
      items.find(item => item.id === id).notes.push({ body: request.postDataJSON().body, author: 'Tester' });
    } else {
      payload = items.find(item => item.id === id);
      if (!payload) return route.fulfill({ status: 404, json: { error: 'Print not found' } });
      if (request.method() === 'PATCH') Object.assign(payload, request.postDataJSON());
    }
    await route.fulfill({ json: payload });
  });
  return errors;
}

test('one queue exposes every print and members can open details', async ({ page }) => {
  const errors = await fixture(page, 'member');
  await page.goto('/#dashboard');
  await expect(page).toHaveURL(/#queue\?view=board$/);
  await expect(page.locator('.mini-item')).toHaveCount(9);
  await expect(page.locator('#route-nav a')).toHaveText(['Queue', 'New print']);
  await page.getByRole('link', { name: /^Print 9/ }).click();
  await expect(page.locator('#detail-form')).toBeVisible();
  await page.reload();
  await expect(page.getByRole('heading', { name: /^Print 9/ })).toBeVisible();
  await page.getByRole('link', { name: 'Back to queue' }).click();
  await expect(page.locator('.mini-item')).toHaveCount(9);
  expect(errors).toEqual([]);
});

test('Back, Forward and explicit parent preserve list filters and clear selection', async ({ page }) => {
  await fixture(page);
  await page.goto('/#queue?view=list&status=queued');
  await page.getByRole('link', { name: 'View print' }).first().click();
  await expect(page).toHaveURL(/status=queued&item=1$/);
  await page.goBack();
  await expect(page.locator('#status-filter')).toHaveValue('queued');
  await expect(page.locator('#item-detail')).toHaveCount(0);
  await page.goForward();
  await expect(page.locator('#detail-form')).toBeVisible();
  await page.getByRole('link', { name: 'Back to queue' }).click();
  await expect(page).toHaveURL(/#queue\?view=list&status=queued$/);
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.locator('#status-filter')).toHaveValue('queued');
});

test('save and notes keep the detail URL, parent context and feedback', async ({ page }) => {
  const errors = await fixture(page);
  await page.goto('/#admin-queue?item=1&status=queued');
  await expect(page).toHaveURL(/#queue\?view=list&status=queued&item=1$/);
  await page.getByLabel('Owner', { exact: true }).fill('Zach');
  await page.getByRole('button', { name: 'Save changes' }).click();
  await expect(page.locator('#detail-status')).toHaveText('Saved.');
  await page.getByLabel('New note').fill('Test note');
  await page.getByRole('button', { name: 'Post note' }).click();
  await expect(page.locator('.timeline').first()).toContainText('Test note');
  await expect(page).toHaveURL(/status=queued&item=1$/);
  expect(errors).toEqual([]);
});

test('new print hides advanced options and opens its detail', async ({ page }) => {
  await fixture(page);
  await page.goto('/#queue');
  await page.getByRole('link', { name: 'New print', exact: true }).click();
  await expect(page.getByLabel('Owner', { exact: true })).not.toBeVisible();
  await page.getByLabel('Title', { exact: true }).fill('New print');
  await page.getByRole('button', { name: 'Add to queue' }).click();
  await expect(page).toHaveURL(/item=10$/);
  await expect(page.getByRole('heading', { name: 'New print', exact: true })).toBeVisible();
  await page.goBack();
  await expect(page.locator('#queue-content')).toBeVisible();
});

test('unknown routes and missing items have a usable way out', async ({ page }) => {
  await fixture(page);
  await page.goto('/#unknown');
  await expect(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
  await page.getByRole('link', { name: 'Return to queue' }).click();
  await page.goto('/#queue?item=999');
  await expect(page.getByRole('alert')).toHaveText('Print not found');
  await page.getByRole('link', { name: 'Back to queue' }).click();
  await expect(page.locator('.mini-item')).toHaveCount(9);
});

test('mobile navigation and details fit a narrow viewport', async ({ page }) => {
  await fixture(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/#queue');
  await expect(page.locator('.mini-item')).toHaveCount(9);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/queue-mobile.png', fullPage: true });
  await page.getByRole('link', { name: /^Print 1 normal/ }).click();
  await expect(page.locator('#detail-form')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/detail-mobile.png', fullPage: true });
});

test('member settings links are absent and direct settings routes are gated', async ({ page }) => {
  await fixture(page, 'member');
  await page.goto('/#admin-users');
  await expect(page.getByRole('heading', { name: 'Admin access required' })).toBeVisible();
  await expect(page.locator('#route-nav a')).toHaveText(['Queue', 'New print']);
  await page.getByRole('link', { name: 'Return to queue' }).click();
  await expect(page.locator('.mini-item')).toHaveCount(9);
});

test('failed note stays editable and reports the error', async ({ page }) => {
  await fixture(page);
  await page.route('**/api/queue-items/1/notes', route => route.fulfill({ status: 500, json: { error: 'Could not save note' } }));
  await page.goto('/#queue?item=1');
  await page.getByLabel('New note').fill('Keep this draft');
  await page.getByRole('button', { name: 'Post note' }).click();
  await expect(page.locator('#note-form .form-status')).toHaveText('Could not save note');
  await expect(page.getByLabel('New note')).toHaveValue('Keep this draft');
  await expect(page.getByRole('button', { name: 'Post note' })).toBeEnabled();
});

test('desktop board and list fit and switching views preserves status', async ({ page }) => {
  await fixture(page);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('/#queue');
  await expect(page.locator('.mini-item')).toHaveCount(9);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/queue-desktop.png', fullPage: true });
  await page.getByLabel('Filter by status').selectOption('queued');
  await page.getByRole('link', { name: 'List', exact: true }).click();
  await expect(page.locator('#status-filter')).toHaveValue('queued');
  await expect(page.locator('.queue-card')).toHaveCount(9);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
