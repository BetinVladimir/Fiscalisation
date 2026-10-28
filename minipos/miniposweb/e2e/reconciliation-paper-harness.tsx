import React from 'react';
import {createRoot} from 'react-dom/client';
import {Surface,useTheme} from 'react-native-paper';
import {BeeloyPaperProvider} from '../src/ui/theme';
import {PaymentReconciliationPanel} from '../src/PaymentReconciliationPanel';
const params=new URLSearchParams(location.search),language=params.get('language')??'en';
document.documentElement.lang=language;
const fixture=await fetch('/__reconciliation_fixture').then(response=>{if(!response.ok)throw Error('Missing signed test fixture');return response.json()});
const proof={authorizations:[] as unknown[]};(window as any).__reconciliationProof=proof;
function Harness(){const theme=useTheme();return <Surface mode="flat" elevation={0} style={{minHeight:'100vh' as any,backgroundColor:theme.colors.background,padding:16}}><PaymentReconciliationPanel language={language} administrator scope={fixture.scope} assertActive={async()=>{}} routes={[]} authorize={async decision=>{proof.authorizations.push(structuredClone(decision));if(params.get('outcome')==='error')throw Error('{"private":"PRIVATE_BROWSER_BANK_SECRET"}');return fixture.command;}}/></Surface>}
createRoot(document.getElementById('root')!).render(<BeeloyPaperProvider><Harness/></BeeloyPaperProvider>);
