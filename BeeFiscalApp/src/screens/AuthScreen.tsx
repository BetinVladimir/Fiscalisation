import {Surface as SafePaperSurface} from 'react-native-paper';
import { AppButton } from '../ui/app-button';
import { FormField } from '../ui/form-field';
import React, { useEffect, useId, useRef, useState } from 'react';
import { Linking, Platform } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { StatusBar } from 'expo-status-bar';
import { Card, HelperText, Text, useTheme } from 'react-native-paper';
import { useEmailOtpAuth } from '../emailOtpAuth';
import { AuthForm } from '../ui/auth-form';
import { ResponsiveScrollView as ScrollView } from '../ui/responsive-scroll-view';
import { LanguageThemeControls } from '../ui/LanguageThemeControls';
import { useUiPreferences } from '../ui/preferences';
import { authCopy } from '../ui/auth-copy.generated';
import { foundationCopy } from '../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import { privacyPolicyUrl } from '../ui/legal';

type AuthState = ReturnType<typeof useEmailOtpAuth>;

export function AuthScreen({ auth }: { auth: AuthState }) {
  const theme = useTheme();
  const errorId = `beefiscal-auth-error-${useId().replace(/:/g, '')}`;
  const { language } = useUiPreferences();
  const copy = authCopy[language];
  const linkLabelStyle = Platform.OS === 'web' ? { whiteSpace: 'normal', overflowWrap: 'anywhere', flexShrink: 1 } as const : { flexShrink: 1 };
  const stage = !auth.temporary ? 'email' : auth.tenants.length === 0 ? 'otp' : 'company';
  const mounted = useRef(false), snapshot = {}, latest = useRef(snapshot); latest.current = snapshot;
  const ownerKey = JSON.stringify([language, stage, auth.temporary, auth.currentTenant?.tenant_id]);
  const ownerRef = useRef({ key: ownerKey });
  if (ownerRef.current.key !== ownerKey) ownerRef.current = { key: ownerKey };
  const owner = ownerRef.current;
  const pending = useRef<{ ticket: object; owner: object } | null>(null);
  const [privacyBusy, setPrivacyBusy] = useState<object | null>(null);
  const [privacyError, setPrivacyError] = useState<object | null>(null);
  const privacyErrorId = `${errorId}-privacy`;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; pending.current = null; }; }, []);
  const openPrivacy = async () => {
    if (!mounted.current || latest.current !== snapshot || privacyBusy === owner || pending.current?.owner === owner) return;
    const ticket = {}; pending.current = { ticket, owner }; setPrivacyBusy(owner); setPrivacyError(null);
    const live = () => mounted.current && ownerRef.current === owner && pending.current?.ticket === ticket;
    try { await Linking.openURL(privacyPolicyUrl(language)); }
    catch { if (live()) setPrivacyError(owner); }
    finally { if (pending.current?.ticket === ticket) { pending.current = null; if (mounted.current && ownerRef.current === owner) setPrivacyBusy(null); } }
  };


  return (
    <SafePaperSurface testID="auth.surface" mode="flat" elevation={0} style={{flex:1,backgroundColor:theme.colors.background}}><SafeAreaView testID="auth.root" style={{ flex: 1 }}>
      <StatusBar style={theme.dark ? 'light' : 'dark'} />
      <LanguageThemeControls />
      <ScrollView contentContainerStyle={{ flexGrow: 1, justifyContent: 'center', paddingVertical: 16 }} keyboardShouldPersistTaps="handled">
        <AuthForm testID="admin-login" style={{ gap: 12 }}>
          <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }}>
            <Card.Content style={{ gap: 16, paddingVertical: 24 }}>
              <Text variant="headlineSmall" accessibilityRole="header">{copy.title}</Text>
              <Text variant="bodyMedium">{copy.description}</Text>

              {auth.error ? <HelperText testID="auth.error" nativeID={errorId} type="error" visible accessibilityRole="alert" accessibilityLiveRegion="polite">{foundationCopy[language].error}</HelperText> : null}

              {stage === 'email' ? <>
                <FormField
                  testID="admin-login-email"
                  editable={auth.ready}
                  aria-describedby={Platform.OS === 'web' && auth.error ? errorId : undefined}
                  mode="outlined"
                  label={copy.email}
                  accessibilityLabel={copy.email}
                  autoCapitalize="none"
                  autoComplete="email"
                  keyboardType="email-address"
                  value={auth.email}
                  onChangeText={auth.setEmail}
                  onSubmitEditing={() => { if (auth.ready) void auth.requestCode(); }}
                />
                <AppButton testID="admin-login-start" mode="contained" contentStyle={{ minHeight: 48 }} disabled={!auth.ready || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(auth.email.trim())} onPress={() => void auth.requestCode()}>{copy.requestCode}</AppButton>
              </> : stage === 'otp' ? <>
                <FormField
                  testID="admin-login-code"
                  editable={auth.ready}
                  aria-describedby={Platform.OS === 'web' && auth.error ? errorId : undefined}
                  mode="outlined"
                  label={copy.code}
                  accessibilityLabel={copy.code}
                  keyboardType="number-pad"
                  textContentType="oneTimeCode"
                  autoComplete="one-time-code"
                  maxLength={6}
                  value={auth.code}
                  onChangeText={auth.setCode}
                  onSubmitEditing={() => { if (auth.ready) void auth.verifyCode(); }}
                />
                <AppButton testID="admin-login-verify" mode="contained" contentStyle={{ minHeight: 48 }} disabled={!auth.ready || !/^\d{6}$/.test(auth.code)} onPress={() => void auth.verifyCode()}>{copy.verify}</AppButton>
              </> : <>
                <Text variant="titleMedium" accessibilityRole="header">{copy.chooseCompany}</Text>
                {auth.tenants.map(tenant => (
                  <AppButton key={tenant.tenant_id} mode="outlined" contentStyle={{ minHeight: 48 }} accessibilityLabel={`${copy.chooseCompanyPrefix} ${tenant.display_name}`} disabled={!auth.ready} onPress={() => void auth.selectTenant(tenant.tenant_id)}>{tenant.display_name}</AppButton>
                ))}
              </>}

              <AppButton loading={privacyBusy === owner} disabled={privacyBusy === owner} aria-describedby={Platform.OS === 'web' && privacyError === owner ? privacyErrorId : undefined} testID="auth.privacy" mode="outlined" contentStyle={{ minHeight: 48 }} labelStyle={linkLabelStyle} accessibilityRole="link" onPress={() => void openPrivacy()}>{copy.privacyPolicy}</AppButton>
              {privacyError === owner ? <HelperText testID="auth.privacy.error" nativeID={privacyErrorId} type="error" visible accessibilityRole="alert" accessibilityLiveRegion="polite">{foundationCopy[language].error}</HelperText> : null}
            </Card.Content>
          </Card>
        </AuthForm>
      </ScrollView>
    </SafeAreaView></SafePaperSurface>
  );
}
