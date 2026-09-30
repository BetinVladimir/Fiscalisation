import React, { useState } from 'react';
import { View } from 'react-native';
import { Button, Card, Chip, HelperText, List, Text, TextInput, useTheme } from 'react-native-paper';
import { Action, Field, ReferencePicker, type Item, label } from '../components/FiscalControls';
import { isRegisterId } from '../registerFilter';

export type DeviceScreenProps = {
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

export function DevicesScreen({ devices, selected, loading, error, canDiagnose, canAdminister, canIssueBleSession, lists, context, telemetry, actions }: DeviceScreenProps) {
  const theme = useTheme();
  const [query, setQuery] = useState('');
  const registerCode = label(lists.registers.find(item => item.id === context.register)?.code || context.register);
  const visibleDevices = devices.filter(device =>
    [device.name, device.vendor, device.model, device.status, device.state]
      .some(value => label(value).toLowerCase().includes(query.trim().toLowerCase())));
  return <View testID="devices.screen" style={{ gap: 8, paddingBottom: 16 }}>
    {!loading && error ? <Card mode="outlined" testID="devices.error"><Card.Content style={{ gap: 8 }}>
      <HelperText type="error" visible accessibilityLiveRegion="polite">{error}</HelperText>
      <Button mode="outlined" contentStyle={{ minHeight: 48 }} onPress={actions.retry}>Опитай отново</Button>
    </Card.Content></Card> : null}
    {!loading && !error && devices.length === 0 ? <Text testID="devices.empty" variant="bodyMedium">Няма устройства за избрания контекст.</Text> : null}
    {!error && devices.length ? <TextInput testID="devices.search" mode="outlined" label="Търсене на устройство"
      accessibilityLabel="Търсене на устройство" value={query} onChangeText={setQuery} /> : null}
    {!loading && !error && devices.length > 0 && visibleDevices.length === 0 ? <Text testID="devices.no-matches" variant="bodyMedium">Няма устройства, които съответстват на търсенето.</Text> : null}
    {visibleDevices.map(device => <View key={label(device.id)}><List.Item testID={`device.item.${label(device.id)}`}
      accessibilityRole="button" accessibilityState={{ selected: selected?.id === device.id }}
      accessibilityLabel={`${label(device.vendor)} ${label(device.model)}, каса ${registerCode}, статус на регистрацията ${label(device.status || device.state)}`}
      style={{ borderRadius: 12, backgroundColor: selected?.id === device.id ? theme.colors.secondaryContainer : theme.colors.surface }}
      title={label(device.name || device.model || device.id)} titleNumberOfLines={2}
      description={`${label(device.vendor)} • ${label(device.model)} • ${label(device.connection_type || device.transport)} • ${registerCode}`}
      descriptionNumberOfLines={3}
      right={() => <Chip compact accessibilityLabel={`Статус на регистрацията ${label(device.status || device.state)}`}>{label(device.status || device.state)}</Chip>}
      onPress={() => actions.selectDevice(device)} />
      {selected?.id === device.id ? <Text testID="device-readiness" variant="bodyMedium" accessibilityLiveRegion="polite"
        style={{ color: theme.colors.onSurface, paddingHorizontal: 16, paddingVertical: 8 }}>
        Готовност: {device.status !== 'ACTIVE' ? 'Не е готово' : telemetry.deviceReadiness === null ? 'Проверява се' : telemetry.deviceReadiness.ready === true ? 'Готово' : 'Не е готово'}
      </Text> : null}
    </View>)}
    {!error ? <Card mode="outlined" testID="device-control-plane"><Card.Content style={{ gap: 12, paddingVertical: 16 }}>
      <Text variant="titleLarge" accessibilityRole="header">Provisioning, BLE и ограничена диагностика</Text>
      <Text variant="bodyMedium">Transport/cloud и достижимостта на крайното ФУ са отделни групи метрики.</Text>
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
        {canDiagnose ? <Action testID="device-diagnostics" label="Диагностика" disabled={loading || !selected} onPress={actions.diagnostics} /> : null}
        {canDiagnose ? <Action testID="device-printer-test" label="Тест на принтера" disabled={loading || !selected} onPress={actions.printerTest} /> : null}
        {canAdminister ? <Action testID="device-provision" label="Нова provisioning сесия" disabled={loading || !selected} onPress={actions.provisioning} /> : null}
      </View>
      {canIssueBleSession ? <>
        <ReferencePicker label="Оператор за BLE" items={lists.operators} value={context.operatorId}
          labelFor={item => `${label(item.operator_code)} ${label(item.first_name)} ${label(item.last_name)}`}
          onSelect={actions.selectOperator} onEmpty={actions.openAdministration} />
        <Field label="Подготвен X25519 client public key" value={context.blePublicKey}
          onChangeText={actions.changeBlePublicKey} testID="ble-client-public-key" />
        <Action testID="ble-session-issue" label="Издай BLE сесия"
          disabled={loading || !isRegisterId(context.register) || !context.operatorId || !context.blePublicKey.trim()}
          onPress={actions.issueBleSession} />
      </> : null}
      {canAdminister ? <>
        <Text variant="titleLarge" accessibilityRole="header">Активация на SmartDevice</Text>
        <Field label="QR / еднократен кратък код" value={context.activationCode}
          onChangeText={actions.changeActivationCode} testID="smart-device-activation-code" />
        <Action testID="smart-device-activation-lookup" label="Намери чакащото устройство"
          disabled={loading || !context.activationCode.trim()} onPress={actions.lookupActivation} />
        {context.activationRequest ? <Text testID="smart-device-activation-preview" variant="bodySmall">
          {JSON.stringify(context.activationRequest, null, 2)}
        </Text> : null}
        <ReferencePicker label="Търговска точка" items={lists.locations} value={context.activationLocationId}
          labelFor={item => label(item.name || item.code)} onSelect={actions.selectActivationLocation}
          onEmpty={actions.openAdministration} />
        <ReferencePicker label="Касово място" items={lists.registers} value={context.register}
          labelFor={item => label(item.code || item.name)} onSelect={actions.selectRegister}
          onEmpty={actions.openAdministration} />
        <Action testID="bluecash-activate" label="Потвърди и привържи устройството"
          disabled={loading || !context.activationRequest || !isRegisterId(context.activationLocationId) || !isRegisterId(context.register)}
          onPress={actions.confirmActivation} />
        <Action testID="smart-device-disconnect" label="Изключи избраното устройство от компанията"
          disabled={loading || !selected} onPress={actions.disconnectDevice} />
      </> : null}
      {telemetry.transportMetrics ? <Text testID="transport-metrics" variant="bodySmall">Transport: {JSON.stringify(telemetry.transportMetrics, null, 2)}</Text> : null}
      {telemetry.deviceHealth ? <Text testID="device-realtime-health" variant="bodySmall">Realtime MQTT→REST: {JSON.stringify(telemetry.deviceHealth, null, 2)}</Text> : null}
      {telemetry.deviceActivity.length ? <Text testID="device-activity-timeline" variant="bodySmall">Последни преходи: {JSON.stringify(telemetry.deviceActivity.slice(0, 20), null, 2)}</Text> : null}
      {telemetry.fiscalMetrics ? <Text testID="fiscal-device-metrics" variant="bodySmall">Крайно ФУ: {JSON.stringify(telemetry.fiscalMetrics, null, 2)}</Text> : null}
      {telemetry.diagnostics ? <Text testID="diagnostics-result" variant="bodySmall">{JSON.stringify(telemetry.diagnostics, null, 2)}</Text> : null}
      {telemetry.provisioning ? <Text testID="provisioning-result" variant="bodySmall">{JSON.stringify(telemetry.provisioning, null, 2)}</Text> : null}
      {telemetry.bleSession ? <Text testID="ble-session-result" variant="bodySmall">{JSON.stringify(telemetry.bleSession, null, 2)}</Text> : null}
      {telemetry.blueCashActivation ? <Text testID="bluecash-activation-result" variant="bodySmall">{JSON.stringify(telemetry.blueCashActivation, null, 2)}</Text> : null}
    </Card.Content></Card> : null}
  </View>;
}
