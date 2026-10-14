import {foundationCopy} from '../../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import {translations,supportedLanguages} from './workTranslations.generated';
export function operationMessage(language:typeof supportedLanguages[number],message:string):string{
 if(message==='OFFLINE_REFERENCE')return translations[language].offlineReference;
 if(message==='CLOUD_PREPARE_UNAVAILABLE')return translations[language].cloudPrepareUnavailable;
 return foundationCopy[language].error;
}
