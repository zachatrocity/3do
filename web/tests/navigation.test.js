import test from 'node:test';
import assert from 'node:assert/strict';
import { parseRoute, routeHref, queueParent } from '../static/js/navigation.js';

test('legacy bookmarks normalize without losing item selection', () => {
  assert.deepEqual(parseRoute('#dashboard'), { name: 'queue', params: { view: 'board' } });
  assert.deepEqual(parseRoute('#admin-queue?item=17'), { name: 'queue', params: { view: 'list', item: '17' } });
});
test('parent retains filter and view but not stale item selection', () => {
  assert.equal(queueParent({ view: 'list', status: 'blocked', item: '17' }), '#queue?view=list&status=blocked');
});
test('invalid values normalize and links encode values', () => {
  assert.deepEqual(parseRoute('#queue?view=bogus&status=bogus&item=-1&extra=yes'), { name: 'queue', params: { view: 'board' } });
  assert.equal(routeHref('queue', { status: '', item: undefined, view: 'list' }), '#queue?view=list');
});
