// Run with: node test/management_v8_adapter.cjs
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../internal/api/management_ai_providers_page.go'), 'utf8');
const start = source.indexOf('(function () {');
const end = source.indexOf('  function installClientMutationRefresh()', start);
assert(start >= 0 && end > start);
const program = source.slice(start, end) + '\nreturn {recordsFromResponse, patchRecord}; })()';
let groups;
let saved;
const client = {
  get: async () => structuredClone(groups),
  put: async (endpoint, value) => { saved = {endpoint, value}; },
};
const api = vm.runInNewContext(program, {window: {__cliproxyManagementClient: client}});

(async () => {
  groups = [{name: 'group-a', 'base-url': 'https://provider.example.test', priority: 4,
    'request-retry': 2, keys: [{'api-key': 'fixture-a', label: 'old'}, {'api-key': 'fixture-b', weight: 3}]}];
  const spec = {brand: 'codex', endpoint: '/config/api-keys/codex'};
  const records = api.recordsFromResponse(spec, structuredClone(groups));
  assert.equal(records.length, 2);
  assert.equal(records[0].priority, 4);
  await api.patchRecord(records[0], {label: 'new', priority: 7});
  assert.equal(saved.endpoint, spec.endpoint);
  assert.equal(saved.value[0].keys[0].label, 'new');
  assert.equal(saved.value[0].keys[0].priority, 7);
  assert.equal(saved.value[0].priority, 4);
  assert.equal(saved.value[0]['request-retry'], 2);
  assert.equal(saved.value[0].keys[1].weight, 3);
  groups[0].keys = groups[0].keys.slice(1);
  await assert.rejects(api.patchRecord(records[0], {label: 'stale'}), /configuration changed/);

  groups = [{name: 'compat', 'base-url': 'https://compat.example.test',
    'health-probe-enabled': true, 'health-probe-interval-seconds': 17,
    keys: [{'api-key': 'fixture-c'}], models: [{name: 'model-a'}]}];
  const compat = api.recordsFromResponse({brand: 'openaiCompatibility', endpoint: '/config/api-keys/openai-compatibility'}, structuredClone(groups))[0];
  await api.patchRecord(compat, {label: 'compat-label'});
  assert.equal(saved.value[0].label, 'compat-label');
  assert.equal(saved.value[0]['health-probe-enabled'], true);
  assert.equal(saved.value[0].keys[0]['api-key'], 'fixture-c');
  assert.equal(saved.value[0].models[0].name, 'model-a');
  console.log('v8 provider adapter: grouped keys, field preservation and stale selection passed');
})().catch(error => { console.error(error); process.exitCode = 1; });
