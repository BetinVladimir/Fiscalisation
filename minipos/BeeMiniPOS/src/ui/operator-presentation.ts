import {isFiscalResourceId} from '../fiscalIdentity';
/** Keep a bright decorative image from washing out the native light status bar. */
export const authIllustrationOpacity=(dark:boolean)=>dark?0.06:0.7;
/** Internal resource keys are for requests, not cashier-facing identity. */
export function registerDisplayLabel(items:readonly {id:string;code?:string;name?:string}[],id:string){
 const register=items.find(value=>value.id===id);
 return [register?.code,register?.name].find(value=>value?.trim()&&!isFiscalResourceId(value.trim()))?.trim()??'';
}
// UNP is the Latin transliteration of the Bulgarian regulated sale identifier УНП.
export const regulatoryIdentityLabel=(language:string)=>language==='en'?'UNP':'УНП';
