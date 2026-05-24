import assert from "node:assert/strict";
import test from "node:test";

import {
  formatCompactTokens,
  formatCurrency,
  formatDuration,
  formatIntegerTokens,
  formatPercent,
  formatSignedNumber,
  formatSignedPercent,
} from "./benchFormatters.mts";

test("bench formatters cover shared run metrics", () => {
  assert.equal(formatDuration(12.34), "12.3s");
  assert.equal(formatDuration(125), "2m 5s");
  assert.equal(formatCurrency(0), "$0.00");
  assert.equal(formatCurrency(0.004), "$0.004");
  assert.equal(formatCurrency(1.23), "$1.23");
  assert.equal(formatCompactTokens(1530), "1.5k");
  assert.equal(formatIntegerTokens(1530), "1,530");
  assert.equal(formatPercent(75.123), "75.1%");
  assert.equal(formatSignedNumber(1.25), "+1.3");
  assert.equal(formatSignedNumber(-1.25), "-1.3");
  assert.equal(formatSignedPercent(2.5), "+2.5pp");
});
