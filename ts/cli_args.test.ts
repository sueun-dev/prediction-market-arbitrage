import test from 'node:test';
import assert from 'node:assert/strict';

import {
  filterPredictArgs,
  resolvePolymarketMaxMarkets,
} from './cli_args.js';

test('filterPredictArgs removes managed output flags', () => {
  const result = filterPredictArgs([
    '--max-markets',
    '2',
    '--full-out',
    '/tmp/full.json',
    '--out=/tmp/view.json',
    '--skip-orderbook',
    '--raw-out',
    '/tmp/raw.json',
  ]);

  assert.deepEqual(result, [
    '--max-markets',
    '2',
    '--skip-orderbook',
    '--raw-out',
    '/tmp/raw.json',
  ]);
});

test('resolvePolymarketMaxMarkets uses CLI value by default', () => {
  assert.equal(
    resolvePolymarketMaxMarkets(['--max-markets', '7'], undefined),
    7,
  );
});

test('resolvePolymarketMaxMarkets supports inline CLI values', () => {
  assert.equal(
    resolvePolymarketMaxMarkets(['--max-markets=9'], undefined),
    9,
  );
});

test('resolvePolymarketMaxMarkets prefers explicit env override', () => {
  assert.equal(
    resolvePolymarketMaxMarkets(['--max-markets', '7'], '11'),
    11,
  );
});
