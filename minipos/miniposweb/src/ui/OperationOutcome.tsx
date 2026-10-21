import {Card,Text} from 'react-native-paper';
import type {FiscalOperation} from '../types';
import {operationOutcomeKind} from './operation-outcome';
import {operationOutcomeCopyFor} from './operation-outcome-copy';
import {fiscalOperationCopyFor} from './fiscal-operation-copy';
export function OperationOutcome({value,language}:{value:FiscalOperation;language:string}){
 const kind=operationOutcomeKind(value),copy=operationOutcomeCopyFor(language);
 return <Card mode="outlined" testID="sale-outcome" style={{marginTop:16,minWidth:0}}><Card.Content style={{gap:8}}>
 <Text variant="titleMedium" accessibilityRole="header" accessibilityLiveRegion="polite" testID="sale-outcome-title">{copy[`${kind}Title`]}</Text>
 <Text variant="bodyMedium">{copy[`${kind}Body`]}</Text>
 {kind==='confirmed'&&value.fiscal_reference?<Text selectable variant="bodyMedium">{fiscalOperationCopyFor(language).fiscal_reference}: {value.fiscal_reference}</Text>:null}
 </Card.Content></Card>;
}
