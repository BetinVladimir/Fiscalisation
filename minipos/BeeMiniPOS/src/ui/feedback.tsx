import { PaperDialogHost } from '../../../../../BeeloyBackend/mobile/shared-ui/src/components/PaperDialogHost';
import React from 'react';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type PropsWithChildren } from 'react';
import { View, ScrollView, Platform, useWindowDimensions, type AlertButton, type AlertOptions } from 'react-native';
import { Dialog, Snackbar, Text, useTheme } from 'react-native-paper';

import { AppButton as Button } from './app-button';

type FeedbackKind = 'info' | 'success' | 'error';
type Feedback = { message: string; kind: FeedbackKind; actionLabel?: string; onAction?: () => void };
type FeedbackApi = { show: (message: string, options?: Partial<Omit<Feedback, 'message'>>) => void; dismiss: () => void };
type AlertRequest = { title: string; message?: string; buttons: AlertButton[]; options?: AlertOptions };

const FeedbackContext = createContext<FeedbackApi | null>(null);
const feedbackListeners = new Set<(feedback: Feedback) => void>();
const alertListeners = new Set<(request: AlertRequest) => void>();
const pendingAlerts: AlertRequest[] = [];
let activeAlert: AlertRequest | null = null;
let activeAlertListener: ((request: AlertRequest) => void) | null = null;
function presentNextAlert() {
  if (activeAlert || !alertListeners.size) return;
  activeAlert = pendingAlerts.shift() ?? null;
  if (activeAlert) {
    activeAlertListener = Array.from(alertListeners).at(-1)!;
    activeAlertListener(activeAlert);
  }
}
function releaseAlert(request: AlertRequest) {
  if (activeAlert !== request) return false;
  activeAlert = null; activeAlertListener = null;
  queueMicrotask(presentNextAlert);
  return true;
}

export function showFeedback(message: string, options: Partial<Omit<Feedback, 'message'>> = {}) {
  const feedback = { message, kind: options.kind || 'info', actionLabel: options.actionLabel, onAction: options.onAction } satisfies Feedback;
  feedbackListeners.forEach(listener => listener(feedback));
}

export const AccessibleAlert = {
  alert(title: string, message?: string, buttons?: AlertButton[], options?: AlertOptions) {
    if (buttons?.length) {
      const request = { title, message, buttons, options };
      pendingAlerts.push(request);
      presentNextAlert();
      return;
    }
    showFeedback(message ? `${title}: ${message}` : title, { kind: 'error' });
  },
};

export function FeedbackProvider({ children }: PropsWithChildren) {
  const theme = useTheme();
  const { height, width } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const maxHeight = Math.max(0, height - insets.top - insets.bottom - 48);
  const dialogWidth = Math.max(0, Math.min(560, width - insets.left - insets.right - 52));
  const mounted = useRef(false);
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const [alert, setAlert] = useState<AlertRequest | null>(null);
  const currentAlert = useRef(alert); currentAlert.current = alert;
  const currentFeedback = useRef(feedback);
  currentFeedback.current = feedback;
  const consumedFeedback = useRef<Feedback | null>(null);
  const backgroundRef = useRef<View>(null);
  const openerRef = useRef<HTMLElement | null>(null);
  const [dialogNode, setDialogNode] = useState<HTMLElement | null>(null);
  const dialogRef = useCallback((node: View | null) => {
    if (Platform.OS === 'web') setDialogNode(node as unknown as HTMLElement | null);
  }, []);
  const receiveAlert = useCallback((request: AlertRequest) => {
    if (!mounted.current) return;
    currentAlert.current = request;
    if (Platform.OS === 'web' && !openerRef.current) openerRef.current = document.activeElement as HTMLElement | null;
    setAlert(request);
  }, []);
  const receiveFeedback = useCallback((request: Feedback) => {
    if (!mounted.current) return;
    currentFeedback.current = request; setFeedback(request);
  }, []);
  const dismiss = useCallback(() => {
    if (!mounted.current) return;
    currentFeedback.current = null; setFeedback(null);
  }, []);
  const dismissFeedback = (request: Feedback | null) => {
    if (!mounted.current || !request || currentFeedback.current !== request || consumedFeedback.current === request) return;
    consumedFeedback.current = request; dismiss();
  };
  const dismissAlert = useCallback((request: AlertRequest) => {
    if (!mounted.current || currentAlert.current !== request || !releaseAlert(request)) return false;
    currentAlert.current = null;
    setAlert(null);
    return true;
  }, []);
  const show = useCallback((message: string, options: Partial<Omit<Feedback, 'message'>> = {}) => {
    receiveFeedback({ message, kind: options.kind || 'info', actionLabel: options.actionLabel, onAction: options.onAction });
  }, [receiveFeedback]);

  useEffect(() => {
    mounted.current = true;
    feedbackListeners.add(receiveFeedback);
    alertListeners.add(receiveAlert);
    presentNextAlert();
    return () => {
      mounted.current = false;
      feedbackListeners.delete(receiveFeedback);
      alertListeners.delete(receiveAlert);
      if (activeAlertListener === receiveAlert && activeAlert) { pendingAlerts.unshift(activeAlert); activeAlert = null; activeAlertListener = null; queueMicrotask(presentNextAlert); }
    };
  }, [receiveAlert, receiveFeedback]);

  const dismissFromOutside = useCallback(() => {
    if (!alert || alert.options?.cancelable === false) return;
    if (dismissAlert(alert)) alert.options?.onDismiss?.();
  }, [alert, dismissAlert]);

  useEffect(() => {
    if (Platform.OS !== 'web' || !alert || !dialogNode) return;
    const background = backgroundRef.current as unknown as HTMLElement | null;
    const previousInert = background?.inert;
    if (background) background.inert = true;
    const targets = () => Array.from(dialogNode.querySelectorAll<HTMLElement>('button:not(:disabled),[href],input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex]:not([tabindex="-1"])')).filter(node => node.getClientRects().length && node.getAttribute('aria-hidden') !== 'true');
    const focusFirst = () => (targets()[0] ?? dialogNode).focus();
    focusFirst();
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); dismissFromOutside(); return; }
      if (event.key !== 'Tab') return;
      const nodes = targets(), index = nodes.indexOf(document.activeElement as HTMLElement);
      if (!nodes.length) { event.preventDefault(); dialogNode.focus(); return; }
      if (event.shiftKey ? index <= 0 : index < 0 || index === nodes.length - 1) { event.preventDefault(); nodes[event.shiftKey ? nodes.length - 1 : 0].focus(); }
    };
    const focusin = (event: FocusEvent) => { if (!dialogNode.contains(event.target as Node)) focusFirst(); };
    document.addEventListener('keydown', keydown, true);
    document.addEventListener('focusin', focusin, true);
    return () => {
      document.removeEventListener('keydown', keydown, true);
      document.removeEventListener('focusin', focusin, true);
      if (background) background.inert = previousInert ?? false;
      const opener = openerRef.current;
      openerRef.current = null;
      if (opener?.isConnected) opener.focus();
    };
  }, [alert, dialogNode, dismissFromOutside]);

  const value = useMemo(() => ({ show, dismiss }), [show, dismiss]);
  return (
    <FeedbackContext.Provider value={value}>
      <View ref={backgroundRef} testID="beeminipos-feedback-background" style={{ flex: 1 }} accessibilityElementsHidden={Boolean(alert)} importantForAccessibility={alert ? 'no-hide-descendants' : 'auto'} aria-hidden={Boolean(alert)}>
      {children}
      <Snackbar

        visible={Boolean(feedback)}
        onDismiss={() => dismissFeedback(feedback)}
        duration={feedback?.kind === 'error' ? 7000 : 4000}
        accessibilityLiveRegion="polite"
        action={feedback?.actionLabel ? { mode: 'outlined', style: { borderColor: theme.colors.inversePrimary }, label: feedback.actionLabel, onPress: () => { if (!mounted.current || currentFeedback.current !== feedback || consumedFeedback.current === feedback) return; consumedFeedback.current = feedback; dismiss(); feedback.onAction?.(); } } : undefined}
      >
        {feedback?.message || ''}
      </Snackbar>
      </View>
      <PaperDialogHost visible={Boolean(alert)} onDismiss={dismissFromOutside}>
          <Dialog dismissable={alert?.options?.cancelable !== false} dismissableBackButton={alert?.options?.cancelable !== false} visible={Boolean(alert)} onDismiss={dismissFromOutside} style={{ alignSelf: 'center', width: dialogWidth, maxWidth: 560, maxHeight }}>
            <View ref={dialogRef} tabIndex={-1} role="dialog" aria-label={alert?.title || ''} aria-modal={Boolean(alert)} accessibilityViewIsModal testID="beeminipos-alert-dialog" style={{ flexShrink: 1 }}>
            <ScrollView testID="beeminipos-alert-scroll" style={{ maxHeight, flexShrink: 1 }} nestedScrollEnabled contentContainerStyle={{ paddingTop: 24 }} keyboardShouldPersistTaps="handled">
            <Dialog.Title>{alert?.title || ''}</Dialog.Title>
            {alert?.message ? <Dialog.Content><Text variant="bodyMedium">{alert.message}</Text></Dialog.Content> : null}
            <Dialog.Actions style={{ flexDirection: width < 600 ? 'column' : 'row', flexWrap: 'wrap', gap: 8 }}>
              {(alert?.buttons?.length ? alert.buttons : [{ text: 'OK' }]).map((button, index) => (
                <Button
                  mode="outlined"
                  testID={`beeminipos-alert-action-${index}`}
                  style={{ ...(width < 600 ? { width: '100%' as const } : {}), flexShrink: 1, borderColor: button.style === 'destructive' ? theme.colors.error : theme.colors.outline, minHeight: 48 }}
                  contentStyle={{ minHeight: 48 }}
                  key={`${button.text || 'OK'}-${index}`}
                  textColor={button.style === 'destructive' ? theme.colors.error : undefined}
                  onPress={() => { if (alert && dismissAlert(alert)) button.onPress?.(); }}
                >
                  {button.text || 'OK'}
                </Button>
              ))}
            </Dialog.Actions>
            </ScrollView>
            </View>
          </Dialog>
      </PaperDialogHost>
    </FeedbackContext.Provider>
  );
}

export function useFeedback() {
  const value = useContext(FeedbackContext);
  if (!value) throw new Error('useFeedback must be used inside BeeloyAppProvider');
  return value;
}
