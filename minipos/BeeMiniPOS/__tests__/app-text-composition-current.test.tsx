import React from 'react';import {act,create} from 'react-test-renderer';
jest.mock('react-native-paper',()=>({Text:'Paper.Text'}));
import {AppText} from '../src/ui/app-text';
test('AppText remains genuine Paper and preserves scaling/semantic overrides',async()=>{let tree:any;await act(async()=>{tree=create(<AppText accessibilityRole="header">Content</AppText>)});const text=tree.root.findByType('Paper.Text');expect(text.props.allowFontScaling).toBe(true);expect(text.props.maxFontSizeMultiplier).toBe(2);expect(text.props.accessibilityRole).toBe('header');await act(async()=>tree.update(<AppText maxFontSizeMultiplier={0}>Content</AppText>));expect(tree.root.findByType('Paper.Text').props.maxFontSizeMultiplier).toBe(2);await act(async()=>tree.unmount())});
