// Authored only; execution deferred until global gate.
import {expect,it} from 'vitest';
import {operationMessage} from '../src/ui/operation-message';
import {translations,supportedLanguages} from '../src/ui/workTranslations.generated';
import {foundationCopy} from '../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
it.each(supportedLanguages)('safe operation messages in %s',language=>{expect(operationMessage(language,'OFFLINE_REFERENCE')).toBe(translations[language].offlineReference);expect(operationMessage(language,'CLOUD_PREPARE_UNAVAILABLE')).toBe(translations[language].cloudPrepareUnavailable);for(const detail of ['Error: token=secret','https://private.example/path','SQL server trace','SESSION_CACHE_UNAVAILABLE',''])expect(operationMessage(language,detail)).toBe(foundationCopy[language].error);});
