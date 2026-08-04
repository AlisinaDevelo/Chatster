import '@testing-library/jest-dom/vitest';

const storageValues = {};

const localStorageStub = {
  getItem(key) {
    return storageValues[key] ?? null;
  },
  setItem(key, value) {
    storageValues[key] = String(value);
  },
  removeItem(key) {
    delete storageValues[key];
  },
  clear() {
    Object.keys(storageValues).forEach((key) => delete storageValues[key]);
  },
};

Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: localStorageStub,
});
