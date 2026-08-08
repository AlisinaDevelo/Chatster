import { expect, test } from '@playwright/test';

const messageContent = (page, content) =>
  page.locator('.message-content').filter({ hasText: content });

async function waitForLive(page) {
  await expect(page).toHaveTitle(/Chatster/i);
  await expect(page.getByText('Live', { exact: true })).toBeVisible();
  await expect(page.getByPlaceholder(/enter your username/i)).toBeEnabled();
}

async function joinChat(page, username) {
  const input = page.getByPlaceholder(/enter your username/i);
  await input.fill(username);
  await input.press('Enter');
  await expect(page.getByText(username, { exact: true })).toBeVisible();
  await expect(page.getByPlaceholder(/type your message/i)).toBeEnabled();
}

async function sendMessage(page, content) {
  const input = page.getByPlaceholder(/type your message/i);
  await expect(input).toBeEnabled();
  await input.fill(content);
  await input.press('Enter');
  await expect(messageContent(page, content)).toBeVisible();
}

function uniqueToken(testInfo) {
  return `${Date.now()}-${testInfo.workerIndex}-${testInfo.repeatEachIndex}`;
}

test('onboards, delivers same-room messages, and catches up after refresh', async ({ page }, testInfo) => {
  const token = uniqueToken(testInfo);
  const username = `browser-${token}`;
  const content = `general-message-${token}`;

  await page.goto('/');
  await waitForLive(page);
  await joinChat(page, username);
  await sendMessage(page, content);

  await page.reload();
  await waitForLive(page);
  await expect(messageContent(page, content)).toBeVisible();
});

test('switches rooms and keeps live messages isolated', async ({ page }, testInfo) => {
  const token = uniqueToken(testInfo);
  const firstUsername = `room-a-${token}`;
  const secondUsername = `room-b-${token}`;
  const generalContent = `general-isolation-${token}`;
  const engineeringContent = `engineering-isolation-${token}`;
  const secondPage = await page.context().newPage();

  try {
    await Promise.all([
      page.goto('/rooms/general'),
      secondPage.goto('/rooms/general'),
    ]);
    await Promise.all([waitForLive(page), waitForLive(secondPage)]);
    await joinChat(page, firstUsername);
    await joinChat(secondPage, secondUsername);

    await sendMessage(secondPage, generalContent);
    await expect(messageContent(page, generalContent)).toBeVisible();

    await page.getByRole('combobox', { name: 'Chat room' }).selectOption('engineering');
    await expect(page).toHaveURL(/\/rooms\/engineering\/?$/);
    await expect(page.getByRole('combobox', { name: 'Chat room' })).toHaveValue('engineering');
    await expect(secondPage.getByText(`${firstUsername} left the chat`, { exact: true })).toBeVisible();

    await sendMessage(page, engineeringContent);

    await expect(messageContent(secondPage, generalContent)).toBeVisible();
    await expect(messageContent(secondPage, engineeringContent)).toHaveCount(0);
    await expect(messageContent(page, generalContent)).toHaveCount(0);
  } finally {
    await secondPage.close();
  }
});

test('keeps older history readable after appending messages', async ({ page }, testInfo) => {
  const token = uniqueToken(testInfo);
  const room = 'off-topic';
  const username = `scroll-${token}`;
  const prefix = `scroll-history-${token}`;
  const appendedCount = 24;

  await page.goto(`/rooms/${room}`);
  await waitForLive(page);
  await joinChat(page, username);
  for (let index = 0; index < appendedCount; index += 1) {
    await sendMessage(page, `${prefix}-${index}`);
  }

  const log = page.getByRole('log', { name: 'Chat messages' });
  await log.focus();
  await expect(log).toBeFocused();

  const logBox = await log.boundingBox();
  await page.mouse.move(logBox.x + (logBox.width / 2), logBox.y + (logBox.height / 2));
  await page.mouse.wheel(0, -100_000);
  await expect(messageContent(page, `${prefix}-0`)).toBeVisible();
  await expect(log).toBeFocused();
});
