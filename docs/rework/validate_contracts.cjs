/* Validate the Draft7-compatible subset of v1 using the existing frontend Ajv. */
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const root = __dirname;
let Ajv;
try { Ajv = require(path.resolve(root, '../../race_engineer_hub/node_modules/ajv')); }
catch { throw new Error('Install existing frontend dependencies with npm ci in race_engineer_hub before validating contracts.'); }
const read = file => JSON.parse(fs.readFileSync(path.join(root, file), 'utf8'));
const native = read('contracts/v1.schema.json');
// The authored schema uses no 2020-only validation keywords. Project its
// definition container/ref paths for the already-installed Draft7 validator.
function project(value) {
  if (Array.isArray(value)) return value.map(project);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([k,v]) => [k === '$defs' ? 'definitions' : k, project(v)]));
  return typeof value === 'string' ? value.replace('#/$defs/', '#/definitions/') : value;
}
const schema = project(native);
schema.$schema = 'http://json-schema.org/draft-07/schema#';
const ajv = new Ajv({allErrors:true, schemaId:'auto'});
ajv.addSchema(schema);
const validators = Object.fromEntries(Object.keys(native.$defs).map(name => [name, ajv.compile({$ref:schema.$id+'#/definitions/'+name})]));
let assertions = 0;
function valid(name, value) {
  const ok = validators[name](value);
  assert(ok, name+': '+JSON.stringify(validators[name].errors)); assertions++;
}
valid('Config', read('defaults.json'));
const examples = read('prompts/v1/examples.json');
const cases = fs.readFileSync(path.join(root,'prompts/v1/eval-cases.jsonl'),'utf8').trim().split('\n').map(JSON.parse);
const tools = read('prompts/v1/tools.json').tools;
for (const item of read('contracts/examples.json')) valid(item.definition, item.value);
for (const item of [...examples, ...cases]) {
  const input = item.input, output = item.output || item.reference_output, obs = input.observation;
  valid('AgentInput', input); valid('AgentOutput', output);
  for (const [key, expected] of Object.entries({invocation_id:input.invocation_id,session_id:obs.context.session_id,branch_id:obs.context.branch_id,observation_seq:obs.observation_seq})) assert.equal(output[key],expected);
  assert(output.notify || output.driver_message === null);
  if (output.driver_message) assert(output.driver_message.split(/\s+/).length<=35);
  const ids = new Set([...obs.metrics,...obs.events,...obs.estimates,...(obs.strategy_evaluation?.candidates || [])].map(x=>x.id));
  for (const call of item.tool_results || []) {
    const tool = tools.find(t=>t.name===call.tool); assert(tool);
    const argsValidator=ajv.compile(project(tool.input_schema)); assert(argsValidator(call.arguments));
    valid('ToolResult',call.result);
    if (call.result.data !== null) valid(tool.data_schema.split('/').pop(),call.result.data);
    for (const artifact of call.result.artifacts) ids.add(artifact.id);
    if (call.result.data?.inspected) {
      const hash=crypto.createHash('sha256').update(call.result.data.text).digest('hex');
      assert.equal(call.result.data.content_sha256,hash);
    }
  }
  assert(output.evidence_ids.every(id=>ids.has(id)),item.id+': unknown evidence');
  if (output.plan.candidate_id) assert(obs.strategy_evaluation?.candidates.some(c=>c.id===output.plan.candidate_id));
  for (const [key,expected] of Object.entries(item.expected?.must_equal || {})) assert.deepEqual(key.split('.').reduce((v,k)=>v[k],output),expected,item.id+': '+key);
  for (const id of item.expected?.must_include_evidence || []) assert(output.evidence_ids.includes(id));
  for (const id of item.expected?.must_include_sources || []) assert(output.sources.some(s=>s.id===id));
  assertions++;
}
const metric=examples[0].input.observation.metrics[0].measurement;
assert(!validators.Measurement({...metric,availability:'restricted'}));
assert(!validators.Measurement({...metric,value:null}));
assert(!validators.AgentOutput({...examples[0].output,extra:true}));
assert(!validators.Context({...examples[0].input.observation.context,season_link_id:'0'}));
for (const operation of read('contracts/http-operations.json').operations) {
  for (const name of [operation.request,operation.response]) if(name && !name.includes(':')) assert(validators[name],name);
}
console.log('PASS: '+Object.keys(validators).length+' contract definitions compiled; '+assertions+' fixture assertions; '+cases.length+' semantic reference cases (no model calls)');
