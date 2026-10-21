import source from '../../scripts/operation-outcome-source.json';
import {operationOutcomeCatalogues} from './operation-outcome-copy.generated';
import {supportedLanguages} from './authTranslations.generated';
export type OperationOutcomeCopy=Record<keyof typeof source,string>;
export function operationOutcomeCopyFor(language:string):OperationOutcomeCopy{
 const normalized=language.toLowerCase().replace('_','-').split('-')[0];
 const locale=(supportedLanguages as readonly string[]).includes(normalized)?normalized:'en';
 const copy=operationOutcomeCatalogues[locale];
 if(!copy)throw Error('Missing validated operation outcome catalogue '+locale);
 return copy as OperationOutcomeCopy;
}
