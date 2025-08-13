/**
 * Test setup for Shadowy Web3 API
 */

import 'jest';

// Mock browser APIs
Object.defineProperty(window, 'crypto', {
  value: {
    getRandomValues: (arr: Uint8Array) => {
      for (let i = 0; i < arr.length; i++) {
        arr[i] = Math.floor(Math.random() * 256);
      }
      return arr;
    }
  }
});

// Mock localStorage and sessionStorage
const mockStorage = {
  getItem: jest.fn(),
  setItem: jest.fn(),
  removeItem: jest.fn(),
  clear: jest.fn(),
  length: 0,
  key: jest.fn()
};

Object.defineProperty(window, 'localStorage', { value: mockStorage });
Object.defineProperty(window, 'sessionStorage', { value: mockStorage });

// Mock fetch
global.fetch = jest.fn();

// Mock WebAssembly
Object.defineProperty(global, 'WebAssembly', {
  value: {
    instantiateStreaming: jest.fn()
  }
});