import test from 'node:test';
import assert from 'node:assert/strict';

import {
  extractMonths,
  extractResolutionDates,
  extractSubject,
  setsEqual,
} from './pair_match_rules.js';

test('extractMonths normalizes month variants', () => {
  assert.deepEqual(
    Array.from(extractMonths('By Sept. 30 and December 1')).sort(),
    ['december', 'september'],
  );
});

test('extractResolutionDates captures explicit resolution dates', () => {
  assert.deepEqual(
    Array.from(
      extractResolutionDates(
        'This market resolves by January 31, 2026 at 11:59 PM ET.',
      ),
    ),
    ['january 31 2026'],
  );
});

test('extractSubject captures leading capitalized subject terms', () => {
  assert.deepEqual(
    Array.from(
      extractSubject(
        'Will Opinion Labs (OPINION) launch a token by March 31, 2026?',
      ),
    ).sort(),
    ['labs', 'opinion'],
  );
});

test('setsEqual compares exact membership', () => {
  assert.equal(setsEqual(new Set(['march']), new Set(['march'])), true);
  assert.equal(setsEqual(new Set(['march']), new Set(['april'])), false);
});
