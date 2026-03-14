import { describe, expect, it } from 'vitest';
import { shortenModelName } from './modelName';

const cases: [string, string][] = [
  // 空值 / 未知格式 → 透传
  ['', ''],
  ['custom-model-alpha', 'custom-model-alpha'],

  // Claude 系列
  ['claude-haiku-4-5-20251001', 'haiku-4.5'],
  ['claude-sonnet-4-6', 'sonnet-4.6'],
  ['claude-opus-4-6', 'opus-4.6'],
  ['claude-sonnet-4-6-2025-10-01', 'sonnet-4.6'],

  // GPT 系列 — 非 claude/gemini 前缀保留
  ['gpt-5.4', 'gpt-5.4'],
  ['gpt-5.3-codex', 'gpt-5.3-codex'],
  ['gpt-5-3-codex', 'gpt-5-3-codex'],

  // Gemini 系列 — 去除前缀，含点号版本不触发版本规范化
  ['gemini-2.5-flash', '2.5-flash'],
  ['gemini-2.5-flash-thinking', '2.5-flash-thinking'],
  ['gemini-3-flash-preview', '3-flash-preview'],

  // 仅日期后缀
  ['some-model-20240101', 'some-model'],

  // 仅前缀
  ['claude-custom', 'custom'],
  ['gemini-custom', 'custom'],
];

describe('shortenModelName', () => {
  it.each(cases)('"%s" → "%s"', (input, expected) => {
    expect(shortenModelName(input)).toBe(expected);
  });
});
