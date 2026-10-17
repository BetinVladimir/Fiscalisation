// Authored only. Runtime deferred until every form and required test is ready.
import React from 'react';
import { act, create } from 'react-test-renderer';
(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
jest.mock('react-native', () => ({ Platform: { OS: 'ios' }, View: 'View', ScrollView: 'ScrollView', useWindowDimensions: () => ({ width: 390, height: 844 }) }));
jest.mock('react-native-safe-area-context', () => ({ useSafeAreaInsets: () => ({ top: 24, bottom: 20, left: 8, right: 8 }) }));
jest.mock('../../../../../BeeloyBackend/mobile/shared-ui/src/components/PaperDialogHost', () => ({ PaperDialogHost: (props: any) => props.visible ? require('react').createElement('DialogHost', props, props.children) : null }));
jest.mock('../app-button', () => ({ AppButton: 'Paper.Button' }));
jest.mock('react-native-paper', () => ({ Snackbar: 'Paper.Snackbar', Text: 'Paper.Text', Dialog: Object.assign((props: any) => require('react').createElement('Paper.Dialog', props, props.children), { Title: 'Paper.Title', Content: 'Paper.Content', Actions: 'Paper.Actions' }), useTheme: () => ({ colors: { outline: '#777', error: '#b00', inversePrimary: '#dad' } }) }));
import { AccessibleAlert, FeedbackProvider, showFeedback } from '../feedback';

test('old snackbar timer cannot erase a replacement even before commit', async () => {
  let tree: any; await act(async () => { tree = create(<FeedbackProvider />); });
  await act(async () => { showFeedback('First'); });
  const dismiss = tree.root.findByType('Paper.Snackbar').props.onDismiss;
  await act(async () => { showFeedback('Second'); dismiss(); });
  expect(tree.root.findByType('Paper.Snackbar').props.children).toBe('Second');
  await act(async () => { tree.unmount(); });
});

test('snackbar action is one-shot and cannot run after unmount', async () => {
  let tree: any; const action = jest.fn();
  await act(async () => { tree = create(<FeedbackProvider />); });
  await act(async () => { showFeedback('Message', { actionLabel: 'Retry', onAction: action }); });
  const press = tree.root.findByType('Paper.Snackbar').props.action.onPress;
  await act(async () => { press(); press(); }); expect(action).toHaveBeenCalledTimes(1);
  await act(async () => { showFeedback('Later', { actionLabel: 'Retry', onAction: action }); });
  const old = tree.root.findByType('Paper.Snackbar').props.action.onPress;
  await act(async () => { tree.unmount(); }); await act(async () => { old(); });
  expect(action).toHaveBeenCalledTimes(1);
});

test('alert uses safe area and compact actions with a single provider owner', async () => {
  let tree: any; const action = jest.fn();
  await act(async () => { tree = create(<><FeedbackProvider key="first" /><FeedbackProvider key="second" /></>); });
  await act(async () => { AccessibleAlert.alert('Confirm', 'Message', [{ text: 'Proceed', onPress: action }]); });
  expect(tree.root.findAllByType('Paper.Dialog')).toHaveLength(1);
  expect(tree.root.findByType('Paper.Dialog').props.style.width).toBe(322);
  expect(tree.root.findByProps({ testID: 'beefiscal-alert-scroll' }).props.style.maxHeight).toBe(752);
  expect(tree.root.findByType('Paper.Actions').props.style.flexDirection).toBe('column');
  const old = tree.root.findByProps({ testID: 'beefiscal-alert-action-0' }).props.onPress;
  // The remaining provider may own the replay, but the unmounted owner's action cannot consume it.
  await act(async () => { tree.update(<><FeedbackProvider key="first" /></>); });
  await act(async () => { old(); }); expect(action).not.toHaveBeenCalled();
  expect(tree.root.findAllByType('Paper.Dialog')).toHaveLength(1);
  const current = tree.root.findByProps({ testID: 'beefiscal-alert-action-0' }).props.onPress;
  await act(async () => { current(); current(); }); expect(action).toHaveBeenCalledTimes(1);
  await act(async () => { tree.unmount(); });
});
