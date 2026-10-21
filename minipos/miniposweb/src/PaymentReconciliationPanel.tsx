import { useEffect, useLayoutEffect, useRef, useState } from "react";
import {AccessiblePaperRadioItem} from "./ui/accessible-paper-radio-item";
import { Card, HelperText, List, RadioButton, Text } from "react-native-paper";
import { View } from "react-native";
import { AppButton as Button } from "./ui/app-button";
import { FormField } from "./ui/form-field";
import {reconciliationCopyFor,type ReconciliationCopy} from "./ui/reconciliation-copy";
import { ApiError } from "./api/client";
import { canonical } from "./fiscal/goCanonical";
import { unbase64url } from "./fiscal/executionAuthorization";
import {
  confirmedReconciliation,
  dispatchReconciliation,
  inspectReconciliation,
  pendingReconciliations,
  stageReconciliation,
  revokeReconciliationScope,
  type ReconciliationCommand,
  type ReconciliationDecision,
  type ReconciliationGrant,
  type ReconciliationRoute,
  type ReconciliationScope,
} from "./fiscal/paymentReconciliation";
import type { HeaderDraft } from "./ui/useHeaderLeaveGuard";
import type { FiscalOperation } from "./types";
export type ReconciliationPanelProps = {
  language?: string;
  canEditHeader?: () => boolean;
  reportHeaderPending?: (pending: boolean) => void;
  registerHeaderDraft?: (draft: HeaderDraft) => () => void;
  administrator: boolean;
  scope: ReconciliationScope;
  authorize: (
    decision: ReconciliationDecision,
    key: string,
  ) => Promise<ReconciliationCommand>;
  routes: ReconciliationRoute[];
  assertActive: () => Promise<void>;
};
const blank: ReconciliationDecision = {
  operation_id: "",
  payment_id: "",
  observed_state: "CARD_APPROVED",
  evidence_reference: "",
  evidence_sha256: "",
};
const wrap={minWidth:0,maxWidth:'100%' as const,flexShrink:1,flexWrap:'wrap' as const};
const observedStates=['CARD_APPROVED','CARD_DECLINED','CARD_NOT_SENT','REVERSED','REVERSAL_FAILED'];
const grantFields=['kind','version','scope','reconciliation_id','tenant_id','location_id','register_id','edge_device_id','terminal_id','binding_sha256','binding_generation','actor_id','evidence_reference','evidence_sha256','nbf','exp'] as const;
const paymentFields=['payment_id','operation_id','receipt_id','state','amount_minor','terminal_reference','rrn','auth_code','reversal_reference','reversal_rrn','reversal_auth_code'] as const;
const references = [
  "terminal_reference",
  "rrn",
  "auth_code",
  "reversal_reference",
  "reversal_rrn",
  "reversal_auth_code",
] as const;
export function PaymentReconciliationPanel(props: ReconciliationPanelProps) {
  const copy=reconciliationCopyFor(props.language??'en');
  const [decision, setDecision] = useState<ReconciliationDecision>(blank),
    [preview, setPreview] = useState<{
      command: ReconciliationCommand;
      grant: ReconciliationGrant;
      attempted: boolean;
    }>(),
    [pending, setPending] = useState<
      { command: ReconciliationCommand; attempted: boolean }[]
    >([]),
    [busy, setBusy] = useState(false),
    [failed, setFailed] = useState(false),
    [result, setResult] = useState<FiscalOperation>();
  const draftState = useRef({ decision, preview, busy });
  draftState.current = { decision, preview, busy };
  const pendingRequest = useRef(false);
  const blocked = useRef(false),
    lastScope = useRef("");
  const current = useRef(props);
  current.current = props;
  const epoch = useRef(0),
    mounted = useRef(false),
    requestKey = useRef(crypto.randomUUID());
  const identity = canonical(props.scope);
  useEffect(() => {
    if (lastScope.current !== identity) {
      blocked.current = false;
      lastScope.current = identity;
    }
    mounted.current = true;
    epoch.current++;
    pendingRequest.current = false;
    setBusy(false);
    setPreview(undefined);
    setDecision({ ...blank });
    setResult(undefined);
    setFailed(false);
    void pendingReconciliations(props.scope)
      .then((rows) => {
        if (mounted.current && canonical(current.current.scope) === identity)
          setPending(rows);
      })
      .catch((error) => {
        if (mounted.current && canonical(current.current.scope) === identity)
          setFailed(true);
      });
    return () => {
      mounted.current = false;
      epoch.current++;
    };
  }, [identity, props.administrator]);
  useLayoutEffect(
    () =>
      props.registerHeaderDraft?.({
        dirty: () =>
          !!draftState.current.preview ||
          canonical(draftState.current.decision) !== canonical(blank),
        pending: () => pendingRequest.current,
        discard: () => {
          epoch.current++;
          pendingRequest.current = false;
          setBusy(false);
          setDecision({ ...blank });
          setPreview(undefined);
          setResult(undefined);
          setFailed(false);
          requestKey.current = crypto.randomUUID();
        },
      }),
    [identity, props.registerHeaderDraft],
  );
  useEffect(() => {
    props.reportHeaderPending?.(busy);
    return () => props.reportHeaderPending?.(false);
  }, [busy, identity]);
  const editable = () =>
    mounted.current &&
    current.current.administrator &&
    !pendingRequest.current &&
    !blocked.current &&
    (current.current.canEditHeader?.() ?? true);
  const update = (field: keyof ReconciliationDecision, value: string) => {
    if (editable()) setDecision({ ...decision, [field]: value });
  };
  const live = (expected: number) =>
    mounted.current &&
    current.current.administrator &&
    epoch.current === expected &&
    canonical(current.current.scope) === identity;
  const active = async (expected = epoch.current) => {
    if (blocked.current) throw new Error("ADMIN_SESSION_REVOKED");
    if (
      !mounted.current ||
      !current.current.administrator ||
      epoch.current !== expected ||
      canonical(current.current.scope) !== identity
    )
      throw new Error("ADMIN_SESSION_CHANGED");
    await current.current.assertActive();
    if (
      !mounted.current ||
      !current.current.administrator ||
      epoch.current !== expected
    )
      throw new Error("ADMIN_SESSION_CHANGED");
  };
  const denied = (e: unknown) =>
    (e instanceof ApiError && (e.status === 401 || e.status === 403)) ||
    (e instanceof Error &&
      ["ADMIN_SESSION_CHANGED", "ADMIN_SESSION_REQUIRED"].includes(e.message));
  const prepare = async () => {
    if (!editable() || preview) return;
    pendingRequest.current = true;
    const version = epoch.current;
    setBusy(true);
    setFailed(false);
    try {
      await active(version);
      const command = await current.current.authorize(
        decision,
        requestKey.current,
      );
      await active(version);
      const grant = await inspectReconciliation(command, props.scope, decision);
      await stageReconciliation(props.scope, command);
      await active(version);
      setPreview({ command, grant, attempted: false });
      const rows = await pendingReconciliations(props.scope);
      if (live(version)) setPending(rows);
    } catch (e) {
      if (denied(e)) {
        blocked.current = true;
        await revokeReconciliationScope(props.scope).catch(() => {});
      }
      if (mounted.current && version === epoch.current) setFailed(true);
    } finally {
      if (mounted.current && version === epoch.current) {
        pendingRequest.current = false;
        setBusy(false);
      }
    }
  };
  const recover = async (row: {
    command: ReconciliationCommand;
    attempted: boolean;
  }) => {
    if (!editable()) return;
    pendingRequest.current = true;
    setBusy(true);
    const version = epoch.current;
    try {
      await active(version);
      const g = JSON.parse(
        new TextDecoder().decode(
          unbase64url(row.command.reconciliation_authorization.payload),
        ),
      ) as ReconciliationGrant;
      const d: ReconciliationDecision = {
        operation_id: g.expected.operation_id,
        payment_id: g.expected.payment_id,
        observed_state: g.observed.state,
        evidence_reference: g.evidence_reference,
        evidence_sha256: g.evidence_sha256,
      };
      for (const f of references) d[f] = g.observed[f];
      const grant = await inspectReconciliation(
        row.command,
        props.scope,
        d,
        Math.floor(Date.now() / 1000),
        row.attempted,
      );
      await active(version);
      setDecision(d);
      setPreview({ command: row.command, grant, attempted: row.attempted });
      setResult(undefined);
      setFailed(false);
    } catch (e) {
      if (denied(e)) {
        blocked.current = true;
        await revokeReconciliationScope(props.scope).catch(() => {});
      }
      if (mounted.current && version === epoch.current) setFailed(true);
    } finally {
      if (mounted.current && version === epoch.current) {
        pendingRequest.current = false;
        setBusy(false);
      }
    }
  };
  const confirm = async () => {
    if (!editable() || !preview) return;
    pendingRequest.current = true;
    const version = epoch.current;
    setBusy(true);
    setFailed(false);
    try {
      await active(version);
      const value = await dispatchReconciliation(
        props.scope,
        preview.command,
        current.current.routes,
        () => active(version),
      );
      await active(version);
      setResult(value);
      setPreview({ ...preview, attempted: true });
      const rows = await pendingReconciliations(props.scope);
      if (live(version)) setPending(rows);
    } catch (e) {
      if (denied(e)) {
        blocked.current = true;
        await revokeReconciliationScope(props.scope).catch(() => {});
      }
      if (mounted.current && version === epoch.current) {
        setFailed(true);
        const saved = await pendingReconciliations(props.scope);
        if (!live(version)) return;
        const row = saved.find(
          (r) =>
            r.command.client_operation_id ===
            preview.command.client_operation_id,
        );
        setPreview(
          denied(e)
            ? undefined
            : { ...preview, attempted: row?.attempted ?? false },
        );
      }
    } finally {
      if (mounted.current && version === epoch.current) {
        pendingRequest.current = false;
        setBusy(false);
      }
    }
  };
  if (!props.administrator) return null;
  return (
    <Card mode="outlined" testID="payment-reconciliation">
      <Card.Content style={{ gap: 12 }}>
        <Text variant="titleLarge" accessibilityRole="header">
          {copy.title}
        </Text>
        <Text>
          {copy.explanation}
        </Text>
        {!preview ? (
          <>
            <FormField
              label={copy.originalOperation}
              value={decision.operation_id}
              onChangeText={(v) => update("operation_id", v)}
            />
            <FormField
              label={copy.payment_id}
              value={decision.payment_id}
              onChangeText={(v) => update("payment_id", v)}
            />
            <View accessibilityRole="radiogroup" accessibilityLabel={copy.observedState} style={{gap:8,minWidth:0}} testID="payment-reconciliation.observed-state">
              <Text variant="labelLarge" accessibilityRole="header">{copy.observedState}</Text>
              <RadioButton.Group value={decision.observed_state} onValueChange={value=>{if(observedStates.includes(value))update("observed_state",value);}}>
                {observedStates.map(state=><AccessiblePaperRadioItem key={state} value={state} status={decision.observed_state===state?'checked':'unchecked'} onPress={()=>update("observed_state",state)} label={copy[('state'+state) as keyof ReconciliationCopy]} disabled={busy} mode="android" labelMaxFontSizeMultiplier={0} labelStyle={{flexShrink:1,flexWrap:'wrap'}} style={{minHeight:48,minWidth:48}} testID={`payment-reconciliation.observed.${state}`}/>)}
              </RadioButton.Group>
            </View>
            {references.map((f) => (
              <FormField
                key={f}
                label={copy[f]}
                value={decision[f] ?? ""}
                onChangeText={(v) => update(f, v)}
              />
            ))}
            <FormField
              label={copy.evidence_reference}
              value={decision.evidence_reference}
              onChangeText={(v) => update("evidence_reference", v)}
            />
            <FormField
              label={copy.evidence_sha256}
              value={decision.evidence_sha256}
              onChangeText={(v) => update("evidence_sha256", v)}
            />
            <Button disabled={busy} onPress={() => void prepare()}>
              {copy.request}
            </Button>
          </>
        ) : (
          <>
            <Text accessibilityRole="header">
              {copy.review}
            </Text>
            <View testID="payment-reconciliation.grant" style={{gap:8,minWidth:0}}>
              {grantFields.map(field=><List.Item key={field} title={copy[field]} titleNumberOfLines={0} titleStyle={wrap} descriptionNumberOfLines={0} descriptionStyle={wrap} description={()=> <Text selectable style={wrap}>{String(preview.grant[field])}</Text>}/>)}
            </View>
            <View testID="payment-reconciliation.comparison" style={{gap:12,minWidth:0}}>
              <Text variant="titleMedium" accessibilityRole="header">{copy.comparison}</Text>
              {paymentFields.map(field=><Card key={field} mode="outlined"><Card.Content style={{gap:8,minWidth:0}}>
                <Text variant="labelLarge" accessibilityRole="header" style={wrap}>{copy[field]}</Text>
                <List.Item title={copy.expected} titleNumberOfLines={0} titleStyle={wrap} description={()=> <Text selectable style={wrap}>{preview.grant.expected[field]}</Text>}/>
                <List.Item title={copy.observed} titleNumberOfLines={0} titleStyle={wrap} description={()=> <Text selectable style={wrap}>{preview.grant.observed[field]}</Text>}/>
              </Card.Content></Card>)}
            </View>
            <Button
              disabled={busy || confirmedReconciliation(result)}
              onPress={() => void confirm()}
            >
              {preview.attempted
                ? copy.recover
                : copy.confirm}
            </Button>
            <Button
              disabled={busy}
              onPress={() => {
                if (!editable()) return;
                setPreview(undefined);
                setResult(undefined);
                requestKey.current = crypto.randomUUID();
              }}
            >
              {copy.back}
            </Button>
          </>
        )}
        {pending.map((row) => (
          <Button
            key={row.command.client_operation_id}
            disabled={busy}
            onPress={() => void recover(row)}
          >
            {copy.saved.replace('{{id}}',row.command.client_operation_id)}
          </Button>
        ))}
        {result ? (
          <Text accessibilityLiveRegion="polite">
            {confirmedReconciliation(result)
              ? copy.recorded
              : copy.unresolved.replace('{{state}}',result.state)}
          </Text>
        ) : null}
        {failed ? (
          <HelperText type="error" visible accessibilityRole="alert">
            {copy.error}
          </HelperText>
        ) : null}
      </Card.Content>
    </Card>
  );
}
