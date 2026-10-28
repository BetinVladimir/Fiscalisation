import test from'node:test';import assert from'node:assert/strict';import{verifyDeviceDataReport,deviceDataCases,deviceDataStageIds}from'./verify-device-data-readable.mjs';
function fixture(){const specs=deviceDataCases.filter(c=>c.language==='en'&&c.width===320&&c.zoom===2).map(c=>({title:c.title,file:'device-data-readable.spec.ts',tests:[{status:'expected',expectedStatus:'passed',results:[{status:'passed',retry:0,errors:[],startTime:'2026-10-18T00:00:00Z',duration:2000,attachments:[{name:'presentation-scope',contentType:'application/json',value:{...c,sections:9,apiWorkflow:false,nativeAcceptance:'OPEN'}},...deviceDataStageIds.map(id=>({name:'axe-'+id,contentType:'application/json',value:{testEngine:{name:'axe-core'},toolOptions:{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa','wcag22aa']}},passes:[{id:'color-contrast'}],violations:[],incomplete:[],url:'http://127.0.0.1:53101/',timestamp:'2026-10-18T00:00:01Z'}}))]}]}]}));return{stats:{expected:2,unexpected:0,skipped:0,flaky:0},errors:[],suites:[{specs}]};}
const verify=r=>verifyDeviceDataReport(r,a=>a.value,'critical2');
test('exact two critical cases require18 raw stage scans and retain scope limits',()=>{const a=verify(fixture());assert.equal(a.rawAA,18);assert.equal(a.fullApplicationAcceptance,'OPEN');});
for(const [name,change]of [
 ['missing section',r=>r.suites[0].specs[0].tests[0].results[0].attachments.pop()],
 ['duplicate section',r=>{const a=r.suites[0].specs[0].tests[0].results[0].attachments;a.push(a[1])}],
 ['violation',r=>r.suites[0].specs[0].tests[0].results[0].attachments[1].value.violations.push({id:'color-contrast'})],
 ['incomplete',r=>r.suites[0].specs[0].tests[0].results[0].attachments[1].value.incomplete.push({id:'color-contrast'})],
 ['wrong locale',r=>r.suites[0].specs[0].tests[0].results[0].attachments[0].value.language='ru'],
 ['missing tag',r=>r.suites[0].specs[0].tests[0].results[0].attachments[1].value.toolOptions.runOnly.values.pop()],
 ['invented workflow acceptance',r=>r.suites[0].specs[0].tests[0].results[0].attachments[0].value.apiWorkflow=true],
 ['old scan',r=>r.suites[0].specs[0].tests[0].results[0].attachments[1].value.timestamp='2026-10-17T00:00:00Z'],
 ['retried case',r=>r.suites[0].specs[0].tests[0].results[0].retry=1],
 ['missing case',r=>r.suites[0].specs.pop()],
 ['wrong route',r=>r.suites[0].specs[0].tests[0].results[0].attachments[1].value.url='http://127.0.0.1:53101/production'],
 ['duplicate case',r=>r.suites[0].specs[1]=structuredClone(r.suites[0].specs[0])],
 ])test(name+' fails closed',()=>{const r=fixture();change(r);assert.throws(()=>verify(r))});
