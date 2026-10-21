// Authored only; composition/ownership assertions are not WCAG runtime evidence.
import React from 'react';
import { act, create } from 'react-test-renderer';
(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
let mockMode = 'system'; const mockSetLanguage = jest.fn(), mockSetTheme = jest.fn();
jest.mock('react-native', () => ({ Platform: { OS: 'ios' }, View: 'View', ScrollView: 'ScrollView', useWindowDimensions: () => ({ height: 844, width: 390 }) }));
jest.mock('react-native-safe-area-context', () => ({ SafeAreaInsetsContext: require('react').createContext({ top: 24, bottom: 20, left: 8, right: 8 }) }));
jest.mock('../preferences', () => ({ languageOptions: [['en', 'English'], ['bg', 'Български']], useUiPreferences: () => ({ language: 'en', themeMode: mockMode, setLanguage: mockSetLanguage, setThemeMode: mockSetTheme }) }));
jest.mock('../app-button', () => ({ AppButton: 'Paper.Button' }));
jest.mock('react-native-paper', () => ({ Appbar: { Header: 'Paper.Header' }, IconButton: 'Paper.IconButton', Text: 'Paper.Text', Card: 'Paper.Card', Menu: { Item: 'Paper.MenuItem' }, useTheme: () => ({ dark: false, colors: { outline: '#777', surface: '#fff', onSurface: '#111' } }) }));
import { LanguageThemeControls } from '../LanguageThemeControls';
beforeEach(() => { jest.clearAllMocks(); mockMode = 'system'; });

test('genuine Paper menu items opt into multiline and flexible height', async () => {
  mockMode = 'system'; let tree: any;
  await act(async () => { tree = create(<LanguageThemeControls />); });
  await act(async () => { tree.root.findByProps({ testID: 'auth.language.open' }).props.onPress(); });
  const item = tree.root.findByProps({ testID: 'auth.language.bg' });
  expect(item.props.titleNumberOfLines).toBe(0); expect(item.props.style.height).toBe('auto');
  expect(item.props.contentStyle.minWidth).toBe(0);
  await act(async () => { item.props.onPress(); item.props.onPress(); });
  expect(mockSetLanguage).toHaveBeenCalledTimes(1); expect(mockSetLanguage).toHaveBeenCalledWith('bg');
  await act(async () => { tree.unmount(); });
});

test('old menu choice cannot cross preference rerender or unmount', async () => {
  mockMode = 'system'; let tree: any;
  await act(async () => { tree = create(<LanguageThemeControls />); });
  await act(async () => { tree.root.findByProps({ testID: 'auth.language.open' }).props.onPress(); });
  const old = tree.root.findByProps({ testID: 'auth.language.bg' }).props.onPress;
  mockMode = 'dark'; await act(async () => { tree.update(<LanguageThemeControls />); });
  await act(async () => { old(); }); expect(mockSetLanguage).not.toHaveBeenCalled();
  const current = tree.root.findByProps({ testID: 'auth.language.bg' }).props.onPress;
  await act(async () => { tree.unmount(); }); await act(async () => { current(); });
  expect(mockSetLanguage).not.toHaveBeenCalled();
});
