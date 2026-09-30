import React from 'react';
import { Linking } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { StatusBar } from 'expo-status-bar';
import { Button, Card, HelperText, Text, TextInput, useTheme } from 'react-native-paper';
import { useEmailOtpAuth } from '../emailOtpAuth';
import { AuthForm } from '../ui/auth-form';
import { ResponsiveScrollView as ScrollView } from '../ui/responsive-scroll-view';
import { LanguageThemeControls } from '../ui/LanguageThemeControls';
import { useUiPreferences } from '../ui/preferences';
import { authCopy } from '../ui/auth-copy.generated';

type AuthState = ReturnType<typeof useEmailOtpAuth>;

export function AuthScreen({ auth }: { auth: AuthState }) {
  const theme = useTheme();
  const { language } = useUiPreferences();
  const copy = authCopy[language];
  const stage = !auth.temporary ? 'email' : auth.tenants.length === 0 ? 'otp' : 'company';

  return (
    <SafeAreaView testID="auth.root" style={{ flex: 1, backgroundColor: theme.colors.background }}>
      <StatusBar style={theme.dark ? 'light' : 'dark'} />
      <LanguageThemeControls />
      <ScrollView contentContainerStyle={{ flexGrow: 1, justifyContent: 'center', paddingVertical: 16 }} keyboardShouldPersistTaps="handled">
        <AuthForm testID="admin-login" style={{ gap: 12 }}>
          <Card mode="outlined">
            <Card.Content style={{ gap: 16, paddingVertical: 24 }}>
              <Text variant="headlineSmall" accessibilityRole="header">{copy.title}</Text>
              <Text variant="bodyMedium">{copy.description}</Text>

              {auth.error ? <HelperText type="error" visible accessibilityLiveRegion="polite">{auth.error}</HelperText> : null}

              {stage === 'email' ? <>
                <TextInput
                  testID="admin-login-email"
                  mode="outlined"
                  label={copy.email}
                  accessibilityLabel={copy.email}
                  autoCapitalize="none"
                  autoComplete="email"
                  keyboardType="email-address"
                  value={auth.email}
                  onChangeText={auth.setEmail}
                />
                <Button testID="admin-login-start" mode="contained" contentStyle={{ minHeight: 48 }} disabled={!auth.ready || !auth.email.trim()} onPress={() => void auth.requestCode()}>{copy.requestCode}</Button>
              </> : stage === 'otp' ? <>
                <TextInput
                  testID="admin-login-code"
                  mode="outlined"
                  label={copy.code}
                  accessibilityLabel={copy.code}
                  keyboardType="number-pad"
                  textContentType="oneTimeCode"
                  maxLength={6}
                  value={auth.code}
                  onChangeText={auth.setCode}
                />
                <Button testID="admin-login-verify" mode="contained" contentStyle={{ minHeight: 48 }} disabled={!auth.ready || auth.code.length !== 6} onPress={() => void auth.verifyCode()}>{copy.verify}</Button>
              </> : <>
                <Text variant="titleMedium" accessibilityRole="header">{copy.chooseCompany}</Text>
                {auth.tenants.map(tenant => (
                  <Button key={tenant.tenant_id} mode="outlined" contentStyle={{ minHeight: 48 }} accessibilityLabel={`${copy.chooseCompanyPrefix} ${tenant.display_name}`} disabled={!auth.ready} onPress={() => void auth.selectTenant(tenant.tenant_id)}>{tenant.display_name}</Button>
                ))}
              </>}

              <Button mode="text" accessibilityRole="link" onPress={() => void Linking.openURL('https://legal.beeloy.org/bg/privacy')}>{copy.privacyPolicy}</Button>
            </Card.Content>
          </Card>
        </AuthForm>
      </ScrollView>
    </SafeAreaView>
  );
}
