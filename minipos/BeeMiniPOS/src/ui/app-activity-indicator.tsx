import React from 'react';
import type { ComponentProps } from 'react';
import { ActivityIndicator } from 'react-native-paper';
import {useLanguageStore} from '../languageStore';
import {foundationCopy} from '../../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';

type Props = ComponentProps<typeof ActivityIndicator> & { accessibilityLabel?: string };

export function AppActivityIndicator({ accessibilityLabel, ...props }: Props) {
  const language=useLanguageStore(state=>state.language);
  const code=language.trim().toLowerCase().split(/[-_]/)[0];
  const copy=Object.prototype.hasOwnProperty.call(foundationCopy,code)?foundationCopy[code as keyof typeof foundationCopy]:foundationCopy.en;
  return <ActivityIndicator {...props} accessibilityLabel={accessibilityLabel??copy.loading} accessibilityLiveRegion="polite" />;
}
