import { requireFiscalResourceId } from "./fiscalIdentity.ts";

export type FiscalRegisterResource = {
  id: string;
  status: "ACTIVE" | "BLOCKED" | "INACTIVE";
  fiscal_device_id?: string | null;
};

export function resolveFiscalDeviceId(
  register: FiscalRegisterResource,
  expectedRegisterId: string,
  developmentFallback = "",
): string {
  const expected = requireFiscalResourceId(
    expectedRegisterId,
    "Фискалната каса",
  );
  if (register.id !== expected)
    throw new Error("Фискалната каса не съвпада с конфигурацията");
  if (register.status !== "ACTIVE")
    throw new Error("Фискалната каса не е активна");
  const candidate = register.fiscal_device_id || developmentFallback;
  if (!candidate)
    throw new Error("Фискалната каса няма активно фискално устройство");
  return requireFiscalResourceId(candidate, "Фискалното устройство");
}
