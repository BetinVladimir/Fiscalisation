// Authored only; runtime deferred by the global gate.
import React from 'react';
import { act, create } from 'react-test-renderer';
(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
jest.mock('react-native-paper', () => ({ Button: 'Paper.Button' }));
jest.mock('../tokens', () => ({ touchTarget: { min: 48 } }));
import { AppButton } from '../app-button';

test('retained action cannot cross disabled/loading render or unmount', async () => {
  const action = jest.fn(); let tree: any;
  await act(async () => { tree = create(<AppButton onPress={action}>Save</AppButton>); });
  const old = tree.root.findByType('Paper.Button').props.onPress;
  await act(async () => { tree.update(<AppButton disabled onPress={action}>Save</AppButton>); });
  await act(async () => { old({}); tree.root.findByType('Paper.Button').props.onPress({}); });
  expect(action).not.toHaveBeenCalled();
  await act(async () => { tree.update(<AppButton loading onPress={action}>Save</AppButton>); });
  expect(tree.root.findByType('Paper.Button').props.disabled).toBe(true);
  await act(async () => { tree.update(<AppButton onPress={action}>Save</AppButton>); });
  const live = tree.root.findByType('Paper.Button').props.onPress;
  await act(async () => { live({}); }); expect(action).toHaveBeenCalledTimes(1);
  await act(async () => { tree.unmount(); });
  await act(async () => { live({}); }); expect(action).toHaveBeenCalledTimes(1);
});

test('default and text actions have an outlined Paper frame and wrapping label', async () => {
  let tree: any; await act(async () => { tree = create(<AppButton mode="text">Long action</AppButton>); });
  const button = tree.root.findByType('Paper.Button');
  expect(button.props.mode).toBe('outlined');
  expect(button.props.labelNumberOfLines).toBe(0);
  expect(button.props.contentStyle[0]).toEqual({ minHeight: 48, minWidth: 48 });
  await act(async () => { tree.unmount(); });
});
