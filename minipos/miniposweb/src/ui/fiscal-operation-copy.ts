import source from '../../scripts/fiscal-operation-result-source.json';
import editorial from './fiscal-operation-copy.editorial.json';
import {fiscalOperationCatalogues} from './fiscal-operation-copy.generated';
import {supportedLanguages} from './authTranslations.generated';
export type FiscalOperationCopy=Record<keyof typeof source,string>;
export function fiscalOperationCopyFor(language:string):FiscalOperationCopy{
 const normalized=language.toLowerCase().replace('_','-').split('-')[0],locale=(supportedLanguages as readonly string[]).includes(normalized)?normalized:'en';
 const raw=fiscalOperationCatalogues[locale];if(!raw)throw Error('Missing validated fiscal operation catalogue '+locale);
 return {...raw,...(editorial as Record<string,Partial<FiscalOperationCopy>>)[locale]} as FiscalOperationCopy;
}
