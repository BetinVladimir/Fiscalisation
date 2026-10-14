// Authored only; execution deferred by global forms/test gate.
import test from 'node:test';import assert from 'node:assert/strict';
import {compositeSelectionValid as valid} from './compositeSelection.ts';
const devices=[{id:'a',status:'ACTIVE',model:'BLUECASH-50'},{id:'f',status:'ACTIVE'},{id:'p',status:'ACTIVE'}];
test('embedded requires same active BlueCash in all roles',()=>{assert.equal(valid('DATECS_BLUECASH50_EMBEDDED','a','a','a',devices),true);assert.equal(valid('DATECS_BLUECASH50_EMBEDDED','a','f','a',devices),false);assert.equal(valid('DATECS_BLUECASH50_EMBEDDED','a','a','',devices),false);assert.equal(valid('DATECS_BLUECASH50_EMBEDDED','a','a','a',[{id:'a',status:'ACTIVE',model:'OTHER'}]),false);});
test('DP profile requires payment separate from adapter',()=>{assert.equal(valid('DATECS_DP150_BLUEPAD50','a','f','p',devices),true);assert.equal(valid('DATECS_DP150_BLUEPAD50','a','f','',devices),false);assert.equal(valid('DATECS_DP150_BLUEPAD50','a','f','a',devices),false);});
test('Daisy profile excludes payment and shared adapter',()=>{assert.equal(valid('DAISY_COMPACT_S01','a','f','',devices),true);assert.equal(valid('DAISY_COMPACT_S01','a','a','',devices),false);assert.equal(valid('DAISY_COMPACT_S01','a','f','p',devices),false);});
test('unknown profile, inactive or duplicate devices rejected',()=>{assert.equal(valid('unknown','a','f','p',devices),false);assert.equal(valid('DAISY_COMPACT_S01','a','f','',devices.map(x=>({...x,status:'DISABLED'}))),false);assert.equal(valid('DAISY_COMPACT_S01','a','f','',[...devices,devices[0]]),false);});
