import source from '../../scripts/reconciliation-copy-source.json';
import editorial from './reconciliation-copy.editorial.json';
import {reconciliationCatalogues} from './reconciliation-copy.generated';
import {supportedLanguages} from './authTranslations.generated';
export type ReconciliationCopy=Record<keyof typeof source,string>;
export function reconciliationCopyFor(language:string):ReconciliationCopy{
 const normalized=language.toLowerCase().replace('_','-').split('-')[0];
 const locale=(supportedLanguages as readonly string[]).includes(normalized)?normalized:'en';
 const raw=reconciliationCatalogues[locale];if(!raw)throw Error('Missing validated reconciliation catalogue '+locale);
 return {...raw,...(editorial as Record<string,Partial<ReconciliationCopy>>)[locale]} as ReconciliationCopy;
}
