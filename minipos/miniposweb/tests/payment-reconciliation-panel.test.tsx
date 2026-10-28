// @vitest-environment jsdom
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeAll, beforeEach, expect, it, vi } from "vitest";
import { IDBFactory } from "fake-indexeddb";
import { webcrypto } from "node:crypto";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
const paperCapture=vi.hoisted(()=>({groups:[] as any[],items:[] as any[]}));
vi.mock('react-native',()=>({Platform:{OS:'web'},View:({children,testID}:any)=><div data-testid={testID}>{children}</div>}));
vi.mock("react-native-paper", () => {
  const C = ({ children }: any) => <div>{children}</div>;
  return { Text: C, Card: Object.assign(C, { Content: C }), HelperText: C,
 List:{Item:({title,description}:any)=><section><span>{title}</span><span>{typeof description==='function'?description():description}</span></section>},
 RadioButton:{Group:(props:any)=>{paperCapture.groups.push(props);return <div>{props.children}</div>},Item:(props:any)=>{paperCapture.items.push(props);return <span>{props.label}</span>}}
 };
});
vi.mock("../src/ui/app-button", () => ({
  AppButton: ({ children, onPress, disabled }: any) => (
    <button disabled={disabled} onClick={onPress}>
      {children}
    </button>
  ),
}));
vi.mock("../src/ui/form-field", () => ({
  FormField: ({ label, value, onChangeText }: any) => (
    <label>
      {label}
      <input
        aria-label={label}
        value={value}
        onInput={(e) => onChangeText((e.target as HTMLInputElement).value)}
      />
    </label>
  ),
}));
import {reconciliationCopyFor} from "../src/ui/reconciliation-copy";
import { PaymentReconciliationPanel } from "../src/PaymentReconciliationPanel";
let fixture: any;
beforeAll(() => {
  const output = path.join(
    fs.mkdtempSync(path.join(os.tmpdir(), "reconciliation-panel-")),
    "fixture.json",
  );
  execFileSync(
    "go",
    [
      "test",
      "./internal/domain",
      "-run",
      "^TestDelegatedReconciliationExplicitAdminCapabilityAndScope$",
      "-count=1",
    ],
    {
      cwd: path.resolve("../../fiscal-backend"),
      env: {
        ...process.env,
        GOCACHE:
          process.env.GOCACHE ??
          path.join(os.tmpdir(), "fiscal-authority-oracle-go-cache"),
        RECONCILIATION_UI_FIXTURE: output,
      },
    },
  );
  fixture = JSON.parse(fs.readFileSync(output, "utf8"));
}, 30000);

let host: HTMLDivElement, root: Root;
beforeEach(() => {
  (globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
  vi.stubGlobal("indexedDB", new IDBFactory());
  vi.stubGlobal("crypto", webcrypto);
  paperCapture.groups=[];paperCapture.items=[];
  host = document.createElement("div");
  document.body.replaceChildren(host);
  root = createRoot(host);
});
const button = (name: string) =>
  [...host.querySelectorAll("button")].find((b) => b.textContent === name)!;
const fill = (name: string, value: string) => {
  const input = host.querySelector<HTMLInputElement>(
    'input[aria-label="' + name + '"]',
  )!;
  Object.getOwnPropertyDescriptor(
    HTMLInputElement.prototype,
    "value",
  )!.set!.call(input, value);
  input.dispatchEvent(new Event("input", { bubbles: true }));
};
it("requires explicit review and confirmation of actual root signed expected/observed state before sending", async () => {
  const authorize = vi.fn(async () => fixture.command),
    submit = vi.fn(async (_command: any, _signal: any) => ({
      operation_id: fixture.command.client_operation_id,
      state: "COMMITTED",
      certainty: "PROVEN_SUCCESS",
      updated_at: new Date().toISOString(),
    }));
  try {
    await act(async () =>
      root.render(
        <PaymentReconciliationPanel
          administrator
          scope={fixture.scope}
          authorize={authorize}
          assertActive={async () => {}}
          routes={[
            {
              kind: "LOCAL_HTTP",
              probe: async () => true,
              submit,
              lookup: async () => {
                throw new Error("unexpected lookup");
              },
            },
          ]}
        />,
      ),
    );
    for (const [label, field] of [
      ["Original operation UUID", "operation_id"],
      ["Payment ID", "payment_id"],
      ["Acquirer evidence reference", "evidence_reference"],
      ["Evidence SHA256", "evidence_sha256"],
      ["Terminal reference", "terminal_reference"],
      ["Retrieval reference number (RRN)", "rrn"],
      ["Authorization code", "auth_code"],
    ])
      await act(async () => fill(label, fixture.decision[field]));
    await act(async () => button("Request signed decision").click());
    await vi.waitFor(async () => {
      await act(async () => {});
      expect(host.textContent).toContain("Review signed server decision");
    });
    expect(authorize).toHaveBeenCalledTimes(1);
    expect(submit).not.toHaveBeenCalled();
    expect(host.querySelector('[data-testid="payment-reconciliation.comparison"]')).not.toBeNull();
    expect(host.querySelector("select,table,dl,dt,dd")).toBeNull();
    expect(host.textContent).toContain("CARD_UNKNOWN");
    expect(host.textContent).toContain("CARD_APPROVED");
    expect(host.textContent).toContain(fixture.grant.evidence_sha256);
    await vi.waitFor(async () => {
      await act(async () => {});
      expect(button("Confirm and send this bank decision").disabled).toBe(false);
    }, {timeout:10000});
    await act(async () =>
      button("Confirm and send this bank decision").click(),
    );
    await vi.waitFor(async () => {
      await act(async () => {});
      expect(host.textContent).toContain(
        "Bank decision recorded; fiscal outcome remains unresolved.",
      );
    }, {timeout:10000});
    expect(submit).toHaveBeenCalledTimes(1);
    expect(submit.mock.calls[0][0]).toEqual(fixture.command);
    expect(button("Recover this decision status").disabled).toBe(true);
  } finally {
    await act(async () => root.unmount());
  }
}, 20000);
it("does not render or submit for nonADMIN and fences issuance completing after logout", async () => {
  let resolve!: (v: any) => void;
  const authorize = vi.fn(() => new Promise<any>((r) => (resolve = r))),
    submit = vi.fn();
  const props = {
    scope: fixture.scope,
    authorize,
    assertActive: async () => {},
    routes: [
      {
        kind: "LOCAL_HTTP" as const,
        probe: async () => true,
        submit,
        lookup: async () => {
          throw new Error("lookup");
        },
      },
    ],
  };
  try {
    await act(async () =>
      root.render(<PaymentReconciliationPanel {...props} administrator />),
    );
    await act(async () => button("Request signed decision").click());
    await vi.waitFor(() => expect(authorize).toHaveBeenCalledTimes(1));
    await act(async () =>
      root.render(
        <PaymentReconciliationPanel {...props} administrator={false} />,
      ),
    );
    await act(async () => resolve(fixture.command));
    expect(host.textContent).not.toContain("Review signed server decision");
    expect(submit).not.toHaveBeenCalled();
    expect(host.querySelector("button")).toBeNull();
  } finally {
    await act(async () => root.unmount());
  }
});

it('renders five genuine Paper radio choices and fences malformed state changes',async()=>{
 const authorize=vi.fn(async()=>fixture.command);
 try{
  await act(async()=>root.render(<PaymentReconciliationPanel administrator scope={fixture.scope} authorize={authorize} assertActive={async()=>{}} routes={[]}/>));
  expect(paperCapture.items.slice(-5).map(p=>p.value)).toEqual(['CARD_APPROVED','CARD_DECLINED','CARD_NOT_SENT','REVERSED','REVERSAL_FAILED']);expect(host.querySelector('select')).toBeNull();
  await act(async()=>paperCapture.groups.at(-1).onValueChange('CARD_DECLINED'));expect(paperCapture.groups.at(-1).value).toBe('CARD_DECLINED');
  await act(async()=>paperCapture.groups.at(-1).onValueChange('FOREIGN_STATE'));expect(paperCapture.groups.at(-1).value).toBe('CARD_DECLINED');
 }finally{await act(async()=>root.unmount());}
});

it('shows safe copy instead of raw error JSON and keeps the decision draft without retrying',async()=>{
 const authorize=vi.fn(async()=>{throw new Error('{"token":"PRIVATE_BANK_SECRET","debug":{"sql":"internal"}}')});
 try{
  await act(async()=>root.render(<PaymentReconciliationPanel administrator scope={fixture.scope} authorize={authorize} assertActive={async()=>{}} routes={[]}/>));
  await act(async()=>fill('Original operation UUID','retained-operation'));
  await act(async()=>button('Request signed decision').click());
  await vi.waitFor(()=>expect(host.textContent).toContain('Unable to load or confirm the bank decision.'));
  expect(host.textContent).not.toContain('PRIVATE_BANK_SECRET');expect(host.textContent).not.toContain('"debug"');
  expect(host.querySelector<HTMLInputElement>('input[aria-label="Original operation UUID"]')!.value).toBe('retained-operation');
  expect(authorize).toHaveBeenCalledTimes(1);expect(button('Request signed decision').disabled).toBe(false);
 }finally{await act(async()=>root.unmount());}
});

it('updates every visible error and choice label on EN BG RU changes without losing draft or repeating issuance',async()=>{
 const authorize=vi.fn(async()=>{throw new Error('PRIVATE_TRANSLATION_SECRET')});
 const props={administrator:true,scope:fixture.scope,authorize,assertActive:async()=>{},routes:[]};
 try{
  await act(async()=>root.render(<PaymentReconciliationPanel {...props} language="en"/>));
  await act(async()=>fill('Original operation UUID','retained-across-language'));
  await act(async()=>button('Request signed decision').click());
  for(const language of ['bg','ru','en']){
   const copy=reconciliationCopyFor(language);await act(async()=>root.render(<PaymentReconciliationPanel {...props} language={language}/>));
   expect(host.textContent).toContain(copy.title);expect(host.textContent).toContain(copy.error);expect(host.textContent).not.toContain('PRIVATE_TRANSLATION_SECRET');
   expect(host.querySelector<HTMLInputElement>('input[aria-label="'+copy.originalOperation+'"]')!.value).toBe('retained-across-language');
   expect(paperCapture.items.slice(-5).map(p=>p.label)).toEqual(['CARD_APPROVED','CARD_DECLINED','CARD_NOT_SENT','REVERSED','REVERSAL_FAILED'].map(state=>copy[('state'+state) as keyof typeof copy]));
  }
  expect(authorize).toHaveBeenCalledTimes(1);
 }finally{await act(async()=>root.unmount());}
});
