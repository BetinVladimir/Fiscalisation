import React from 'react';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type PropsWithChildren } from 'react';
import { Alert as NativeAlert, View, type AlertButton, type AlertOptions } from 'react-native';
import { Button, Dialog, Portal, Snackbar, Text, useTheme } from 'react-native-paper';

type FeedbackKind = 'info' | 'success' | 'error';
type Feedback = { message: string; kind: FeedbackKind; actionLabel?: string; onAction?: () => void };
type FeedbackApi = { show: (message: string, options?: Partial<Omit<Feedback, 'message'>>) => void; dismiss: () => void };
type AlertRequest = { title: string; message?: string; buttons: AlertButton[]; options?: AlertOptions };

const FeedbackContext = createContext<FeedbackApi | null>(null);
const feedbackListeners = new Set<(feedback: Feedback) => void>();
const alertListeners = new Set<(request: AlertRequest) => void>();

export function showFeedback(message: string, options: Partial<Omit<Feedback, 'message'>> = {}) {
  const feedback = { message, kind: options.kind || 'info', actionLabel: options.actionLabel, onAction: options.onAction } satisfies Feedback;
  feedbackListeners.forEach(listener => listener(feedback));
}

export const AccessibleAlert = {
  alert(title: string, message?: string, buttons?: AlertButton[], options?: AlertOptions) {
    if (buttons?.length) {
      const request = { title, message, buttons, options };
      if (alertListeners.size) alertListeners.forEach(listener => listener(request));
      else if (options) NativeAlert.alert(title, message, buttons, options);
      else NativeAlert.alert(title, message, buttons);
      return;
    }
    showFeedback(message ? `${title}: ${message}` : title, { kind: 'error' });
  },
};

export function FeedbackProvider({ children }: PropsWithChildren) {
  const theme = useTheme();
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const [alert, setAlert] = useState<AlertRequest | null>(null);
  const dismiss = useCallback(() => setFeedback(null), []);
  const dismissAlert = useCallback(() => setAlert(null), []);
  const show = useCallback((message: string, options: Partial<Omit<Feedback, 'message'>> = {}) => {
    setFeedback({ message, kind: options.kind || 'info', actionLabel: options.actionLabel, onAction: options.onAction });
  }, []);

  useEffect(() => {
    feedbackListeners.add(setFeedback);
    alertListeners.add(setAlert);
    return () => { feedbackListeners.delete(setFeedback); alertListeners.delete(setAlert); };
  }, []);

  const value = useMemo(() => ({ show, dismiss }), [show, dismiss]);
  return (
    <FeedbackContext.Provider value={value}>
      {children}
      <Snackbar
        visible={Boolean(feedback)}
        onDismiss={dismiss}
        duration={feedback?.kind === 'error' ? 7000 : 4000}
        accessibilityLiveRegion="polite"
        action={feedback?.actionLabel ? { label: feedback.actionLabel, onPress: () => { feedback.onAction?.(); dismiss(); } } : undefined}
      >
        {feedback?.message || ''}
      </Snackbar>
      <Portal>
        <View accessibilityViewIsModal>
          <Dialog visible={Boolean(alert)} onDismiss={alert?.options?.cancelable === false ? undefined : dismissAlert}>
            <Dialog.Title>{alert?.title || ''}</Dialog.Title>
            {alert?.message ? <Dialog.Content><Text variant="bodyMedium">{alert.message}</Text></Dialog.Content> : null}
            <Dialog.Actions>
              {(alert?.buttons?.length ? alert.buttons : [{ text: 'OK' }]).map((button, index) => (
                <Button
                  key={`${button.text || 'OK'}-${index}`}
                  textColor={button.style === 'destructive' ? theme.colors.error : undefined}
                  onPress={() => { dismissAlert(); button.onPress?.(); }}
                >
                  {button.text || 'OK'}
                </Button>
              ))}
            </Dialog.Actions>
          </Dialog>
        </View>
      </Portal>
    </FeedbackContext.Provider>
  );
}

export function useFeedback() {
  const value = useContext(FeedbackContext);
  if (!value) throw new Error('useFeedback must be used inside BeeloyAppProvider');
  return value;
}
