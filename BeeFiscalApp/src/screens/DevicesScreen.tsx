import {deviceScreenLabel} from '../ui/device-screen-copy';
import {foundationCopy} from '../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import {DeviceDataFields} from '../components/DeviceDataFields';
import {deviceDataCopyFor} from '../ui/device-data-copy';
import {useUiPreferences} from '../ui/preferences';
import { AppButton } from '../ui/app-button';
import { FormField } from '../ui/form-field';
import React, { useEffect, useId, useRef, useState } from 'react';
import { Platform, View } from 'react-native';
import { Card, Chip, HelperText, List, Text, useTheme } from 'react-native-paper';
import { Action, Field, ReferencePicker, type Item, label } from '../components/FiscalControls';
import { isRegisterId } from '../registerFilter';

export type DeviceScreenProps = {
  dataOwnerKey: string;
  devices: Item[];
  selected: Item | null;
  loading: boolean;
  error: string;
  canDiagnose: boolean;
  canAdminister: boolean;
  canIssueBleSession: boolean;
  lists: { operators: Item[]; locations: Item[]; registers: Item[] };
  context: {
    register: string;
    operatorId: string;
    blePublicKey: string;
    activationCode: string;
    activationRequest: Item | null;
    activationLocationId: string;
  };
  telemetry: {
    transportMetrics: Item | null;
    deviceHealth: Item | null;
    deviceReadiness: Item | null;
    deviceActivity: Item[];
    fiscalMetrics: Item | null;
    diagnostics: Item | null;
    provisioning: Item | null;
    bleSession: Item | null;
    blueCashActivation: Item | null;
  };
  actions: {
    retry: () => void;
    selectDevice: (device: Item) => void;
    openAdministration: () => void;
    diagnostics: () => void;
    printerTest: () => void;
    provisioning: () => void;
    selectOperator: (id: string) => void;
    changeBlePublicKey: (value: string) => void;
    issueBleSession: () => void;
    changeActivationCode: (value: string) => void;
    lookupActivation: () => void;
    selectActivationLocation: (id: string) => void;
    selectRegister: (id: string) => void;
    confirmActivation: () => void;
    disconnectDevice: () => void;
  };
};

export function DevicesScreen({ dataOwnerKey, devices, selected, loading, error, canDiagnose, canAdminister, canIssueBleSession, lists, context, telemetry, actions }: DeviceScreenProps) {
  const theme = useTheme();
  const {language}=useUiPreferences(),dataCopy=deviceDataCopyFor(language);
  const ui=(value:string)=>deviceScreenLabel(language,value);
  const errorId = `beefiscal-devices-error-${useId().replace(/:/g, '')}`;
  const [query, setQuery] = useState('');
  const mounted = useRef(false), snapshot = {}, latest = useRef(snapshot); latest.current = snapshot;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const current = () => mounted.current && latest.current === snapshot && !loading && !error;
  const registerCode = label(lists.registers.find(item => item.id === context.register)?.code || context.register);
  const visibleDevices = devices.filter(device =>
    [device.name, device.vendor, device.model, device.status, device.state]
      .some(value => label(value).toLowerCase().includes(query.trim().toLowerCase())));
  return <View testID="devices.screen" style={{ gap: 8, paddingBottom: 16 }}>
    {!loading && error ? <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID="devices.error"><Card.Content style={{ gap: 8 }}>
      <HelperText nativeID={errorId} accessibilityRole="alert" type="error" visible accessibilityLiveRegion="polite">{foundationCopy[language].error}</HelperText>
      <AppButton testID="devices.retry" disabled={loading} aria-describedby={Platform.OS === 'web' ? errorId : undefined} mode="outlined" contentStyle={{ minHeight: 48 }} onPress={actions.retry}>{ui("Опитай отново")}</AppButton>
    </Card.Content></Card> : null}
    {!loading && !error && devices.length === 0 ? <Text testID="devices.empty" variant="bodyMedium">{ui("Няма устройства за избрания контекст.")}</Text> : null}
    {!error && devices.length ? <FormField editable={!loading} testID="devices.search" mode="outlined" label={ui("Търсене на устройство")}
      accessibilityLabel={ui("Търсене на устройство")} value={query} onChangeText={next => { if (current()) setQuery(next); }} /> : null}
    {!loading && !error && devices.length > 0 && visibleDevices.length === 0 ? <Text testID="devices.no-matches" variant="bodyMedium">{ui("Няма устройства, които съответстват на търсенето.")}</Text> : null}
    {visibleDevices.map(device => <View key={label(device.id)}><List.Item disabled={loading || !!error} testID={`device.item.${label(device.id)}`}
      accessibilityRole="button" accessibilityState={{ selected: selected?.id === device.id }}
      accessibilityLabel={`${label(device.vendor)} ${label(device.model)}, ${ui('каса')} ${registerCode}, ${ui('Статус на регистрацията')} ${label(device.status || device.state)}`}
      style={{ minHeight: 48, borderWidth: 1, borderColor: theme.colors.outline, borderRadius: 12, backgroundColor: selected?.id === device.id ? theme.colors.secondaryContainer : theme.colors.surface }}
      title={label(device.name || device.model || device.id)} titleNumberOfLines={0} titleMaxFontSizeMultiplier={0}
      description={`${label(device.vendor)} • ${label(device.model)} • ${label(device.connection_type || device.transport)} • ${registerCode}`}
      descriptionNumberOfLines={0} descriptionMaxFontSizeMultiplier={0}
      right={() => <Chip compact accessibilityLabel={`${ui('Статус на регистрацията')} ${label(device.status || device.state)}`}>{label(device.status || device.state)}</Chip>}
      onPress={() => { if (current() && visibleDevices.includes(device)) actions.selectDevice(device); }} />
      {selected?.id === device.id ? <Text testID="device-readiness" variant="bodyMedium" accessibilityLiveRegion="polite"
        style={{ color: theme.colors.onSurface, paddingHorizontal: 16, paddingVertical: 8 }}>
        {ui('Готовност')}: {ui(device.status !== 'ACTIVE' ? 'Не е готово' : telemetry.deviceReadiness === null ? 'Проверява се' : telemetry.deviceReadiness.ready === true ? 'Готово' : 'Не е готово')}
      </Text> : null}
    </View>)}
    {!error ? <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID="device-control-plane"><Card.Content style={{ gap: 12, paddingVertical: 16 }}>
      <Text variant="titleLarge" accessibilityRole="header">{ui("Provisioning, BLE и ограничена диагностика")}</Text>
      <Text variant="bodyMedium">{ui("Transport/cloud и достижимостта на крайното ФУ са отделни групи метрики.")}</Text>
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
        {canDiagnose ? <Action testID="device-diagnostics" label={ui("Диагностика")} disabled={loading || !selected} onPress={actions.diagnostics} /> : null}
        {canDiagnose ? <Action testID="device-printer-test" label={ui("Тест на принтера")} disabled={loading || !selected} onPress={actions.printerTest} /> : null}
        {canAdminister ? <Action testID="device-provision" label={ui("Нова provisioning сесия")} disabled={loading || !selected} onPress={actions.provisioning} /> : null}
      </View>
      {canIssueBleSession ? <>
        <ReferencePicker disabled={loading} label={ui("Оператор за BLE")} items={lists.operators} value={context.operatorId}
          labelFor={item => `${label(item.operator_code)} ${label(item.first_name)} ${label(item.last_name)}`}
          onSelect={actions.selectOperator} onEmpty={actions.openAdministration} />
        <Field disabled={loading} label={ui("Подготвен X25519 client public key")} value={context.blePublicKey}
          onChangeText={actions.changeBlePublicKey} testID="ble-client-public-key" />
        <Action testID="ble-session-issue" label={ui("Издай BLE сесия")}
          disabled={loading || !isRegisterId(context.register) || !context.operatorId || !context.blePublicKey.trim()}
          onPress={actions.issueBleSession} />
      </> : null}
      {canAdminister ? <>
        <Text variant="titleLarge" accessibilityRole="header">{ui("Активация на SmartDevice")}</Text>
        <Field disabled={loading} label={ui("QR / еднократен кратък код")} value={context.activationCode}
          onChangeText={actions.changeActivationCode} testID="smart-device-activation-code" />
        <Action testID="smart-device-activation-lookup" label={ui("Намери чакащото устройство")}
          disabled={loading || !context.activationCode.trim()} onPress={actions.lookupActivation} />
        {context.activationRequest ? <DeviceDataFields testID="smart-device-activation-preview" ownerKey={dataOwnerKey} value={context.activationRequest} title={dataCopy['section.activationRequest']} copy={dataCopy}/> : null}
        <ReferencePicker disabled={loading} label={ui("Търговска точка")} items={lists.locations} value={context.activationLocationId}
          labelFor={item => label(item.name || item.code)} onSelect={actions.selectActivationLocation}
          onEmpty={actions.openAdministration} />
        <ReferencePicker disabled={loading} label={ui("Касово място")} items={lists.registers} value={context.register}
          labelFor={item => label(item.code || item.name)} onSelect={actions.selectRegister}
          onEmpty={actions.openAdministration} />
        <Action testID="bluecash-activate" label={ui("Потвърди и привържи устройството")}
          disabled={loading || !context.activationRequest || !isRegisterId(context.activationLocationId) || !isRegisterId(context.register)}
          onPress={actions.confirmActivation} />
        <Action testID="smart-device-disconnect" label={ui("Изключи избраното устройство от компанията")}
          disabled={loading || !selected} onPress={actions.disconnectDevice} />
      </> : null}
      {telemetry.transportMetrics ? <DeviceDataFields testID="transport-metrics" ownerKey={dataOwnerKey} value={telemetry.transportMetrics} title={dataCopy['section.transport']} copy={dataCopy}/> : null}
      {telemetry.deviceHealth ? <DeviceDataFields testID="device-realtime-health" ownerKey={dataOwnerKey} value={telemetry.deviceHealth} title={dataCopy['section.health']} copy={dataCopy}/> : null}
      {telemetry.deviceActivity.length ? <DeviceDataFields testID="device-activity-timeline" ownerKey={dataOwnerKey} value={telemetry.deviceActivity} title={dataCopy['section.activity']} copy={dataCopy}/> : null}
      {telemetry.fiscalMetrics ? <DeviceDataFields testID="fiscal-device-metrics" ownerKey={dataOwnerKey} value={telemetry.fiscalMetrics} title={dataCopy['section.fiscal']} copy={dataCopy}/> : null}
      {telemetry.diagnostics ? <DeviceDataFields testID="diagnostics-result" ownerKey={dataOwnerKey} value={telemetry.diagnostics} title={dataCopy['section.diagnostics']} copy={dataCopy}/> : null}
      {telemetry.provisioning ? <DeviceDataFields testID="provisioning-result" ownerKey={dataOwnerKey} value={telemetry.provisioning} title={dataCopy['section.provisioning']} copy={dataCopy}/> : null}
      {telemetry.bleSession ? <DeviceDataFields testID="ble-session-result" ownerKey={dataOwnerKey} value={telemetry.bleSession} title={dataCopy['section.ble']} copy={dataCopy}/> : null}
      {telemetry.blueCashActivation ? <DeviceDataFields testID="bluecash-activation-result" ownerKey={dataOwnerKey} value={telemetry.blueCashActivation} title={dataCopy['section.activation']} copy={dataCopy}/> : null}
    </Card.Content></Card> : null}
  </View>;
}
