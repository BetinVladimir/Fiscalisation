import React from 'react';import {act,create} from 'react-test-renderer';
let mockLanguage='en';
jest.mock('react-native-paper',()=>({ActivityIndicator:'Paper.Indicator'}));
jest.mock('../src/ui/preferences',()=>({useUiPreferences:()=>({language:mockLanguage})}));
import {AppActivityIndicator} from '../src/ui/app-activity-indicator';
import {foundationCopy} from '../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
test('Paper loading announcement follows every supported locale and explicit override', async () => {
  let tree: any;
  try {
    await act(async () => { tree = create(<AppActivityIndicator />); });
    for (const [language, copy] of Object.entries(foundationCopy)) {
      mockLanguage = language;
      await act(async () => { tree.update(<AppActivityIndicator />); });
      const indicator = tree.root.findByType('Paper.Indicator');
      expect(typeof copy.loading).toBe('string');
      expect(copy.loading.trim().length).toBeGreaterThan(0);
      expect(indicator.props.accessibilityLabel).toBe(copy.loading);
      expect(indicator.props.accessibilityLiveRegion).toBe('polite');
    }
    for (const language of ['constructor', '__proto__', 'unknown', '']) {
      mockLanguage = language;
      await act(async () => { tree.update(<AppActivityIndicator />); });
      expect(tree.root.findByType('Paper.Indicator').props.accessibilityLabel).toBe(foundationCopy.en.loading);
    }
    mockLanguage = ' RU_ru ';
    await act(async () => { tree.update(<AppActivityIndicator />); });
    expect(tree.root.findByType('Paper.Indicator').props.accessibilityLabel).toBe(foundationCopy.ru.loading);
    await act(async () => { tree.update(<AppActivityIndicator accessibilityLabel="Explicit" size="large" animating={false} />); });
    const indicator = tree.root.findByType('Paper.Indicator');
    expect(indicator.props.accessibilityLabel).toBe('Explicit');
    expect(indicator.props.size).toBe('large');
    expect(indicator.props.animating).toBe(false);
  } finally {
    if (tree) await act(async () => { tree.unmount(); });
    mockLanguage = 'en';
  }
});
