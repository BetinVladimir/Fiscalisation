import React, { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { useResponsiveLayout } from './responsive';
import { Platform, ScrollView, View } from 'react-native';
import {PaperDialogHost} from '../../../../BeeloyBackend/mobile/shared-ui/src/components/PaperDialogHost';
import {SafeAreaInsetsContext} from 'react-native-safe-area-context';
import { ActivityIndicator, Button, Dialog, Text, useTheme } from 'react-native-paper';

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
  const insets=useContext(SafeAreaInsetsContext)??{top:0,bottom:0,left:0,right:0};
  const mounted=useRef(false),snapshot={visible,busy,title,message,onConfirm,onDismiss},latest=useRef(snapshot),admitted=useRef<object|null>(null);latest.current=snapshot;
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[]);
  const actionable=()=>mounted.current&&latest.current===snapshot&&visible&&!busy;
  const dismiss=()=>{if(!actionable()||admitted.current===snapshot)return;admitted.current=snapshot;onDismiss()};
  const confirm=()=>{if(!actionable()||admitted.current===snapshot)return;admitted.current=snapshot;onConfirm()};
  const [dialogNode, setDialogNode] = useState<HTMLElement | null>(null);
  const dialogRef = useCallback((node: View | null) => {
    if (Platform.OS === 'web') setDialogNode(node as unknown as HTMLElement | null);
  }, []);
  const busyRef = useRef(busy), dismissRef = useRef(dismiss);
  busyRef.current = busy; dismissRef.current = dismiss;
  useEffect(() => {
    if (Platform.OS !== 'web' || !visible || !dialogNode) return;
    const opener = document.activeElement as HTMLElement | null;
    const restorations:Array<()=>void>=[];
    for(let branch:HTMLElement|null=dialogNode;branch?.parentElement;branch=branch.parentElement){
      for(const sibling of Array.from(branch.parentElement.children)){
        if(sibling===branch||!(sibling instanceof HTMLElement)||/^(SCRIPT|STYLE|LINK)$/.test(sibling.tagName))continue;
        const inert=sibling.inert,hidden=sibling.getAttribute('aria-hidden');
        sibling.inert=true;sibling.setAttribute('aria-hidden','true');
        restorations.push(()=>{sibling.inert=inert;if(hidden===null)sibling.removeAttribute('aria-hidden');else sibling.setAttribute('aria-hidden',hidden);});
      }
      if(branch.parentElement===document.body)break;
    }
    const targets = () => Array.from(dialogNode.querySelectorAll<HTMLElement>('button[data-testid="beefiscal-confirm-cancel"]:not(:disabled),button[data-testid="beefiscal-confirm-action"]:not(:disabled),[data-testid="beefiscal-confirm-message"]')).filter(node => node.getClientRects().length && node.getAttribute('aria-hidden') !== 'true' && (node.getAttribute('data-testid') !== 'beefiscal-confirm-message' || node.scrollHeight > node.clientHeight));
    const focusFirst = () => { const nodes = targets(); (nodes.find(node => node.getAttribute("data-testid") === "beefiscal-confirm-cancel") ?? nodes[0] ?? dialogNode).focus(); };
    focusFirst();
    // Portal commits can follow the owner's effect; focus newly mounted actions.
    const observer = new MutationObserver(() => { if (document.activeElement === dialogNode && targets().length) focusFirst(); });
    observer.observe(dialogNode, { childList: true, subtree: true, attributes: true, attributeFilter: ['disabled'] });
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); if (!busyRef.current) dismissRef.current(); return; }
      if (event.key !== 'Tab') return;
      const nodes = targets(), index = nodes.indexOf(document.activeElement as HTMLElement);
      if (!nodes.length) { event.preventDefault(); dialogNode.focus(); return; }
      if (event.shiftKey ? index <= 0 : index < 0 || index === nodes.length - 1) { event.preventDefault(); nodes[event.shiftKey ? nodes.length - 1 : 0].focus(); }
    };
    const focusin = (event: FocusEvent) => { if (!dialogNode.contains(event.target as Node)) focusFirst(); };
    document.addEventListener('keydown', keydown, true);
    document.addEventListener('focusin', focusin, true);
    return () => { restorations.reverse().forEach(restore=>restore()); observer.disconnect(); document.removeEventListener('keydown', keydown, true); document.removeEventListener('focusin', focusin, true); if (opener?.isConnected) opener.focus(); };
  }, [visible, dialogNode]);
  const { compact, height, width } = useResponsiveLayout();
  const actionStyle = { flexShrink: 1, marginRight: 0, ...(compact ? { width: '100%' as const } : {}) };
  const wrap = Platform.OS === 'web' ? { whiteSpace: 'normal', overflowWrap: 'anywhere', flexShrink: 1 } as const : { flexShrink: 1 };
  const maxHeight=Math.max(0,height-insets.top-insets.bottom-48);
  return (
    <PaperDialogHost visible={visible} onDismiss={dismiss}>
      <View ref={dialogRef} tabIndex={-1} role={visible ? "dialog" : undefined} aria-label={visible ? title : undefined} aria-modal={visible} accessibilityViewIsModal={visible} pointerEvents="box-none" style={{ flex: 1 }}>
      <Dialog visible={visible} style={{maxHeight,width:Math.max(0,Math.min(560,width-insets.left-insets.right-52)),alignSelf:'center'}} dismissable={!busy} dismissableBackButton={!busy} onDismiss={busy ? undefined : dismiss}>
        <ScrollView testID="beefiscal-confirm-message" tabIndex={0} role="region" aria-label={title} style={{maxHeight,flexShrink:1}} contentContainerStyle={{paddingBottom:8}} keyboardShouldPersistTaps="handled" nestedScrollEnabled>
        <Dialog.Title>{title}</Dialog.Title>
        <Dialog.Content><Text variant="bodyMedium">{message}</Text></Dialog.Content>
        <Dialog.Actions style={{ flexDirection: compact ? 'column' : 'row', flexWrap: 'wrap', gap: 12 }}>
          <Button labelNumberOfLines={0} testID="beefiscal-confirm-cancel" style={actionStyle} mode="outlined" contentStyle={{ minHeight: 48 }} labelStyle={wrap} disabled={busy} onPress={dismiss}>{cancelLabel}</Button>
          <Button labelNumberOfLines={0} testID="beefiscal-confirm-action" style={actionStyle} mode="contained" contentStyle={{ minHeight: 48 }} labelStyle={wrap} textColor={destructive ? theme.colors.onError : undefined} buttonColor={destructive ? theme.colors.error : undefined} accessibilityState={{ busy: Boolean(busy) }} icon={busy ? ({ size, color }) => <ActivityIndicator size={size} color={color} accessibilityLabel={confirmLabel} /> : undefined} disabled={busy} onPress={confirm}>{confirmLabel}</Button>
        </Dialog.Actions>
        </ScrollView>
      </Dialog>
      </View>
    </PaperDialogHost>
  );
}
