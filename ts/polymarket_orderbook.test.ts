import test from 'node:test';
import assert from 'node:assert/strict';

import {
  normalizePolymarketBook,
  type PolymarketBookPayload,
} from './polymarket_orderbook.js';

test('normalizePolymarketBook sorts and limits bids/asks', () => {
  const payload: PolymarketBookPayload = {
    timestamp: '1773090708212',
    bids: [
      {price: '0.001', size: '7991'},
      {price: '0.228', size: '15'},
      {price: '0.244', size: '6'},
      {price: '0.22', size: '54.61'},
    ],
    asks: [
      {price: '0.999', size: '167.53'},
      {price: '0.278', size: '5'},
      {price: '0.256', size: '23.14'},
      {price: '0.292', size: '5'},
    ],
  };

  const book = normalizePolymarketBook(payload, 2);

  assert.equal(book.updateTimestampMs, 1773090708212);
  assert.equal(book.bestBid, 0.244);
  assert.equal(book.bestAsk, 0.256);
  assert.deepEqual(book.bids, [
    {price: 0.244, size: 6},
    {price: 0.228, size: 15},
  ]);
  assert.deepEqual(book.asks, [
    {price: 0.256, size: 23.14},
    {price: 0.278, size: 5},
  ]);
});

test('normalizePolymarketBook filters invalid rows and keeps empty arrays', () => {
  const payload: PolymarketBookPayload = {
    asks: [
      {price: 'NaN', size: '1'},
      {price: '1.2', size: '1'},
      {price: '0.42', size: '0'},
    ],
    bids: [{price: '0', size: '5'}],
  };

  const book = normalizePolymarketBook(payload, 8);

  assert.equal(book.bestAsk, null);
  assert.equal(book.bestBid, null);
  assert.deepEqual(book.asks, []);
  assert.deepEqual(book.bids, []);
});
