import React from 'react';
import { StyleSheet, View } from 'react-native';
import { HelperText, Text as PaperText, TextInput, useTheme } from 'react-native-paper';
import { Action, AdminCard, Choices, Field, ReferencePicker, type Item, label } from '../components/FiscalControls';
import { MVP1_DEVICE_PROFILES, type DeviceProfile } from '../deviceProfiles';
import { isRegisterId, registerResourcePath } from '../registerFilter';

type AdminStringKey =
  | 'locationCode'
  | 'locationName'
  | 'locationAddress'
  | 'registerCode'
  | 'locationId'
  | 'operatorCode'
  | 'operatorId'
  | 'operatorFirstName'
  | 'operatorLastName'
  | 'deviceVendor'
  | 'deviceModel'
  | 'deviceSerial'
  | 'deviceKind'
  | 'deviceDriverId'
  | 'deviceProtocolId'
  | 'deviceProtocolVersion'
  | 'deviceTransport'
  | 'deviceTransportParameters'
  | 'bindingRole'
  | 'deviceId'
  | 'adapterDeviceId'
  | 'fiscalEndpointId'
  | 'paymentEndpointId'
  | 'expectedRegisterVersion'
  | 'register';
type AdminStrings = Record<AdminStringKey, string> & {
  [K in AdminStringKey as `set${Capitalize<K>}`]: (value: string) => void;
};
export type AdministrationScreenProps = AdminStrings & {
  adminLists: { locations: Item[]; registers: Item[]; operators: Item[]; devices: Item[] };
  adminBusy: boolean;
  compositeBinding: Item | null;
  mutateAdmin: (description: string, action: () => Promise<unknown>) => Promise<void>;
  stageCompositeBinding: () => Promise<void>;
  disableCompositeBinding: () => Promise<void>;
  transportParameter: (name: string, fallback?: string) => string;
  setTransportParameter: (name: string, value: string, numeric?: boolean) => void;
  call: (path: string, init?: RequestInit) => Promise<Item>;
  setCompositeProfile: (value: DeviceProfile['id']) => void;
  setMessage: (value: string) => void;
};

const s = StyleSheet.create({
  adminGrid: { flexDirection: 'row', flexWrap: 'wrap', gap: 14, paddingBottom: 24 },
  actions: { flexDirection: 'row', gap: 10, marginTop: 20, flexWrap: 'wrap' },
});

export function AdministrationScreen(props: AdministrationScreenProps) {
  const theme = useTheme();
  const {
    adminLists,
    adminBusy,
    compositeBinding,
    mutateAdmin,
    stageCompositeBinding,
    disableCompositeBinding,
    transportParameter,
    setTransportParameter,
    call,
    locationCode,
    locationName,
    locationAddress,
    registerCode,
    locationId,
    operatorCode,
    operatorId,
    operatorFirstName,
    operatorLastName,
    deviceVendor,
    deviceModel,
    deviceSerial,
    deviceKind,
    deviceDriverId,
    deviceProtocolId,
    deviceProtocolVersion,
    deviceTransport,
    deviceTransportParameters,
    bindingRole,
    deviceId,
    adapterDeviceId,
    fiscalEndpointId,
    paymentEndpointId,
    expectedRegisterVersion,
    register,
    setLocationCode,
    setLocationName,
    setLocationAddress,
    setRegisterCode,
    setLocationId,
    setOperatorCode,
    setOperatorId,
    setOperatorFirstName,
    setOperatorLastName,
    setDeviceVendor,
    setDeviceModel,
    setDeviceSerial,
    setDeviceKind,
    setDeviceDriverId,
    setDeviceProtocolId,
    setDeviceProtocolVersion,
    setDeviceTransport,
    setDeviceTransportParameters,
    setBindingRole,
    setDeviceId,
    setAdapterDeviceId,
    setFiscalEndpointId,
    setPaymentEndpointId,
    setExpectedRegisterVersion,
    setRegister,
    setCompositeProfile,
    setMessage,
  } = props;
  const transportJsonValid = (() => {
    try {
      const value = JSON.parse(deviceTransportParameters || '{}');
      return value !== null && typeof value === 'object' && !Array.isArray(value);
    } catch {
      return false;
    }
  })();
  return (
    <View testID="admin-editors" style={s.adminGrid}>
      <AdminCard
        title="Точка на продажба"
        count={adminLists.locations.length}
      >
        <Choices
          items={adminLists.locations}
          selectedId={locationId}
          labelFor={(x) => `${label(x.code)} • ${label(x.name)}`}
          onSelect={(x) => {
            setLocationId(label(x.id));
            setLocationCode(label(x.code));
            setLocationName(label(x.name));
            setLocationAddress(label(x.address));
          }}
        />
        <Field
          label="Код на точката"
          value={locationCode}
          onChangeText={setLocationCode}
          testID="admin-location-code"
        />
        <Field
          label="Име на точката"
          value={locationName}
          onChangeText={setLocationName}
        />
        <Field
          label="Адрес"
          value={locationAddress}
          onChangeText={setLocationAddress}
        />
        <Action
          label="Запази точка"
          disabled={adminBusy}
          testID="admin-location-save"
          onPress={() =>
            mutateAdmin("Запис на точка", async () => {
              const current = adminLists.locations.find(
                (x) => x.id === locationId,
              );
              const v = await call(
                current
                  ? `/locations/${encodeURIComponent(locationId)}`
                  : "/locations",
                {
                  method: current ? "PATCH" : "POST",
                  headers: current
                    ? { "If-Match": String(current.version) }
                    : {},
                  body: JSON.stringify({
                    code: locationCode,
                    name: locationName,
                    address: locationAddress,
                    status: "ACTIVE",
                  }),
                },
              );
              setLocationId(label(v.id));
            })
          }
        />
      </AdminCard>
      <AdminCard
        title="Касово място"
        count={adminLists.registers.length}
      >
        <Choices
          items={adminLists.registers}
          selectedId={register}
          labelFor={(x) => label(x.code)}
          onSelect={(x) => {
            setRegister(label(x.id));
            setRegisterCode(label(x.code));
            setLocationId(label(x.location_id));
          }}
        />
        <ReferencePicker label="Търговска точка" items={adminLists.locations} value={locationId} labelFor={(x)=>label(x.name || x.code)} onSelect={setLocationId} onEmpty={()=>setMessage("Първо създайте търговска точка в картата по-горе.")}/>
        <Field
          label="Код на касата"
          value={registerCode}
          onChangeText={setRegisterCode}
        />
        <Action
          label="Запази каса"
          disabled={adminBusy}
          testID="admin-register-save"
          onPress={() =>
            mutateAdmin("Запис на каса", async () => {
              const current = adminLists.registers.find(
                (x) => x.id === register,
              );
              const v = await call(
                current ? registerResourcePath(register) : "/registers",
                {
                  method: current ? "PATCH" : "POST",
                  headers: current
                    ? { "If-Match": String(current.version) }
                    : {},
                  body: JSON.stringify({
                    location_id: locationId,
                    code: registerCode,
                    status: "ACTIVE",
                    fiscal_device_id: current?.fiscal_device_id,
                    payment_terminal_id: current?.payment_terminal_id,
                  }),
                },
              );
              setRegister(label(v.id));
            })
          }
        />
      </AdminCard>
      <AdminCard
        title="Фискален оператор"
        count={adminLists.operators.length}
      >
        <Choices
          items={adminLists.operators}
          selectedId={operatorId}
          labelFor={(x) =>
            `${label(x.code)} • ${label(x.first_name)} ${label(x.last_name)}`
          }
          onSelect={(x) => {
            setOperatorId(label(x.id));
            setOperatorCode(label(x.code));
            setOperatorFirstName(label(x.first_name));
            setOperatorLastName(label(x.last_name));
          }}
        />
        <Field
          label="Код (4 символа)"
          value={operatorCode}
          onChangeText={setOperatorCode}
          testID="admin-operator-code"
        />
        <Field
          label="Име"
          value={operatorFirstName}
          onChangeText={setOperatorFirstName}
        />
        <Field
          label="Фамилия"
          value={operatorLastName}
          onChangeText={setOperatorLastName}
        />
        <Action
          label="Запази оператор"
          disabled={adminBusy}
          testID="admin-operator-save"
          onPress={() =>
            mutateAdmin("Запис на оператор", async () => {
              const current = adminLists.operators.find(
                (x) => x.id === operatorId,
              );
              const v = await call(
                current
                  ? `/operators/${encodeURIComponent(operatorId)}`
                  : "/operators",
                {
                  method: current ? "PATCH" : "POST",
                  headers: current
                    ? { "If-Match": String(current.version) }
                    : {},
                  body: JSON.stringify({
                    code: operatorCode,
                    first_name: operatorFirstName,
                    last_name: operatorLastName,
                    roles: current?.roles || ["CASHIER"],
                    active_from:
                      current?.active_from || new Date().toISOString(),
                    active_to: current?.active_to || null,
                  }),
                },
              );
              setOperatorId(label(v.id));
            })
          }
        />
      </AdminCard>
      <AdminCard
        title="Фискално устройство"
        count={adminLists.devices.length}
      >
        <Choices
          items={adminLists.devices}
          selectedId={deviceId}
          labelFor={(x) =>
            `${label(x.vendor)} ${label(x.model)} • ${label(x.serial)}`
          }
          onSelect={(x) => {
            setDeviceId(label(x.id));
            setDeviceVendor(label(x.vendor));
            setDeviceModel(label(x.model));
            setDeviceSerial(label(x.serial));
            setDeviceKind(label(x.kind));
            setDeviceDriverId(label(x.driver_id || "datecs"));
            setDeviceProtocolId(label(x.protocol_id || "datecs-fp"));
            setDeviceProtocolVersion(
              label(x.protocol_version || "2.11.4"),
            );
            setDeviceTransport(label(x.transport || "RS232"));
            setDeviceTransportParameters(
              JSON.stringify(x.transport_parameters || {}, null, 2),
            );
            setBindingRole(
              label(x.kind) === "PAYMENT_TERMINAL"
                ? "OPTIONAL_PAYMENT_TERMINAL"
                : "FISCAL_DEVICE",
            );
          }}
        />
        <View style={s.actions}>
          <Action
            label="Фискално устройство"
            disabled={adminBusy}
            testID="admin-device-kind-fiscal"
            onPress={() => {
              setDeviceKind("FISCAL_DEVICE");
              setBindingRole("FISCAL_DEVICE");
            }}
          />
          <Action
            label="Опционален POS терминал"
            disabled={adminBusy}
            testID="admin-device-kind-terminal"
            onPress={() => {
              setDeviceKind("PAYMENT_TERMINAL");
              setBindingRole("OPTIONAL_PAYMENT_TERMINAL");
            }}
          />
        </View>
        <PaperText variant="bodyMedium" style={{ color: theme.colors.onSurfaceVariant }}>
          Capability: {deviceKind} • binding: {bindingRole}
        </PaperText>
        <Field
          label="Производител"
          value={deviceVendor}
          onChangeText={setDeviceVendor}
        />
        <Field
          label="Модел"
          value={deviceModel}
          onChangeText={setDeviceModel}
        />
        <Field
          label="Сериен номер"
          value={deviceSerial}
          onChangeText={setDeviceSerial}
          testID="admin-device-serial"
        />
        <Field
          label="Driver ID"
          value={deviceDriverId}
          onChangeText={setDeviceDriverId}
          testID="admin-device-driver"
        />
        <Field
          label="Protocol ID"
          value={deviceProtocolId}
          onChangeText={setDeviceProtocolId}
          testID="admin-device-protocol"
        />
        <Field
          label="Protocol version"
          value={deviceProtocolVersion}
          onChangeText={setDeviceProtocolVersion}
          testID="admin-device-protocol-version"
        />
        <View style={s.actions}>
          <Action
            testID="transport-rs232"
            label="RS-232"
            disabled={adminBusy}
            onPress={() => {
              setDeviceTransport("RS232");
              setDeviceTransportParameters(
                '{"baud":115200,"data_bits":8,"parity":"N","stop_bits":1,"tx_pin":17,"rx_pin":18}',
              );
            }}
          />
          <Action
            testID="transport-usb"
            label="USB CDC"
            disabled={adminBusy}
            onPress={() => {
              setDeviceTransport("USB_SERIAL");
              setDeviceTransportParameters(
                '{"vid":0,"pid":0,"interface":0}',
              );
            }}
          />
          <Action
            testID="transport-ble"
            label="BLE GATT"
            disabled={adminBusy}
            onPress={() => {
              setDeviceTransport("BLE_GATT");
              setDeviceTransportParameters(
                '{"address_policy":"ADVERTISING_IDENTITY"}',
              );
            }}
          />
        </View>
        <PaperText variant="bodyMedium" style={{ color: theme.colors.onSurfaceVariant }}>Избран transport: {deviceTransport}</PaperText>
        {deviceTransport === "RS232" ? (
          <>
            <Field
              label="Baud"
              value={transportParameter("baud", "115200")}
              onChangeText={(v) =>
                setTransportParameter("baud", v, true)
              }
            />
            <Field
              label="Data bits"
              value={transportParameter("data_bits", "8")}
              onChangeText={(v) =>
                setTransportParameter("data_bits", v, true)
              }
            />
            <Field
              label="Parity (N/E/O)"
              value={transportParameter("parity", "N")}
              onChangeText={(v) => setTransportParameter("parity", v)}
            />
            <Field
              label="Stop bits"
              value={transportParameter("stop_bits", "1")}
              onChangeText={(v) =>
                setTransportParameter("stop_bits", v, true)
              }
            />
            <Field
              label="TX pin"
              value={transportParameter("tx_pin", "17")}
              onChangeText={(v) =>
                setTransportParameter("tx_pin", v, true)
              }
            />
            <Field
              label="RX pin"
              value={transportParameter("rx_pin", "18")}
              onChangeText={(v) =>
                setTransportParameter("rx_pin", v, true)
              }
            />
          </>
        ) : null}
        {deviceTransport === "USB_SERIAL" ? (
          <>
            <Field
              label="USB VID"
              value={transportParameter("vid", "0")}
              onChangeText={(v) =>
                setTransportParameter("vid", v, true)
              }
            />
            <Field
              label="USB PID"
              value={transportParameter("pid", "0")}
              onChangeText={(v) =>
                setTransportParameter("pid", v, true)
              }
            />
            <Field
              label="CDC interface"
              value={transportParameter("interface", "0")}
              onChangeText={(v) =>
                setTransportParameter("interface", v, true)
              }
            />
            <Field
              label="Очакван USB serial"
              value={transportParameter("serial", "")}
              onChangeText={(v) => setTransportParameter("serial", v)}
            />
          </>
        ) : null}
        {deviceTransport === "BLE_GATT" ? (
          <>
            <Field
              label="Advertising identity"
              value={transportParameter("advertising_identity", "")}
              onChangeText={(v) =>
                setTransportParameter("advertising_identity", v)
              }
            />
            <Field
              label="Service UUID"
              value={transportParameter("service_uuid", "")}
              onChangeText={(v) =>
                setTransportParameter("service_uuid", v)
              }
            />
            <Field
              label="TX characteristic UUID"
              value={transportParameter("tx_characteristic_uuid", "")}
              onChangeText={(v) =>
                setTransportParameter("tx_characteristic_uuid", v)
              }
            />
            <Field
              label="RX characteristic UUID"
              value={transportParameter("rx_characteristic_uuid", "")}
              onChangeText={(v) =>
                setTransportParameter("rx_characteristic_uuid", v)
              }
            />
          </>
        ) : null}
        <TextInput
          mode="outlined"
          label="Transport JSON"
          accessibilityLabel="Transport JSON"
          value={deviceTransportParameters}
          onChangeText={setDeviceTransportParameters}
          error={!transportJsonValid}
          multiline
          numberOfLines={3}
          style={{ width: '100%', minWidth: 0 }}
          testID="admin-device-transport-parameters"
        />
        <HelperText type={transportJsonValid ? 'info' : 'error'} visible>
          {transportJsonValid ? 'Разширени транспортни параметри в JSON формат.' : 'Въведете валиден JSON обект.'}
        </HelperText>
        <Action
          label="Запази DEV ФУ"
          disabled={adminBusy || !transportJsonValid}
          testID="admin-device-save"
          onPress={() =>
            mutateAdmin("Запис на ФУ", async () => {
              const current = adminLists.devices.find(
                (x) => x.id === deviceId,
              );
              const v = await call(
                current
                  ? `/devices/${encodeURIComponent(deviceId)}`
                  : "/devices",
                {
                  method: current ? "PATCH" : "POST",
                  headers: current
                    ? { "If-Match": String(current.version) }
                    : {},
                  body: JSON.stringify({
                    kind: deviceKind,
                    vendor: deviceVendor,
                    model: deviceModel,
                    serial: deviceSerial,
                    driver_id: deviceDriverId,
                    protocol_id: deviceProtocolId,
                    protocol_version: deviceProtocolVersion,
                    transport: deviceTransport,
                    transport_parameters: JSON.parse(
                      deviceTransportParameters || "{}",
                    ),
                    status: current?.status || "DRAFT",
                    environment: current?.environment || "DEV",
                    simulated: current?.simulated ?? true,
                    approved_type_evidence_id:
                      current?.approved_type_evidence_id,
                    service_contract_evidence_id:
                      current?.service_contract_evidence_id,
                  }),
                },
              );
              setDeviceId(label(v.id));
            })
          }
        />
        {(() => {
          const current = adminLists.devices.find(
            (x) => x.id === deviceId,
          );
          const target =
            current?.status === "DRAFT"
              ? "PENDING_SERVICE_ACTIVATION"
              : current?.status === "PENDING_SERVICE_ACTIVATION"
                ? "ACTIVE"
                : "";
          if (!current || !target) return null;
          return (
            <Action
              label={
                target === "ACTIVE"
                  ? "Активирай ФУ"
                  : "Предай към сервизна активация"
              }
              disabled={adminBusy}
              testID="admin-device-activate"
              onPress={() =>
                mutateAdmin("Промяна на lifecycle", () =>
                  call(`/devices/${encodeURIComponent(deviceId)}`, {
                    method: "PATCH",
                    headers: { "If-Match": String(current.version) },
                    body: JSON.stringify({
                      ...current,
                      status: target,
                      id: undefined,
                      version: undefined,
                      created_at: undefined,
                      updated_at: undefined,
                    }),
                  }),
                )
              }
            />
          );
        })()}
      </AdminCard>
      <AdminCard title="Привързване към касата" count={0}>
        <ReferencePicker label="Касово място" items={adminLists.registers} value={register} labelFor={(x)=>label(x.code || x.name)} onSelect={setRegister} onEmpty={()=>setMessage("Първо създайте касово място.")}/>
        <View style={s.actions}>
          <Action
            label="Роля ФУ"
            disabled={adminBusy}
            testID="admin-binding-role-fiscal"
            onPress={() => setBindingRole("FISCAL_DEVICE")}
          />
          <Action
            label="Роля POS"
            disabled={adminBusy}
            testID="admin-binding-role-terminal"
            onPress={() => setBindingRole("OPTIONAL_PAYMENT_TERMINAL")}
          />
        </View>
        <ReferencePicker label="Устройство" items={adminLists.devices} value={deviceId} labelFor={(x)=>`${label(x.vendor)} ${label(x.model)} · ${label(x.serial)}`} onSelect={setDeviceId} onEmpty={()=>setMessage("Първо създайте устройство.")}/>
        <Action
          label="Привържи ФУ"
          disabled={adminBusy || !isRegisterId(register)}
          testID="admin-binding-save"
          onPress={() =>
            mutateAdmin("Привързване на ФУ", () =>
              call(registerResourcePath(register, "/bindings"), {
                method: "POST",
                body: JSON.stringify({
                  device_id: deviceId,
                  role: bindingRole,
                }),
              }),
            )
          }
        />
        <PaperText variant="bodyMedium" style={{ color: theme.colors.onSurfaceVariant }}>
          Composite profile — активира се само след apply ACK от
          адаптера
        </PaperText>
        <View style={s.actions}>
          {MVP1_DEVICE_PROFILES.map((profile) => (
            <Action
              key={profile.id}
              label={profile.id}
              disabled={adminBusy}
              testID={`composite-profile-${profile.id}`}
              onPress={() => setCompositeProfile(profile.id)}
            />
          ))}
        </View>
        <ReferencePicker label="Адаптер" items={adminLists.devices} value={adapterDeviceId} labelFor={(x)=>`${label(x.vendor)} ${label(x.model)} · ${label(x.serial)}`} onSelect={setAdapterDeviceId} onEmpty={()=>setMessage("Първо създайте устройство-адаптер.")}/>
        <ReferencePicker label="Фискално устройство" items={adminLists.devices.filter((x)=>x.kind === "FISCAL_DEVICE")} value={fiscalEndpointId} labelFor={(x)=>`${label(x.vendor)} ${label(x.model)} · ${label(x.serial)}`} onSelect={setFiscalEndpointId} onEmpty={()=>setMessage("Първо създайте фискално устройство.")}/>
        <ReferencePicker label="Платежен терминал (незадължително)" items={adminLists.devices.filter((x)=>x.kind === "PAYMENT_TERMINAL")} value={paymentEndpointId} labelFor={(x)=>`${label(x.vendor)} ${label(x.model)} · ${label(x.serial)}`} onSelect={setPaymentEndpointId} required={false}/>
        <Field
          label="Expected register version"
          value={expectedRegisterVersion}
          onChangeText={setExpectedRegisterVersion}
          testID="composite-register-version"
        />
        <Action
          label="Подготви composite binding"
          disabled={adminBusy || !isRegisterId(register)}
          testID="composite-binding-save"
          onPress={stageCompositeBinding}
        />
        {compositeBinding ? (
          <>
            <PaperText testID="composite-binding-state" variant="bodyMedium" style={{ color: theme.colors.onSurfaceVariant }}>
              Статус: {label(compositeBinding.status)} • generation{" "}
              {label(compositeBinding.generation)} • applied generation{" "}
              {label(compositeBinding.applied_generation || "—")}
            </PaperText>
            {compositeBinding.status === "PENDING" ? (
              <PaperText
                testID="composite-binding-awaiting-adapter"
                variant="bodyMedium"
                style={{ color: theme.colors.onSurfaceVariant }}
              >
                Очаква се подписано apply потвърждение от адаптера
              </PaperText>
            ) : null}
            <View style={s.actions}>
              <Action
                label="Изключи / rebind"
                disabled={
                  adminBusy || compositeBinding.status === "REVOKED"
                }
                testID="composite-binding-disable"
                onPress={disableCompositeBinding}
              />
            </View>
          </>
        ) : null}
      </AdminCard>
    </View>
  );
}
