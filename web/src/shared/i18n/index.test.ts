import { describe, expect, it } from 'vitest';

import { detectLanguage, resources } from './index';

function keysOf(value: unknown, prefix = ''): string[] {
  if (typeof value !== 'object' || value === null) {
    return [prefix];
  }
  return Object.entries(value).flatMap(([key, child]) =>
    keysOf(child, prefix ? `${prefix}.${key}` : key),
  );
}

describe('detectLanguage', () => {
  it.each([
    [['pt-BR', 'en'], 'pt'],
    [['PT'], 'pt'],
    [['en-US'], 'en'],
    [['fr', 'pt'], 'pt'],
    [['fr', 'de'], 'en'],
    [[], 'en'],
  ] as const)('maps %j to %s', (preferred, expected) => {
    expect(detectLanguage(preferred)).toBe(expected);
  });
});

describe('locales', () => {
  it('define the same keys in every language', () => {
    expect(keysOf(resources.pt.translation).sort()).toEqual(
      keysOf(resources.en.translation).sort(),
    );
  });
});
