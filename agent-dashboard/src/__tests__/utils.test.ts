import {
  cn,
  formatBytes,
  formatDate,
  formatRelativeTime,
  generateId,
  truncate,
  sleep,
  parseSSEData,
  getFileExtension,
  getFileIcon,
  debounce,
  throttle,
  isClient,
  getApiBaseUrl,
} from '../lib/utils';

describe('cn (className merger)', () => {
  it('merges class names correctly', () => {
    expect(cn('foo', 'bar')).toBe('foo bar');
  });

  it('handles conditional classes', () => {
    expect(cn('foo', false && 'bar', 'baz')).toBe('foo baz');
  });

  it('merges conflicting Tailwind classes', () => {
    expect(cn('px-2', 'px-4')).toBe('px-4');
  });

  it('handles arrays', () => {
    expect(cn(['foo', 'bar'])).toBe('foo bar');
  });

  it('handles objects', () => {
    expect(cn({ foo: true, bar: false, baz: true })).toBe('foo baz');
  });
});

describe('formatBytes', () => {
  it('formats 0 bytes', () => {
    expect(formatBytes(0)).toBe('0 Bytes');
  });

  it('formats bytes', () => {
    expect(formatBytes(500)).toBe('500 Bytes');
  });

  it('formats kilobytes', () => {
    expect(formatBytes(1024)).toBe('1 KB');
  });

  it('formats megabytes', () => {
    expect(formatBytes(1024 * 1024)).toBe('1 MB');
  });

  it('formats gigabytes', () => {
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1 GB');
  });

  it('respects decimal places', () => {
    expect(formatBytes(1536, 1)).toBe('1.5 KB');
    expect(formatBytes(1536, 0)).toBe('2 KB');
  });
});

describe('formatDate', () => {
  it('formats a date object', () => {
    const date = new Date('2024-01-15T10:30:00Z');
    const result = formatDate(date);
    expect(result).toBeTruthy();
    expect(typeof result).toBe('string');
  });

  it('formats a date string', () => {
    const result = formatDate('2024-01-15T10:30:00Z');
    expect(result).toBeTruthy();
    expect(typeof result).toBe('string');
  });
});

describe('formatRelativeTime', () => {
  it('formats recent date as relative time', () => {
    const now = new Date();
    const result = formatRelativeTime(now);
    expect(result).toContain('ago');
  });
});

describe('generateId', () => {
  it('generates unique IDs', () => {
    const id1 = generateId();
    const id2 = generateId();
    expect(id1).not.toBe(id2);
  });

  it('generates IDs with expected format', () => {
    const id = generateId();
    expect(id).toMatch(/^\d+-[a-z0-9]+$/);
  });
});

describe('truncate', () => {
  it('does not truncate short text', () => {
    expect(truncate('hello', 10)).toBe('hello');
  });

  it('truncates long text with ellipsis', () => {
    expect(truncate('hello world', 5)).toBe('hello...');
  });

  it('handles exact length', () => {
    expect(truncate('hello', 5)).toBe('hello');
  });
});

describe('sleep', () => {
  beforeEach(() => {
    jest.useRealTimers();
  });

  it('resolves after specified delay', async () => {
    const start = Date.now();
    await sleep(50);
    const elapsed = Date.now() - start;
    expect(elapsed).toBeGreaterThanOrEqual(45); // Allow some tolerance
  });
});

describe('parseSSEData', () => {
  it('parses valid JSON', () => {
    const result = parseSSEData<{ message: string }>('{"message": "hello"}');
    expect(result).toEqual({ message: 'hello' });
  });

  it('returns null for [DONE]', () => {
    expect(parseSSEData('[DONE]')).toBeNull();
  });

  it('returns null for invalid JSON', () => {
    const consoleSpy = jest.spyOn(console, 'error').mockImplementation();
    expect(parseSSEData('invalid json')).toBeNull();
    consoleSpy.mockRestore();
  });
});

describe('getFileExtension', () => {
  it('extracts extension from filename', () => {
    expect(getFileExtension('document.pdf')).toBe('pdf');
    expect(getFileExtension('image.PNG')).toBe('png');
  });

  it('handles multiple dots', () => {
    expect(getFileExtension('file.name.txt')).toBe('txt');
  });

  it('handles no extension', () => {
    expect(getFileExtension('filename')).toBe('');
  });
});

describe('getFileIcon', () => {
  it('returns correct icon for PDF', () => {
    expect(getFileIcon('application/pdf')).toBe('file-text');
  });

  it('returns correct icon for Word documents', () => {
    expect(getFileIcon('application/msword')).toBe('file-text');
    expect(getFileIcon('application/vnd.openxmlformats-officedocument')).toBe('file-text');
  });

  it('returns correct icon for images', () => {
    expect(getFileIcon('image/png')).toBe('image');
    expect(getFileIcon('image/jpeg')).toBe('image');
  });

  it('returns correct icon for audio', () => {
    expect(getFileIcon('audio/mp3')).toBe('music');
  });

  it('returns correct icon for video', () => {
    expect(getFileIcon('video/mp4')).toBe('video');
  });

  it('returns default icon for unknown types', () => {
    expect(getFileIcon('application/unknown')).toBe('file');
  });
});

describe('debounce', () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it('delays function execution', () => {
    const fn = jest.fn();
    const debouncedFn = debounce(fn, 100);

    debouncedFn();
    expect(fn).not.toHaveBeenCalled();

    jest.advanceTimersByTime(100);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it('resets delay on subsequent calls', () => {
    const fn = jest.fn();
    const debouncedFn = debounce(fn, 100);

    debouncedFn();
    jest.advanceTimersByTime(50);
    debouncedFn(); // Reset timer
    jest.advanceTimersByTime(50);
    expect(fn).not.toHaveBeenCalled();

    jest.advanceTimersByTime(50);
    expect(fn).toHaveBeenCalledTimes(1);
  });
});

describe('throttle', () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it('executes immediately on first call', () => {
    const fn = jest.fn();
    const throttledFn = throttle(fn, 100);

    throttledFn();
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it('ignores calls within throttle period', () => {
    const fn = jest.fn();
    const throttledFn = throttle(fn, 100);

    throttledFn();
    throttledFn();
    throttledFn();
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it('allows calls after throttle period', () => {
    const fn = jest.fn();
    const throttledFn = throttle(fn, 100);

    throttledFn();
    jest.advanceTimersByTime(100);
    throttledFn();
    expect(fn).toHaveBeenCalledTimes(2);
  });
});

describe('isClient', () => {
  it('returns true in browser environment', () => {
    expect(isClient()).toBe(true);
  });
});

describe('getApiBaseUrl', () => {
  it('returns default URL when env var not set', () => {
    const originalEnv = process.env.NEXT_PUBLIC_API_URL;
    delete process.env.NEXT_PUBLIC_API_URL;

    expect(getApiBaseUrl()).toBe('http://localhost:8000');

    process.env.NEXT_PUBLIC_API_URL = originalEnv;
  });
});
