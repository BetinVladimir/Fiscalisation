import React from 'react';
import { View } from 'react-native';
import { Button, Dialog, Portal, Text, useTheme } from 'react-native-paper';

type Props = {
  visible: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
  busy?: boolean;
  onConfirm: () => void;
  onDismiss: () => void;
};

export function ConfirmDialog({ visible, title, message, confirmLabel = 'Подтвердить', cancelLabel = 'Отмена', destructive, busy, onConfirm, onDismiss }: Props) {
  const theme = useTheme();
  return (
    <Portal>
      <View accessibilityViewIsModal>
      <Dialog visible={visible} onDismiss={busy ? undefined : onDismiss}>
        <Dialog.Title>{title}</Dialog.Title>
        <Dialog.Content><Text variant="bodyMedium">{message}</Text></Dialog.Content>
        <Dialog.Actions>
          <Button disabled={busy} onPress={onDismiss}>{cancelLabel}</Button>
          <Button mode="contained" buttonColor={destructive ? theme.colors.error : undefined} loading={busy} disabled={busy} onPress={onConfirm}>{confirmLabel}</Button>
        </Dialog.Actions>
      </Dialog>
      </View>
    </Portal>
  );
}
