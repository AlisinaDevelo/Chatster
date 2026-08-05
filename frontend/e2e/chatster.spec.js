import { expect, test } from '@playwright/test';

const messageContent = (page, content) =>
  page.locator('.message-content').filter({ hasText: content });

const messageCount = (page) => page.locator('.message-count');

async function waitForLive(page) {
  await expect(page).toHaveTitle(/Chatster/i);
  await expect(page.getByText('Live', { exact: true })).toBeVisible();
  await expect(page.getByPlaceholder(/enter your username/i)).toBeEnabled();
}

async function joinChat(page, username) {
  const input = page.getByPlaceholder(/enter your username/i);
  await input.fill(username);
  await page.getByRole('button', { name: 'Join chat' }).click();
  await expect(page.getByText(username, { exact: true })).toBeVisible();
  await expect(page.getByPlaceholder(/type your message/i)).toBeEnabled();
}

async function sendMessage(page, content) {
  const input = page.getByPlaceholder(/type your message/i);
  await expect(input).toBeEnabled();
  await input.fill(content);
  await page.getByRole('button', { name: 'Send' }).click();
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

    const generalCountAfterSwitch = await messageCount(secondPage).textContent();
    await sendMessage(page, engineeringContent);

    await expect(secondPage.getByText(generalCountAfterSwitch, { exact: true })).toBeVisible();
    await expect(messageContent(secondPage, engineeringContent)).toHaveCount(0);
    await expect(messageContent(page, generalContent)).toHaveCount(0);
  } finally {
    await secondPage.close();
  }
});
